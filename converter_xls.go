package markitdown

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/extrame/xls"
)

// XlsConverter handles legacy XLS files.
type XlsConverter struct{}

// NewXlsConverter creates a new XlsConverter.
func NewXlsConverter() *XlsConverter {
	return &XlsConverter{}
}

func (c *XlsConverter) Accepts(info StreamInfo) bool {
	if info.Extension == ".xls" {
		return true
	}
	mime := strings.ToLower(info.MIMEType)
	return strings.HasPrefix(mime, "application/vnd.ms-excel")
}

func (c *XlsConverter) Convert(reader io.ReadSeeker, info StreamInfo) (*DocumentConverterResult, error) {
	// extrame/xls requires a file path, so we need to write to a temp file
	tmpFile, err := os.CreateTemp("", "markitdown-*.xls")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, reader); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	tmpFile.Close()

	wb, err := xls.Open(tmpPath, "utf-8")
	if err != nil {
		return nil, fmt.Errorf("open XLS: %w", err)
	}

	var md strings.Builder

	for i := 0; i < wb.NumSheets(); i++ {
		sheet := wb.GetSheet(i)
		if sheet == nil {
			continue
		}

		sheetName := sheet.Name
		if sheetName == "" {
			sheetName = fmt.Sprintf("Sheet%d", i+1)
		}

		// Collect all rows
		var rows [][]string
		maxRow := int(sheet.MaxRow)
		for rowIdx := 0; rowIdx <= maxRow; rowIdx++ {
			row := sheet.Row(rowIdx)
			if row == nil {
				continue
			}

			var cells []string
			lastCol := row.LastCol()
			for colIdx := 0; colIdx < lastCol; colIdx++ {
				cells = append(cells, row.Col(colIdx))
			}
			rows = append(rows, cells)
		}

		if len(rows) == 0 {
			continue
		}

		fmt.Fprintf(&md, "## %s\n", sheetName)
		md.WriteString(renderMarkdownTable(rows))
		md.WriteString("\n")
	}

	return &DocumentConverterResult{
		Markdown: md.String(),
	}, nil
}
