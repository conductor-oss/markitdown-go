# markitdown-go

[![Go Reference](https://pkg.go.dev/badge/github.com/conductoross/markitdown-go.svg)](https://pkg.go.dev/github.com/conductoross/markitdown-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/conductoross/markitdown-go)](https://goreportcard.com/report/github.com/conductoross/markitdown-go)
[![Build](https://github.com/conductoross/markitdown-go/actions/workflows/ci.yml/badge.svg)](https://github.com/conductoross/markitdown-go/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/conductoross/markitdown-go)](https://github.com/conductoross/markitdown-go/releases)

A pure-Go library and CLI that converts documents to Markdown. Go port of the Python [markitdown](https://github.com/microsoft/markitdown) library.

## Features
- Pure Go, no CGO, no external runtime dependencies
- 12 format converters: PDF, DOCX, PPTX, XLSX, XLS, HTML, RSS/Atom, CSV, EPUB, Jupyter, plain text, ZIP
- Deterministic output with golden test suite
- Optional PDFium PDF backend via WebAssembly (no CGO)

## Supported formats

| Format | Extensions | Notes |
|--------|------------|-------|
| PDF | `.pdf` | Text extraction via PDFium (WebAssembly, no CGO) |
| Word | `.docx` | Headings, tables, lists, hyperlinks, comments, math (OMML to LaTeX) |
| PowerPoint | `.pptx` | Slides, tables, notes, image alt text |
| Excel | `.xlsx` | Multi-sheet markdown tables |
| Excel (legacy) | `.xls` | Multi-sheet markdown tables |
| HTML | `.html`, `.htm` | Full HTML-to-Markdown conversion |
| RSS/Atom | `.xml`, `.rss`, `.atom` | Feed items with titles, dates, content |
| CSV | `.csv` | Markdown table with auto charset detection |
| EPUB | `.epub` | Metadata, table of contents, chapter content |
| Jupyter | `.ipynb` | Markdown + fenced code cells with output |
| Plain text | `.txt`, `.md`, `.json`, `.jsonl` | Charset detection and UTF-8 conversion |
| ZIP | `.zip` | Recursively converts supported files inside |

## Install

```bash
go get github.com/conductoross/markitdown-go
```

## Library quick start

```go
package main

import (
	"fmt"
	"log"

	markitdown "github.com/conductoross/markitdown-go"
)

func main() {
	m := markitdown.New()

	// Convert a local file
	result, err := m.ConvertFile("report.pdf")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Markdown)
}
```

### More examples

```go
// Convert a URL
result, err := m.ConvertURL("https://example.com/page.html")

// Convert with auto-detection (file path or URL)
result, err := m.Convert("report.pdf")
result, err := m.Convert("https://example.com/page.html")

// Convert from a reader with metadata hints
f, _ := os.Open("data.csv")
result, err := m.ConvertReader(f, markitdown.StreamInfo{
	Extension: ".csv",
	MIMEType:  "text/csv",
	Charset:   "shift_jis",
})

// Options
m := markitdown.New(
	markitdown.WithKeepDataURIs(true), // preserve base64 data URIs in output
)
```

## CLI quick start

Build:
```bash
go build -o markitdown ./cmd/markitdown
```

Convert a file to stdout:
```bash
./markitdown report.pdf
```

Convert and write to a file:
```bash
./markitdown -o output.md report.docx
```

Convert from stdin with format hint:
```bash
cat data.csv | ./markitdown -x csv
```

Convert a URL:
```bash
./markitdown https://example.com/page.html
```

### CLI flags

```
Usage: markitdown [flags] [source]

Arguments:
  source    File path or URL to convert (reads stdin if omitted)

Flags:
  -o, --output string       Output file (default: stdout)
  -x, --extension string    File extension hint for stdin input (e.g. "pdf", ".csv")
  -m, --mime-type string    MIME type hint
  -c, --charset string      Charset hint (e.g. "shift_jis", "utf-8")
  -v, --version             Show version
      --keep-data-uris      Keep full base64-encoded data URIs in output
```

## PDF backend

PDF extraction uses [PDFium](https://pdfium.googlesource.com/pdfium/) via [go-pdfium](https://github.com/klippa-app/go-pdfium) running in a [Wazero](https://github.com/tetratelabs/wazero) WebAssembly sandbox. This is the default -- no build tags or external dependencies needed. It produces high-quality text with proper word boundaries and spacing.

For a lighter-weight build (~8 MB smaller binary), you can opt into the [ledongthuc/pdf](https://github.com/ledongthuc/pdf) backend instead:

```bash
go build -tags nopdfium ./...
```

Note: the `nopdfium` backend has limited word boundary detection and may produce text without spaces.

## Notes
- PDF extraction is text-based; image-only PDFs produce no output without OCR.
- DOCX math equations (OMML) are converted to LaTeX notation.
- CJK charset detection works without hints but is most reliable when `Charset` is provided in `StreamInfo`.
