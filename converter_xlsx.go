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

package markitdown

import (
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

// XlsxConverter handles XLSX files.
type XlsxConverter struct{}

// NewXlsxConverter creates a new XlsxConverter.
func NewXlsxConverter() *XlsxConverter {
	return &XlsxConverter{}
}

func (c *XlsxConverter) Accepts(info StreamInfo) bool {
	if info.Extension == ".xlsx" {
		return true
	}
	mime := strings.ToLower(info.MIMEType)
	return strings.HasPrefix(mime, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
}

func (c *XlsxConverter) Convert(reader io.ReadSeeker, info StreamInfo) (*DocumentConverterResult, error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, fmt.Errorf("open XLSX: %w", err)
	}
	defer f.Close()

	var md strings.Builder
	sheets := f.GetSheetList()

	for _, sheet := range sheets {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		if len(rows) == 0 {
			continue
		}

		// Sheet heading
		fmt.Fprintf(&md, "## %s\n", sheet)

		// Render as markdown table
		table := renderMarkdownTable(rows)
		md.WriteString(table)
		md.WriteString("\n")
	}

	return &DocumentConverterResult{
		Markdown: md.String(),
	}, nil
}
