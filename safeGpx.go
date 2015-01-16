package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"github.com/kellydunn/golang-geo"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	kAppVersion = "1.0"
)

//
// Misc
//

func polygonDesc(polygon geo.Polygon) string {
	points := polygon.Points()
	descriptions := make([]string, 0)
	for _, point := range points {
		description := fmt.Sprint(*point)
		descriptions = append(descriptions, description)
	}
	result := fmt.Sprintf("[%v]", strings.Join(descriptions, " "))
	return result
}

//
// XmlProcessor
//

type XmlProcessor struct {
	Polygon              geo.Polygon
	VerbosePrinting      bool
	xmlDecoder           *xml.Decoder
	xmlEncoder           *xml.Encoder
	inPointToSkip        bool
	xmlns                string
	skippedPointsCounter int
}

func (self *XmlProcessor) Process(inputReader io.Reader, outputWriter io.Writer) (err error) {

	self.xmlDecoder = xml.NewDecoder(inputReader)

	self.xmlEncoder = xml.NewEncoder(outputWriter)
	self.xmlEncoder.Indent("", "  ")

	for {
		token, err := self.xmlDecoder.Token()
		if token == nil {
			break
		}
		if err != nil {
			return err
		}

		// fmt.Fprintf(os.Stderr, "token: %v\n", token)

		err = self.handleToken(token)
		if err != nil {
			return err
		}
	}

	self.xmlEncoder.Flush()

	return
}

func (self *XmlProcessor) Xmlns() string {
	return self.xmlns
}

func (self *XmlProcessor) SkippedPointsCount() int {
	return self.skippedPointsCounter
}

func (self *XmlProcessor) handleToken(token xml.Token) (err error) {

	skipToken := self.inPointToSkip

	switch t := token.(type) {

	case xml.StartElement:

		if t.Name.Local == "gpx" {

			for _, attribute := range t.Attr {
				if attribute.Name.Local == "xmlns" {
					self.xmlns = attribute.Value
				}
			}

		} else if t.Name.Local == "trkpt" {

			if self.inPointToSkip {
				return errors.New(fmt.Sprintf("trkpt inside trkpt at at position %v", self.xmlDecoder.InputOffset()))
			}

			lat := math.MaxFloat64
			lon := math.MaxFloat64

			for _, attribute := range t.Attr {
				if attribute.Name.Local == "lat" {
					localLat, err := strconv.ParseFloat(attribute.Value, 10)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: can't decode latitude of track point at position %v\n", self.xmlDecoder.InputOffset())
					} else {
						lat = localLat
					}
				} else if attribute.Name.Local == "lon" {
					localLon, err := strconv.ParseFloat(attribute.Value, 10)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: can't decode longitude of track point at position %v\n", self.xmlDecoder.InputOffset())
					} else {
						lon = localLon
					}
				}
			}

			if lat == math.MaxFloat64 || lon == math.MaxFloat64 {
				return nil
			}

			point := geo.NewPoint(lat, lon)

			if self.Polygon.Contains(point) {
				if self.VerbosePrinting {
					fmt.Fprintf(os.Stderr, "Skipping point: (%+.6f, %+.6f)\n", point.Lat(), point.Lng())
				}
				self.skippedPointsCounter++
				self.inPointToSkip = true
				skipToken = true
			}
		}

	case xml.EndElement:

		if t.Name.Local == "trkpt" {
			if self.inPointToSkip {
				self.inPointToSkip = false
				skipToken = true
			}
		}

	case xml.CharData:

		str := string(t)
		newStr := strings.Replace(str, "\x0a", "", -1)
		if len(newStr) != len(str) {
			token = []byte(newStr)
		}

	}

	if !skipToken {
		// fmt.Fprintf(os.Stderr, "Writing token\n")
		self.xmlEncoder.EncodeToken(token)
	} else {
		// fmt.Fprintf(os.Stderr, "Skipping token\n")
	}

	return nil
}

//
// Points
//

type Points []geo.Point

func (p *Points) String() string {
	return fmt.Sprint(*p)
}

func (p *Points) Set(value string) error {
	values := strings.Split(value, ",")
	for i := 0; i < len(values); i += 2 {
		if i+1 >= len(values) {
			break
		}
		lat, err := strconv.ParseFloat(values[i], 10)
		if err != nil {
			return err
		}
		lon, err := strconv.ParseFloat(values[i+1], 10)
		if err != nil {
			return err
		}
		point := geo.NewPoint(lat, lon)
		*p = append(*p, *point)
	}
	return nil
}

