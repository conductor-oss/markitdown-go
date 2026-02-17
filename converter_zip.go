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
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// ZipConverter handles ZIP files by recursively converting their contents.
type ZipConverter struct {
	markitdown *MarkItDown
}

// NewZipConverter creates a new ZipConverter.
func NewZipConverter(m *MarkItDown) *ZipConverter {
	return &ZipConverter{markitdown: m}
}

func (c *ZipConverter) Accepts(info StreamInfo) bool {
	if info.Extension == ".zip" {
		return true
	}
	mime := strings.ToLower(info.MIMEType)
	return strings.HasPrefix(mime, "application/zip")
}

func (c *ZipConverter) Convert(reader io.ReadSeeker, info StreamInfo) (*DocumentConverterResult, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read ZIP: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open ZIP: %w", err)
	}

	var md strings.Builder
	filename := info.Filename
	if filename == "" {
		filename = "archive"
	}
	md.WriteString(fmt.Sprintf("Content from the zip file `%s`:\n\n", filename))

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}

		fileData, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		ext := strings.ToLower(filepath.Ext(f.Name))
		fileInfo := StreamInfo{
			Extension: ext,
			Filename:  filepath.Base(f.Name),
		}

		// Detect MIME type
		fileReader := bytes.NewReader(fileData)
		fileInfo.MIMEType = detectMIMEType(fileReader, ext)
		fileReader.Seek(0, io.SeekStart)

		// Try to convert
		result, err := c.markitdown.ConvertReader(fileReader, fileInfo)
		if err != nil {
			// Skip files that can't be converted
			continue
		}

		if strings.TrimSpace(result.Markdown) != "" {
			md.WriteString(fmt.Sprintf("## File: %s\n", f.Name))
			md.WriteString(result.Markdown)
			md.WriteString("\n\n")
		}
	}

	return &DocumentConverterResult{
		Markdown: md.String(),
	}, nil
}
