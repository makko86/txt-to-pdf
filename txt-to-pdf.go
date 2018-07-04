package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/jung-kurt/gofpdf"
)

var inputFile string
var outputFile string
var fontSize int
var pageSize string
var orientation string
var tabSpacing int
var dirMode bool
var verboseMode bool
var lineCount int

//Flag-defaults
const (
	defaultInputFile   = ""
	defaultOutputFile  = ""
	defaultFontSize    = 10
	defaultPageSize    = "A4"
	defaultOrientation = "P"
	defaultTabSpacing  = 8
	defaultDirMode     = false
	defaultVerboseMode = false
	defaultLineCount   = 0
)

//Flag usages
const (
	usageInputFile   = "input file. Optional: Can be omitted to use STDIN as input"
	usageOutputFile  = "output file Optional: Can be omitted if \"inputFile\" is specified. In this case \"inputFile.pdf\" will be used as outputFile"
	usageFontSize    = "Fontsize to use."
	usagePageSize    = "Page size to use. Possible values: A3, A4, A5, Letter, Legal."
	usageOrientation = "Page orientation to use. Possible values: L, P"
	usageTabSpacing  = "Number of spaces used to replace tabstops."
	usageDirMode     = "Optional. If set \"inputFile\" and \"outpuFile\" will be treated as directories. txt-to-pdf will try to parse every single file in \"inputFile\""
	usageVerboseMode = "Optional."
	usageLineCount   = "Optional. If set a pagebreak will be inserted every n-pages."
)

type errorMessage string

func (em errorMessage) Error() string {
	return fmt.Sprintf(string(em))
}

func defineFlags() {
	flag.StringVar(&inputFile, "if", defaultInputFile, usageInputFile)
	flag.StringVar(&outputFile, "of", defaultOutputFile, usageOutputFile)
	flag.IntVar(&fontSize, "fs", defaultFontSize, usageFontSize)
	flag.StringVar(&pageSize, "ps", defaultPageSize, usagePageSize)
	flag.StringVar(&orientation, "po", defaultOrientation, usageOrientation)
	flag.IntVar(&tabSpacing, "ts", defaultTabSpacing, usageTabSpacing)
	flag.BoolVar(&dirMode, "dir", defaultDirMode, usageDirMode)
	flag.BoolVar(&verboseMode, "verb", defaultVerboseMode, usageVerboseMode)
	flag.IntVar(&lineCount, "lc", defaultLineCount, usageLineCount)
	flag.Parse()
}

//Check if the commandline arguments have the correct values etc.
func flagsOkay() error {
	if inputFile == "" && outputFile == "" {
		return errorMessage("No input or output specified")
	}

	if fontSize <= 1 {
		return errorMessage("FontSize must be greater than 1")
	} else if tabSpacing <= 1 {
		return errorMessage("tabSpacing must be graeter than 1")
	}

	switch strings.ToUpper(pageSize) {
	case gofpdf.PageSizeA3, gofpdf.PageSizeA4, gofpdf.PageSizeA5, gofpdf.PageSizeLegal, gofpdf.PageSizeLetter:
		break
	default:
		return errorMessage("incorrect page size specified!")
	}

	switch strings.ToUpper(orientation) {
	case "L", "LANDSCAPE", "P", "PORTRAIT":
		break
	default:
		return errorMessage("incorrect orientation specified")
	}

	if dirMode && inputFile == "" {
		return errorMessage("must specify inputFile when using dirMode!")
	}

	return nil
}

