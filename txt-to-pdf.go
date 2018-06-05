package main

import (
	"flag"
	"fmt"
	"io"
	"os"
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

//Flag-defaults
const (
	defaultInputFile   = ""
	defaultOutputFile  = ""
	defaultFontSize    = 10
	defaultPageSize    = "A4"
	defaultOrientation = "P"
	defaultTabSpacing  = 8
	defaultDirMode     = false
)

//Flag usages
const (
	usageInputFile   = "input file. Optional: Can be omitted to use STDIN as input"
	usageOutputFile  = "output file Optional: Can be omitted if \"inputFile\" is specified. In this case \"inputFile\".pdf will be used as outputFile"
	usageFontSize    = "Fontsize to use."
	usagePageSize    = "Page size to use. Possible values: A3, A4, A5, Letter, Legal."
	usageOrientation = "Page orientation to use. Possible values: L, P"
	usageTabSpacing  = "Number of spaces used to replace tabstops."
	usageDirMode     = "Optional. If set \"inputFile\" and \"outpuFile\" will be treated as directories. txt-to-pdf will try to parse every single file in \"inputFile\""
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
	flag.Parse()
}

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

func createPdfFile(outputFilePath string, input string) error {
	pdf := gofpdf.New(orientation, "pt", pageSize, "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.AddPage()
	pdf.SetFont("courier", "", float64(fontSize))
	pdf.MultiCell(0, float64(fontSize), tr(input), "", "", false)

	return pdf.OutputFileAndClose(outputFilePath)
}

func createPdfFromFile(inputFilePath string, outputFilePath string) error {
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

func ceatePdfFromFolder(inputPath string, outputPath string) error {
	dir, err := os.Open(inputPath)
	defer dir.Close()
	if err == nil {
		info, err := dir.Stat()
		if err == nil {
			if info.IsDir() {
				files, _ := dir.Readdir(0)
				c := make(chan inOutFilePair, 10)
				//channel for gathering error messages
				ce := make(chan string, runtime.GOMAXPROCS(0))
				var wg sync.WaitGroup
				//limit max goroutines to GOMAXPROCS
				wg.Add(runtime.GOMAXPROCS(0))

				// populate channel with files
				go func(c chan inOutFilePair) {
					for _, s := range files {
						if !s.IsDir() {
							filePair := inOutFilePair{
								in:  inputPath + s.Name(),
								out: outputPath + s.Name() + ".pdf",
							}
							c <- filePair
						}
					}
					close(c)
				}(c)

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
						message += s + " "
					}
					return errorMessage(message)
				}
				return nil
			}
			return errorMessage(fmt.Sprintf("%s is not a directory!", inputPath))
		}
	}
	return err
}

func createPdfFromStdin() error {
	input, err := parseInput(os.Stdin)
	if err == nil {
		return createPdfFile(outputFile, input)
	}
	return err
}

func parseInput(r io.Reader) (string, error) {
	//read from r replacing '/t' with spaces
	cap := 512
	buf := make([]byte, cap)
	var b strings.Builder
	b.Grow(cap)
	var n int
	var err error
	for err == nil {
		n, err = r.Read(buf)
		for i := 0; i < n; i++ {
			if buf[i] != '\t' {
				b.WriteByte(buf[i])
			} else {
				for j := 0; j < tabSpacing; j++ {
					b.WriteByte(' ')
				}
			}
		}

	}
	if err == io.EOF {
		return b.String(), nil
	}
	return "", err
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
			outputFile = inputFile + ".pdf"
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
