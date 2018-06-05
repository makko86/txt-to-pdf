# txt-to-pdf
Tool for converting a text file to pdf


Usage of ./txt-to-pdf:
  -dir
    	Optional. If set "inputFile" and "outpuFile" will be treated as directories. txt-to-pdf will try to parse every single file in "inputFile"
  -fs int
    	Fontsize to use. (default 10)
  -if string
    	input file. Optional: Can be omitted to use STDIN as input
  -of string
    	output file Optional: Can be omitted if "inputFile" is specified. In this case "inputFile".pdf will be used as outputFile
  -po string
    	Page orientation to use. Possible values: L, P (default "P")
  -ps string
    	Page size to use. Possible values: A3, A4, A5, Letter, Legal. (default "A4")
  -ts int
    	Number of spaces used to replace tabstops. (default 8)



Uses github.com/jung-kurt/gofpdf