var GSkipArea Points
var GVerbose bool
var GPrintVersion bool
var GOutputFileName string

func init() {
	flag.Var(&GSkipArea, "skipArea", "Area (in format lat1,lon1,lat2,lon2,etc.) to exclude from resulting GPX file.\n\t\t\tYou may use only 2 points (top-left and bottom-right) to specify rectangular area.")
	flag.BoolVar(&GVerbose, "v", false, "Use verbose output")
	flag.BoolVar(&GPrintVersion, "version", false, "Print version and quit")
	flag.StringVar(&GOutputFileName, "o", "", "Output file name")
}

//
// Main
//

func appName() string {
	return filepath.Base(os.Args[0])
}

func usage() {
	appName := appName()
	fmt.Fprintf(os.Stderr, "%v is a tool for filtering out unwanted regions from GPX files.\n", appName)
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Usage:\n\n")
	fmt.Fprintf(os.Stderr, "\t%v <options> [-o <outputGpxFile>] <sourceGpxFile>\n", appName)
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Options are:\n")
	fmt.Fprintf(os.Stderr, "\n")

	flag.CommandLine.VisitAll(func(flag *flag.Flag) {
		defaultValue := ""
		if len(flag.DefValue) > 0 && flag.DefValue != "[]" {
			defaultValue = fmt.Sprintf(" [%v]", flag.DefValue)
		}
		nameWithColon := fmt.Sprintf("%v:", flag.Name)
		fmt.Fprintf(os.Stderr, "\t-%-12s\t%s%s\n", nameWithColon, flag.Usage, defaultValue)
	})
}

func polygonFromSkipArea(skipArea Points) geo.Polygon {

	var polygon geo.Polygon

	if len(skipArea) == 2 {
		topLeft := skipArea[0]
		bottomRight := skipArea[1]
		topRight := geo.NewPoint((&topLeft).Lat(), (&bottomRight).Lng())
		bottomLeft := geo.NewPoint((&bottomRight).Lat(), (&topLeft).Lng())
		polygon.Add(&topLeft)
		polygon.Add(topRight)
		polygon.Add(&bottomRight)
		polygon.Add(bottomLeft)
	} else {
		for _, point := range skipArea {
			polygon.Add(&point)
		}
	}

	return polygon
}

func main() {
	flag.Parse()

	if GPrintVersion {
		fmt.Fprintf(os.Stderr, "%v v%v\n", appName(), kAppVersion)
		return
	}

	if flag.NArg() < 1 {
		usage()
		return
	}

	if len(GSkipArea) < 1 {
		fmt.Fprintf(os.Stderr, "Please specify skipArea.\n")
		return
	}

	inputFileName := flag.Args()[0]

	polygon := polygonFromSkipArea(GSkipArea)

	if GVerbose {
		fmt.Fprintf(os.Stderr, "Polygon is: %v\n", polygonDesc(polygon))
	}

	inputFile, err := os.Open(inputFileName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer inputFile.Close()

	outputBuffer := new(bytes.Buffer)
	outputWriter := bufio.NewWriter(outputBuffer)

	xp := &XmlProcessor{}
	xp.Polygon = polygon
	xp.VerbosePrinting = GVerbose
	err = xp.Process(inputFile, outputWriter)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	skippedPointsCount := xp.SkippedPointsCount()
	if skippedPointsCount > 0 {
		optionalS := ""
		if skippedPointsCount > 1 {
			optionalS = "s"
		}
		fmt.Fprintf(os.Stderr, "Skipped %d point%v.\n", skippedPointsCount, optionalS)
	}

	outputWriter.Flush()

	resultString := outputBuffer.String()
	xmlnsString := fmt.Sprintf(" xmlns=\"%v\"", xp.Xmlns())
	gpxString := fmt.Sprintf("<gpx%v", xmlnsString)

	resultString = strings.Replace(resultString, xmlnsString, "", -1)
	resultString = strings.Replace(resultString, "<gpx", gpxString, 1)

	outputFile := os.Stdout

	if len(GOutputFileName) != 0 {
		outputFile, err = os.Create(GOutputFileName)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		defer outputFile.Close()
	}

	_, err = outputFile.WriteString(resultString)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
}
