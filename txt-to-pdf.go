package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

var inputFile string
var outputFile string
var fontSize int
var pageSize string
var orientation string

//Flag-defaults
const (
	defaultInputFile  = ""
	defaultOutputFile = ""
	//defaultInputFile   = "/tmp/test/input_test.txt"
	//defaultOutputFile  = "/tmp/test/input_test.pdf"
	defaultFontSize    = 10
	defaultPageSize    = "A4"
	defaultOrientation = "P"
)

//Flag usages
const (
	usageInputFile   = "input filename"
	usageOutputFile  = "output filename"
	usageFontSize    = "Fontsize to use. Measured in points."
	usagePageSize    = "Page size to use. Possible values: A3, A4, A5, Letter, Legal, Tabloid."
	usageOrientation = "Page orientation to use. Possible values: L, P"
)

func defineFlags() {
	flag.StringVar(&inputFile, "f", defaultInputFile, usageInputFile)
	flag.StringVar(&outputFile, "o", defaultOutputFile, usageOutputFile)
	flag.IntVar(&fontSize, "fs", 10, usageFontSize)
	flag.StringVar(&pageSize, "ps", defaultPageSize, usagePageSize)
	flag.StringVar(&orientation, "po", defaultOrientation, usageOrientation)
}

func createPdfFile(outFile string, input string) error {
	pdf := gofpdf.New(orientation, "pt", pageSize, "")
	pdf.AddPage()
	pdf.SetFont("courier", "", float64(fontSize))
	pdf.MultiCell(0, float64(fontSize), input, "", "", false)

	return pdf.OutputFileAndClose(outFile)
}

func main() {
	defineFlags()
	flag.Parse()
	//check if in and outputfiles have been set. if not show help and exit
	if outputFile == "" {
		fmt.Println("No outputfile set!")
		flag.PrintDefaults()
		os.Exit(1)
	}
	if inputFile == "" {
		fmt.Println("No inputfile set!")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if inputRaw, err := ioutil.ReadFile(inputFile); err != nil {
		fmt.Println("Error reading InputFile!", err)
		os.Exit(1)
	} else {
		input := string(inputRaw)
		//replace tabs (/t) with spaces
		input = strings.Replace(input, "\t", "        ", -1)
		if err := createPdfFile(outputFile, input); err != nil {
			fmt.Println("Error creating pdf file!", err)
			os.Exit(1)
		}
	}
}
