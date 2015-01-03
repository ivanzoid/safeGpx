package main

import (
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"github.com/kellydunn/golang-geo"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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

type XmlProcessor struct {
	xmlDecoder    *xml.Decoder
	xmlEncoder    *xml.Encoder
	inPointToSkip bool
	Polygon       geo.Polygon
	xmlns         string
}

func (self *XmlProcessor) Process(inputFileName, outputFileName string) (err error) {

	inputFile, err := os.Open(inputFileName)
	if err != nil {
		return
	}
	defer inputFile.Close()

	self.xmlDecoder = xml.NewDecoder(inputFile)

	outputFile, err := os.Create(outputFileName)
	if err != nil {
		return
	}
	defer outputFile.Close()

	self.xmlEncoder = xml.NewEncoder(outputFile)
	self.xmlEncoder.Indent("", "  ")

	for {
		token, err := self.xmlDecoder.Token()
		if token == nil {
			break
		}
		if err != nil {
			return err
		}

		fmt.Printf("token: %v\n", token)

		err = self.handleToken(token)
		if err != nil {
			return err
		}
	}

	self.xmlEncoder.Flush()

	return
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
						fmt.Printf("Warning: can't decode latitude of track point at position %v\n", self.xmlDecoder.InputOffset())
					} else {
						lat = localLat
					}
				} else if attribute.Name.Local == "lon" {
					localLon, err := strconv.ParseFloat(attribute.Value, 10)
					if err != nil {
						fmt.Printf("Warning: can't decode longitude of track point at position %v\n", self.xmlDecoder.InputOffset())
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
				fmt.Printf("Skipping point: %v\n", point)
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
		fmt.Printf("Writing token\n")
		self.xmlEncoder.EncodeToken(token)
	} else {
		fmt.Printf("Skipping token\n")
	}

	return nil
}

func appName() string {
	return filepath.Base(os.Args[0])
}

func usage() {
	appName := appName()
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
		fmt.Fprintf(os.Stderr, "\t-%s:\t%s%s\n", flag.Name, flag.Usage, defaultValue)
	})
}

var skipArea Points

func init() {
	// Tie the command-line flag to the intervalFlag variable and
	// set a usage message.
	flag.Var(&skipArea, "skipArea", "Area (in format lat1,lon1,lat2,lon2,etc.) to exclude from resulting GPX file.\n\t\t\tYou may use only 2 points (top-left and bottom-right) to specify rectangular area.")
}

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		usage()
		return
	}

	if len(skipArea) < 1 {
		fmt.Printf("Please specify skipArea.\n")
		return
	}

	fmt.Printf("skipArea: %v\n", skipArea)

	fileName := flag.Args()[0]
	outputFileName := ""

	if flag.NArg() >= 2 {
		outputFileName = flag.Args()[1]
	} else {
		dir := filepath.Dir(fileName)
		base := filepath.Base(fileName)
		ext := filepath.Ext(base)
		name := base[0 : len(base)-len(ext)]
		outputFileName = fmt.Sprintf("%v/%v_safe%v", dir, name, ext)
		outputFileName = filepath.Clean(outputFileName)
	}

	fmt.Printf("%v -> %v\n", fileName, outputFileName)

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

	fmt.Printf("polygon: %v\n", polygonDesc(polygon))

	xp := &XmlProcessor{}
	xp.Polygon = polygon
	err := xp.Process(fileName, outputFileName)
	if err != nil {
		fmt.Println(err)
	}
}
