package markitdown

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// CsvConverter handles CSV files.
type CsvConverter struct{}

// NewCsvConverter creates a new CsvConverter.
func NewCsvConverter() *CsvConverter {
	return &CsvConverter{}
}

func (c *CsvConverter) Accepts(info StreamInfo) bool {
	if info.Extension == ".csv" {
		return true
	}
	mime := strings.ToLower(info.MIMEType)
	return strings.HasPrefix(mime, "text/csv") || strings.HasPrefix(mime, "application/csv")
}

func (c *CsvConverter) Convert(reader io.ReadSeeker, info StreamInfo) (*DocumentConverterResult, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}

	// Decode to UTF-8 using charset hint or detection
	var text string
	if info.Charset != "" {
		enc := lookupEncoding(info.Charset)
		if enc != nil {
			decoded, err := enc.NewDecoder().Bytes(data)
			if err == nil {
				text = string(decoded)
			}
		}
	}
	if text == "" {
		text = decodeWithDetection(data)
	}

	// Parse CSV
	r := csv.NewReader(strings.NewReader(text))
	r.FieldsPerRecord = -1 // allow variable fields
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse CSV: %w", err)
	}

	if len(records) == 0 {
		return &DocumentConverterResult{Markdown: ""}, nil
	}

	// Render as markdown table
	md := renderMarkdownTable(records)

	return &DocumentConverterResult{
		Markdown: md,
	}, nil
}

// renderMarkdownTable renders a 2D string slice as a markdown table.
func renderMarkdownTable(records [][]string) string {
	if len(records) == 0 {
		return ""
	}

	// Determine the number of columns from the header
	numCols := len(records[0])

	var b strings.Builder

	// Header row
	b.WriteString("| ")
	for i := 0; i < numCols; i++ {
		if i < len(records[0]) {
			b.WriteString(records[0][i])
		}
		b.WriteString(" | ")
	}
	b.WriteString("\n")

	// Separator row
	b.WriteString("| ")
	for i := 0; i < numCols; i++ {
		b.WriteString("---")
		b.WriteString(" | ")
	}
	b.WriteString("\n")

	// Data rows
	for _, row := range records[1:] {
		b.WriteString("| ")
		for i := 0; i < numCols; i++ {
			if i < len(row) {
				b.WriteString(row[i])
			}
			b.WriteString(" | ")
		}
		b.WriteString("\n")
	}

	return b.String()
}
