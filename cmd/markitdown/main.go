// Copyright 2026 Conductor OSS
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
// specific language governing permissions and limitations under the License.

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	markitdown "github.com/nicholasgasior/markitdown-go"
)

var version = "dev"

func main() {
	var (
		output        string
		extension     string
		mimeType      string
		charset       string
		showVersion   bool
		keepDataURIs  bool
	)

	flag.StringVar(&output, "o", "", "Output file (default: stdout)")
	flag.StringVar(&output, "output", "", "Output file (default: stdout)")
	flag.StringVar(&extension, "x", "", "File extension hint (for stdin input)")
	flag.StringVar(&extension, "extension", "", "File extension hint (for stdin input)")
	flag.StringVar(&mimeType, "m", "", "MIME type hint")
	flag.StringVar(&mimeType, "mime-type", "", "MIME type hint")
	flag.StringVar(&charset, "c", "", "Charset hint")
	flag.StringVar(&charset, "charset", "", "Charset hint")
	flag.BoolVar(&showVersion, "v", false, "Show version")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.BoolVar(&keepDataURIs, "keep-data-uris", false, "Keep full base64-encoded data URIs")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: markitdown [flags] [source]\n\n")
		fmt.Fprintf(os.Stderr, "Convert documents to Markdown.\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  source    File path or URL to convert (reads stdin if omitted)\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if showVersion {
		fmt.Printf("markitdown %s\n", version)
		os.Exit(0)
	}

	// Normalize extension
	if extension != "" {
		extension = strings.ToLower(extension)
		if !strings.HasPrefix(extension, ".") {
			extension = "." + extension
		}
	}

	// Create MarkItDown instance
	var opts []markitdown.Option
	if keepDataURIs {
		opts = append(opts, markitdown.WithKeepDataURIs(true))
	}
	m := markitdown.New(opts...)

	var result *markitdown.DocumentConverterResult
	var err error

	args := flag.Args()

	if len(args) == 0 {
		// Read from stdin
		data, readErr := io.ReadAll(os.Stdin)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", readErr)
			os.Exit(1)
		}

		info := markitdown.StreamInfo{
			Extension: extension,
			MIMEType:  mimeType,
			Charset:   charset,
		}

		reader := newBytesReadSeeker(data)
		if info.MIMEType == "" && info.Extension != "" {
			info.MIMEType = mimeFromExt(info.Extension)
		}
		result, err = m.ConvertReader(reader, info)
	} else {
		source := args[0]
		if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
			result, err = m.ConvertURL(source)
		} else {
			result, err = m.ConvertFile(source)
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Write output
	if output != "" {
		dir := filepath.Dir(output)
		if dir != "." {
			os.MkdirAll(dir, 0o755)
		}
		if writeErr := os.WriteFile(output, []byte(result.Markdown+"\n"), 0o644); writeErr != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", writeErr)
			os.Exit(1)
		}
	} else {
		fmt.Print(result.Markdown)
		fmt.Println()
	}
}

// bytesReadSeeker wraps a byte slice as io.ReadSeeker.
type bytesReadSeeker struct {
	*strings.Reader
}

func newBytesReadSeeker(data []byte) io.ReadSeeker {
	return strings.NewReader(string(data))
}

// mimeFromExt returns a MIME type for common extensions (CLI use only).
func mimeFromExt(ext string) string {
	m := map[string]string{
		".pdf":  "application/pdf",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".xls":  "application/vnd.ms-excel",
		".html": "text/html",
		".htm":  "text/html",
		".csv":  "text/csv",
		".txt":  "text/plain",
		".json": "application/json",
		".xml":  "text/xml",
		".rss":  "application/rss+xml",
		".epub": "application/epub+zip",
		".zip":  "application/zip",
	}
	if v, ok := m[ext]; ok {
		return v
	}
	return ""
}
