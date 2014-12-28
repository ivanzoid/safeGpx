package main

import (
	"flag"
	"fmt"
	"github.com/kellydunn/golang-geo"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"strconv"
)

type Points []geo.Point

func (p *Points) String() string {
	return fmt.Sprint(*p)
}

func (p *Points) Set(value string) error {
	values := strings.Split(value, ",")
	for i := 0; i < len(values); i += 2 {
		if i + 1 >= len(values) {
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

type XmlProcessor struct {
	reader io.Reader
	writer io.Writer
	insideTrackPointToSkip bool
}

func (self *XmlProcessor) Process(inputFileName, outputFileName string) (err error) {

	type TrackPoint struct {
		Lat float64 `xml:"lat,attr"`
		Lon float64 `xml:"lon,attr"`
	}

	inputFile, err := os.Open(inputFileName)
	if err != nil {
		return
	}

	defer inputFile.Close()

	xmlDecoder := xml.NewDecoder(inputFile)

	for {
		token, err := xmlDecoder.Token()
		if token == nil {
			break
		}
		if err != nil {
			return err
		}

		fmt.Printf("token: %v\n", token)

		switch t := token.(type) {
			case xml.StartElement:
				if t.Name.Local == "trkpt" {
					var trackPoint TrackPoint
					err = xmlDecoder.DecodeElement(&trackPoint, &t)
					if err != nil {
						fmt.Printf("Warning: can't decode track point at position %v\n", xmlDecoder.InputOffset())
						continue
					}
				} else {

				}
			case xml.EndElement:
				if t.Name.Local == "trkpt" {
					self.insideTrackPointToSkip = false
				}
		}
	}

	// outputFile, err := os.Create(outputFileName)
	// if err != nil {
	// 	panic(err)
	// }

	// defer func() {
	// 	err = outputFile.Close()
	// }()

	// // outputFileWriter := bufio.NewWriter(outputFile)

	// err = mxj.HandleXmlReader(inputFileReader, self.filterHandler, self.errHandler)
	// if err != nil {
	// 	return
	// }

	return
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

	x := &XmlProcessor{}
	err := x.Process(fileName, outputFileName)
	if err != nil {
		panic(err)
	}
}
