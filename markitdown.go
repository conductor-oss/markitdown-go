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
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

const (
	// PrioritySpecific is for format-specific converters (PDF, DOCX, etc.).
	PrioritySpecific = 0.0
	// PriorityGeneric is for fallback converters (PlainText, HTML, ZIP).
	PriorityGeneric = 10.0
)

type registeredConverter struct {
	converter DocumentConverter
	priority  float64
	name      string
}

// MarkItDown is the main document-to-markdown conversion engine.
type MarkItDown struct {
	converters   []registeredConverter
	keepDataURIs bool
	styleMap     string
}

// New creates a new MarkItDown instance with the given options.
func New(opts ...Option) *MarkItDown {
	m := &MarkItDown{}
	for _, opt := range opts {
		opt(m)
	}
	m.enableBuiltins()
	return m
}

// RegisterConverter adds a custom converter with the given priority.
// Lower priority values are tried first.
func (m *MarkItDown) RegisterConverter(name string, c DocumentConverter, priority float64) {
	m.converters = append(m.converters, registeredConverter{
		converter: c,
		priority:  priority,
		name:      name,
	})
	sort.SliceStable(m.converters, func(i, j int) bool {
		return m.converters[i].priority < m.converters[j].priority
	})
}

// Convert auto-detects the source type (file path or URL) and converts it.
func (m *MarkItDown) Convert(source string) (*DocumentConverterResult, error) {
	// Check if it's a URL
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return m.ConvertURL(source)
	}
	return m.ConvertFile(source)
}

// ConvertFile converts a local file to markdown.
func (m *MarkItDown) ConvertFile(path string) (*DocumentConverterResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(path))
	filename := filepath.Base(path)

	info := StreamInfo{
		Extension: ext,
		Filename:  filename,
		LocalPath: path,
	}

	// Detect MIME type
	info.MIMEType = detectMIMEType(f, ext)

	// Reset after MIME detection
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	return m.ConvertReader(f, info)
}

// ConvertReader converts a stream to markdown using the provided StreamInfo.
func (m *MarkItDown) ConvertReader(r io.ReadSeeker, info StreamInfo) (*DocumentConverterResult, error) {
	return m.convert(r, info)
}

// ConvertURL fetches a URL and converts the response to markdown.
func (m *MarkItDown) ConvertURL(url string) (*DocumentConverterResult, error) {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("fetch URL: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	reader := bytes.NewReader(data)

	// Build StreamInfo from response
	info := StreamInfo{
		URL: url,
	}

	// Extract content type
	ct := resp.Header.Get("Content-Type")
	if ct != "" {
		parts := strings.Split(ct, ";")
		info.MIMEType = strings.TrimSpace(parts[0])
		for _, p := range parts[1:] {
			p = strings.TrimSpace(p)
			if strings.HasPrefix(p, "charset=") {
				info.Charset = strings.Trim(strings.TrimPrefix(p, "charset="), `"'`)
			}
		}
	}

	// Extract extension from URL path
	urlPath := strings.Split(url, "?")[0]
	info.Extension = strings.ToLower(filepath.Ext(urlPath))

	// Extract filename from URL
	if info.Extension != "" {
		info.Filename = filepath.Base(urlPath)
	}

	// Detect MIME if not provided
	if info.MIMEType == "" {
		info.MIMEType = detectMIMEType(reader, info.Extension)
		reader.Seek(0, io.SeekStart)
	}

	return m.ConvertReader(reader, info)
}

// convert is the internal dispatch method.
func (m *MarkItDown) convert(r io.ReadSeeker, info StreamInfo) (*DocumentConverterResult, error) {
	var failedAttempts []FailedConversionAttempt

	for _, rc := range m.converters {
		if !rc.converter.Accepts(info) {
			continue
		}

		// Reset reader position before conversion
		if _, err := r.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("seek: %w", err)
		}

		result, err := rc.converter.Convert(r, info)
		if err != nil {
			failedAttempts = append(failedAttempts, FailedConversionAttempt{
				Converter: rc.name,
				Err:       err,
			})
			continue
		}

		// Post-process / normalize output
		result.Markdown = normalizeOutput(result.Markdown)
		return result, nil
	}

	if len(failedAttempts) > 0 {
		return nil, &ConversionError{Attempts: failedAttempts}
	}

	return nil, &UnsupportedFormatError{
		Extension: info.Extension,
		MIMEType:  info.MIMEType,
	}
}

// enableBuiltins registers all built-in converters.
func (m *MarkItDown) enableBuiltins() {
	// Specific format converters (priority 0.0 - tried first)
	m.RegisterConverter("csv", NewCsvConverter(), PrioritySpecific)
	m.RegisterConverter("rss", NewRSSConverter(), PrioritySpecific)
	m.RegisterConverter("ipynb", NewIpynbConverter(), PrioritySpecific)
	m.RegisterConverter("docx", NewDocxConverter(m), PrioritySpecific)
	m.RegisterConverter("xlsx", NewXlsxConverter(), PrioritySpecific)
	m.RegisterConverter("xls", NewXlsConverter(), PrioritySpecific)
	m.RegisterConverter("pptx", NewPptxConverter(m), PrioritySpecific)
	m.RegisterConverter("pdf", NewPdfConverter(), PrioritySpecific)
	m.RegisterConverter("epub", NewEpubConverter(m), PrioritySpecific)

	// Generic format converters (priority 10.0 - tried last as fallbacks)
	m.RegisterConverter("html", NewHTMLConverter(m), PriorityGeneric)
	m.RegisterConverter("zip", NewZipConverter(m), PriorityGeneric)
	m.RegisterConverter("plaintext", NewPlainTextConverter(), PriorityGeneric)
}

// detectMIMEType detects the MIME type from content and extension.
func detectMIMEType(r io.ReadSeeker, ext string) string {
	// Try content-based detection first
	mtype, err := mimetype.DetectReader(r)
	if err == nil && mtype.String() != "application/octet-stream" {
		return mtype.String()
	}

	// Fall back to extension-based detection
	return mimeFromExtension(ext)
}

// mimeFromExtension returns a MIME type for common extensions.
func mimeFromExtension(ext string) string {
	extMap := map[string]string{
		".pdf":      "application/pdf",
		".docx":     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".pptx":     "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".xlsx":     "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".xls":      "application/vnd.ms-excel",
		".html":     "text/html",
		".htm":      "text/html",
		".csv":      "text/csv",
		".txt":      "text/plain",
		".text":     "text/plain",
		".md":       "text/markdown",
		".markdown": "text/markdown",
		".json":     "application/json",
		".jsonl":    "application/jsonl",
		".xml":      "text/xml",
		".rss":      "application/rss+xml",
		".atom":     "application/atom+xml",
		".epub":     "application/epub+zip",
		".zip":      "application/zip",
		".ipynb":    "application/x-ipynb+json",
	}
	if m, ok := extMap[ext]; ok {
		return m
	}
	return "application/octet-stream"
}
