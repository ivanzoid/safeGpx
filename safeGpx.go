package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/clbanning/mxj"
	// "github.com/kellydunn/golang-geo"
	"io"
	"os"
	"path/filepath"
)

type XmlProcessor struct {
	reader io.Reader
	writer io.Writer
}

func (self *XmlProcessor) Process(inputFileName, outputFileName string) (err error) {

	inputFile, err := os.Open(inputFileName)
	if err != nil {
		return
	}

	defer func() {
		err = inputFile.Close()
	}()

	inputFileReader := bufio.NewReader(inputFile)

	outputFile, err := os.Create(outputFileName)
	if err != nil {
		panic(err)
	}

	defer func() {
		err = outputFile.Close()
	}()

	// outputFileWriter := bufio.NewWriter(outputFile)

	err = mxj.HandleXmlReader(inputFileReader, self.filterHandler, self.errHandler)
	if err != nil {
		return
	}

	return
}

func (self *XmlProcessor) filterHandler(m mxj.Map) bool {

	// xmlVal, err := m.Xml()
	// if err != nil {
	// 	fmt.Println(err)
	// 	return false
	// }

	// xlmString := string(xmlVal)

	fmt.Printf("LeafPaths: '%v'\n", m.LeafPaths())

	// _, err = self.writer.Write(xmlVal)
	// if err != nil {
	// 	fmt.Println(err)
	// 	return false
	// }

	return true
}

func (self *XmlProcessor) errHandler(err error) bool {
	fmt.Println(err)
	return true
}

func appName() string {
	return filepath.Base(os.Args[0])
}

func usage() {
	appName := appName()
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Usage:\n\n")
	fmt.Fprintf(os.Stderr, "\t%v <gpxFile> [-o <outputGpxFile>]\n", appName)
	fmt.Fprintf(os.Stderr, "\n")
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
		fmt.Printf("dir = %v, ext = %v, base = %v, name = %v\n", dir, ext, base, name)
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
