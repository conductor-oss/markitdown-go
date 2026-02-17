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
	"regexp"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"golang.org/x/net/html"
)

// HTMLConverter handles HTML files.
type HTMLConverter struct {
	markitdown *MarkItDown
}

// NewHTMLConverter creates a new HTMLConverter.
func NewHTMLConverter(m *MarkItDown) *HTMLConverter {
	return &HTMLConverter{markitdown: m}
}

func (c *HTMLConverter) Accepts(info StreamInfo) bool {
	switch info.Extension {
	case ".html", ".htm":
		return true
	}
	mime := strings.ToLower(info.MIMEType)
	return strings.HasPrefix(mime, "text/html") || strings.HasPrefix(mime, "application/xhtml")
}

func (c *HTMLConverter) Convert(reader io.ReadSeeker, info StreamInfo) (*DocumentConverterResult, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}

	htmlStr := string(data)

	result, err := c.ConvertString(htmlStr)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ConvertString converts an HTML string to markdown.
func (c *HTMLConverter) ConvertString(htmlStr string) (*DocumentConverterResult, error) {
	// Extract title from HTML
	title := extractHTMLTitle(htmlStr)

	// Remove script and style tags before conversion
	htmlStr = removeScriptAndStyle(htmlStr)

	// Convert HTML to Markdown
	md, err := convertHTMLToMarkdown(htmlStr)
	if err != nil {
		return nil, fmt.Errorf("convert HTML to markdown: %w", err)
	}

	// Post-process: truncate data URIs unless configured to keep them
	keepDataURIs := false
	if c.markitdown != nil {
		keepDataURIs = c.markitdown.keepDataURIs
	}
	if !keepDataURIs {
		md = truncateDataURIs(md)
	}

	return &DocumentConverterResult{
		Markdown: md,
		Title:    title,
	}, nil
}

// convertHTMLToMarkdown converts HTML to markdown using html-to-markdown.
func convertHTMLToMarkdown(htmlStr string) (string, error) {
	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(
				commonmark.WithHeadingStyle("atx"),
			),
			table.NewTablePlugin(),
		),
	)

	md, err := conv.ConvertString(htmlStr)
	if err != nil {
		return "", err
	}

	return md, nil
}

var (
	reScript  = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)
	reStyle   = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`)
	reDataURI = regexp.MustCompile(`(data:[a-zA-Z0-9/+.-]+;base64,)[A-Za-z0-9+/=]{64,}`)
)

// removeScriptAndStyle removes <script> and <style> tags and their content.
func removeScriptAndStyle(htmlStr string) string {
	htmlStr = reScript.ReplaceAllString(htmlStr, "")
	htmlStr = reStyle.ReplaceAllString(htmlStr, "")
	return htmlStr
}

// truncateDataURIs truncates large base64 data URIs to data:mime/type;base64...
func truncateDataURIs(md string) string {
	return reDataURI.ReplaceAllString(md, "${1}...")
}

// extractHTMLTitle extracts the title from an HTML document.
func extractHTMLTitle(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}

	var title string
	var findTitle func(*html.Node)
	findTitle = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" {
			if n.FirstChild != nil {
				title = n.FirstChild.Data
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findTitle(c)
			if title != "" {
				return
			}
		}
	}
	findTitle(doc)

	return strings.TrimSpace(title)
}
