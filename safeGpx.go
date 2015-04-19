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
	appVersion = "1.0"
)

//
// Misc
//

func polygonDesc(polygon geo.Polygon) string {
	points := polygon.Points()
	var descriptions []string
	for _, point := range points {
		description := fmt.Sprint(*point)
		descriptions = append(descriptions, description)
	}
	result := fmt.Sprintf("[%v]", strings.Join(descriptions, " "))
	fmt.Println("Woo hoo")
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

func (xp *XmlProcessor) Process(inputReader io.Reader, outputWriter io.Writer) (err error) {

	xp.xmlDecoder = xml.NewDecoder(inputReader)

	xp.xmlEncoder = xml.NewEncoder(outputWriter)
	xp.xmlEncoder.Indent("", "  ")

	for {
		token, err := xp.xmlDecoder.Token()
		if token == nil {
			break
		}
		if err != nil {
			return err
		}

		// fmt.Printf("token: %v\n", token)

		err = xp.handleToken(token)
		if err != nil {
			return err
		}
	}

	xp.xmlEncoder.Flush()

	return
}

func (xp *XmlProcessor) Xmlns() string {
	return xp.xmlns
}

func (xp *XmlProcessor) SkippedPointsCount() int {
	return xp.skippedPointsCounter
}

func (xp *XmlProcessor) handleToken(token xml.Token) (err error) {

	skipToken := xp.inPointToSkip

	switch t := token.(type) {

	case xml.StartElement:

		if t.Name.Local == "gpx" {

			for _, attribute := range t.Attr {
				if attribute.Name.Local == "xmlns" {
					xp.xmlns = attribute.Value
				}
			}

		} else if t.Name.Local == "trkpt" {

			if xp.inPointToSkip {
				return errors.New(fmt.Sprintf("trkpt inside trkpt at at position %v", xp.xmlDecoder.InputOffset()))
			}

			lat := math.MaxFloat64
			lon := math.MaxFloat64

			for _, attribute := range t.Attr {
				if attribute.Name.Local == "lat" {
					localLat, err := strconv.ParseFloat(attribute.Value, 10)
					if err != nil {
						fmt.Printf("Warning: can't decode latitude of track point at position %v\n", xp.xmlDecoder.InputOffset())
					} else {
						lat = localLat
					}
				} else if attribute.Name.Local == "lon" {
					localLon, err := strconv.ParseFloat(attribute.Value, 10)
					if err != nil {
						fmt.Printf("Warning: can't decode longitude of track point at position %v\n", xp.xmlDecoder.InputOffset())
					} else {
						lon = localLon
					}
				}
			}

			if lat == math.MaxFloat64 || lon == math.MaxFloat64 {
				return nil
			}

			point := geo.NewPoint(lat, lon)

			if xp.Polygon.Contains(point) {
				if xp.VerbosePrinting {
					fmt.Printf("Skipping point: (%+.6f, %+.6f)\n", point.Lat(), point.Lng())
				}
				xp.skippedPointsCounter++
				xp.inPointToSkip = true
				skipToken = true
			}
		}

	case xml.EndElement:

		if t.Name.Local == "trkpt" {
			if xp.inPointToSkip {
				xp.inPointToSkip = false
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
		// fmt.Printf("Writing token\n")
		xp.xmlEncoder.EncodeToken(token)
	} else {
		// fmt.Printf("Skipping token\n")
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

var gSkipArea Points
var gVerbose bool
var gPrintVersion bool

func init() {
	flag.Var(&gSkipArea, "skipArea", "Area (in format lat1,lon1,lat2,lon2,etc.) to exclude from resulting GPX file.\n\t\t\tYou may use only 2 points (top-left and bottom-right) to specify rectangular area.")
	flag.BoolVar(&gVerbose, "v", false, "Use verbose output")
	flag.BoolVar(&gPrintVersion, "version", false, "Print version and quit")
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
	fmt.Fprintf(os.Stderr, "\t%v <options> <gpxFile> [-o <outputGpxFile>]\n", appName)
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

func safeFileName(inputFileName string) string {

	dir := filepath.Dir(inputFileName)
	base := filepath.Base(inputFileName)
	ext := filepath.Ext(base)
	name := base[0 : len(base)-len(ext)]
	outputFileName := fmt.Sprintf("%v/%v_safe%v", dir, name, ext)
	outputFileName = filepath.Clean(outputFileName)

	return outputFileName
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

	if gPrintVersion {
		fmt.Fprintf(os.Stderr, "%v v%v\n", appName(), appVersion, appVersion)
		return
	}

	if flag.NArg() < 1 {
		usage()
		return
	}

	if len(gSkipArea) < 1 {
		fmt.Fprintf(os.Stderr, "Please specify skipArea.\n")
		return
	}

	inputFileName := flag.Args()[0]
	outputFileName := ""

	if flag.NArg() >= 2 {
		outputFileName = flag.Args()[1]
	} else {
		outputFileName = safeFileName(inputFileName)
	}

	polygon := polygonFromSkipArea(gSkipArea)

	if gVerbose {
		fmt.Printf("Polygon is: %v\n", polygonDesc(polygon))
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
	xp.VerbosePrinting = gVerbose
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
		fmt.Printf("Skipped %d point%v.\n", skippedPointsCount, optionalS)
	}

	outputWriter.Flush()

	resultString := outputBuffer.String()
	xmlnsString := fmt.Sprintf(" xmlns=\"%v\"", xp.Xmlns())
	gpxString := fmt.Sprintf("<gpx%v", xmlnsString)

	resultString = strings.Replace(resultString, xmlnsString, "", -1)
	resultString = strings.Replace(resultString, "<gpx", gpxString, 1)

	outputFile, err := os.Create(outputFileName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer outputFile.Close()

	_, err = outputFile.WriteString(resultString)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
}