//creates the pdf file at "outputFilePath" with "input" as the content
//linebreaks will be inserted here not inside the gofpdf functions
func createPdfFile(outputFilePath string, input string) error {
	dbg("createPdfFile", "creating "+outputFilePath)
	defer dbg("createPdfFile", "done "+outputFilePath)

	pdf := gofpdf.New(orientation, "pt", pageSize, "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.AddPage()
	pdf.SetFont("courier", "", float64(fontSize))

	//try creating the file line by line -> faster than just using pdf.Write(), etc...
	j := 0
	lc := 1
	for i, v := range input {
		if v == '\n' {
			//dbg("createPdfile", fmt.Sprintf("i: %d, j: %d", i, j))
			if j < i {
				pdf.Write(float64(fontSize), tr(input[j:i]))
			}
			pdf.Ln(float64(fontSize))
			if lc == lineCount {
				pdf.AddPage()
				lc = 1
			}
			lc++
			j = i + 1
		}
	}

	return pdf.OutputFileAndClose(outputFilePath)
}

func createPdfFromFile(inputFilePath string, outputFilePath string) error {
	dbg("createPdfFromFile", "converting "+inputFilePath)
	defer dbg("createPdfFromFile", "done "+inputFilePath)
	fi, err := os.Open(inputFilePath)
	defer fi.Close()
	if err == nil {
		input, err := parseInput(fi)
		if err == nil {
			return createPdfFile(outputFilePath, input)
		}
		return err
	}
	return err
}

type inOutFilePair struct {
	in  string
	out string
}

//checks if input and output folders exist, and concurrently creates the output pdf files
func ceatePdfFromFolder(inputPath string, outputPath string) error {
	dbg("createPdfFromFolder", "converting files in "+inputPath)
	defer dbg("createPdfFromFolder", "done")
	inDir, err := os.Open(inputPath)
	defer inDir.Close()
	if err != nil {
		return err
	}
	outDir, err := os.Open(outputPath)
	defer outDir.Close()
	if err != nil {
		return err
	}

	info, err := inDir.Stat()
	if err == nil {
		if !info.IsDir() {
			return errorMessage(fmt.Sprintf("%s is not a directory!", inputPath))
		}
	} else {
		return err
	}

	info, err = outDir.Stat()
	if err == nil {
		if !info.IsDir() {
			return errorMessage(fmt.Sprintf("%s is not a directory!", outputPath))
		}
	} else {
		return err
	}

	files, _ := inDir.Readdir(0)
	c := make(chan inOutFilePair, 10)
	//channel for gathering error messages. A bit messy.
	ce := make(chan string, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	//limit max goroutines to GOMAXPROCS
	wg.Add(runtime.GOMAXPROCS(0))

	// populate channel with files
	go func(c chan inOutFilePair) {
		for _, s := range files {
			if !s.IsDir() {
				filePair := inOutFilePair{
					in:  filepath.Join(inputPath, s.Name()),
					out: parseFileName(filepath.Join(outputPath, s.Name())),
				}
				dbg("createPdfFromFolder", "adding to channel: "+filePair.in)
				c <- filePair
			}
		}
		close(c)
	}(c)
	//create n files at the same time
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		go func(c chan inOutFilePair, ce chan string, wg *sync.WaitGroup) {
			defer wg.Done()
			for {
				filePair, ok := <-c
				if !ok {
					break
				}
				if err := createPdfFromFile(filePair.in, filePair.out); err != nil {
					ce <- err.Error()
					break
				}
			}
		}(c, ce, &wg)
	}
	wg.Wait()
	close(ce) //need to close the error channel
	if len(ce) > 0 {
		//Errors occured. What a mess...
		var message string
		for s := range ce {
			message += s + "\n"
		}
		return errorMessage(message)
	}
	return nil
}

func createPdfFromStdin() error {
	input, err := parseInput(os.Stdin)
	if err == nil {
		return createPdfFile(outputFile, input)
	}
	return err
}

//read from r replacing '\t' with spaces
//remove any '\r'
func parseInput(r io.Reader) (string, error) {
	spaces := make([]byte, tabSpacing)
	for i := 0; i < tabSpacing; i++ {
		spaces[i] = ' '
	}
	var err error
	if input, err := ioutil.ReadAll(r); err == nil {
		output := strings.Replace(string(input), string('\t'), string(spaces), -1)
		output = strings.Replace(output, string('\r'), "", -1)
		output = strings.Replace(output, "%", "%%", -1)
		return output, nil
	}
	return "", err
}

//remove any existing file extension and add ".pdf"
func parseFileName(file string) string {
	if suffix := path.Ext(file); suffix != "" {
		return strings.TrimSuffix(file, suffix) + ".pdf"
	}
	return file + ".pdf"
}

func main() {
	defineFlags()

	if err := flagsOkay(); err != nil {
		fmt.Println(err)
		fmt.Println()
		flag.PrintDefaults()
		os.Exit(1)
	}

	if dirMode {
		//read from directory
		if outputFile == "" {
			outputFile = inputFile
		}
		if err := ceatePdfFromFolder(inputFile, outputFile); err != nil {
			fmt.Println("Error creating pdf file(s)!", err)
			os.Exit(1)
		}
	} else if inputFile != "" {
		//read from inputfile
		if outputFile == "" {
			outputFile = parseFileName(inputFile)
		}
		if err := createPdfFromFile(inputFile, outputFile); err != nil {
			fmt.Println("Error creating pdf file!", err)
			os.Exit(1)
		}
	} else if err := createPdfFromStdin(); err != nil {
		//read from stdin
		fmt.Println("Error creating pdf file!", err)
		os.Exit(1)
	}
}

func dbg(function string, message string) {
	if verboseMode {
		fmt.Printf("%s: %s\n", function, message)
	}
}
