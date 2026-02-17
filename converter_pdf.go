//go:build nopdfium

package markitdown

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PdfConverter handles PDF files.
type PdfConverter struct{}

// NewPdfConverter creates a new PdfConverter.
func NewPdfConverter() *PdfConverter {
	return &PdfConverter{}
}

func (c *PdfConverter) Accepts(info StreamInfo) bool {
	if info.Extension == ".pdf" {
		return true
	}
	mime := strings.ToLower(info.MIMEType)
	return strings.HasPrefix(mime, "application/pdf")
}

func (c *PdfConverter) Convert(reader io.ReadSeeker, info StreamInfo) (*DocumentConverterResult, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read PDF: %w", err)
	}

	pdfReader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open PDF: %w", err)
	}

	var md strings.Builder
	numPages := pdfReader.NumPage()

	for i := 1; i <= numPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}

		text := c.extractPageText(page)
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		md.WriteString(text)
		md.WriteString("\n\n")
	}

	result := md.String()
	if strings.TrimSpace(result) == "" {
		return &DocumentConverterResult{
			Markdown: "[No readable text content found in PDF]",
		}, nil
	}

	return &DocumentConverterResult{
		Markdown: result,
	}, nil
}

// pdfTextElement represents a positioned text element on a PDF page.
type pdfTextElement struct {
	x    float64
	y    float64
	text string
	size float64
}

// pdfLine represents a line of text on a PDF page.
type pdfLine struct {
	y        float64
	elements []pdfTextElement
}

// extractPageText extracts text from a single PDF page using GetTextByRow,
// falling back to position-based extraction from Content().Text.
func (c *PdfConverter) extractPageText(page pdf.Page) string {
	// Use GetTextByRow to extract text with word boundary detection
	rows, err := page.GetTextByRow()
	if err == nil && len(rows) > 0 {
		var result strings.Builder
		for _, row := range rows {
			var lineText strings.Builder
			prevWasEmpty := false
			for _, word := range row.Content {
				s := word.S
				if s == "" {
					prevWasEmpty = true
					continue
				}
				if lineText.Len() > 0 && prevWasEmpty {
					// Empty string between non-empty strings = word boundary
					last := lineText.String()
					if len(last) > 0 && last[len(last)-1] != ' ' {
						lineText.WriteString(" ")
					}
				}
				lineText.WriteString(s)
				prevWasEmpty = false
			}
			text := strings.TrimSpace(lineText.String())
			if text != "" {
				result.WriteString(text)
				result.WriteString("\n")
			}
		}
		text := result.String()
		if strings.TrimSpace(text) != "" {
			return text
		}
	}

	// Fallback: character-level extraction with position data
	content := page.Content()
	if len(content.Text) == 0 {
		return ""
	}

	var elements []pdfTextElement
	for _, t := range content.Text {
		if strings.TrimSpace(t.S) == "" {
			continue
		}
		elements = append(elements, pdfTextElement{
			x:    t.X,
			y:    t.Y,
			text: t.S,
			size: t.FontSize,
		})
	}

	if len(elements) == 0 {
		return ""
	}

	// Group into lines based on Y proximity
	yTolerance := 3.0
	if len(elements) > 0 && elements[0].size > 0 {
		yTolerance = elements[0].size * 0.3
	}

	var lines []pdfLine
	for _, elem := range elements {
		found := false
		for i := range lines {
			if pdfAbs(lines[i].y-elem.y) < yTolerance {
				lines[i].elements = append(lines[i].elements, elem)
				found = true
				break
			}
		}
		if !found {
			lines = append(lines, pdfLine{y: elem.y, elements: []pdfTextElement{elem}})
		}
	}

	// Sort lines by Y descending (top to bottom in PDF coordinates)
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].y > lines[j].y
	})

	var result strings.Builder
	for _, ln := range lines {
		sort.Slice(ln.elements, func(i, j int) bool {
			return ln.elements[i].x < ln.elements[j].x
		})

		var lineText strings.Builder
		var lastX float64
		var lastWidth float64
		first := true

		for _, elem := range ln.elements {
			if !first {
				gap := elem.x - (lastX + lastWidth)
				// Use font-size-relative threshold for word spacing
				threshold := elem.size * 0.2
				if threshold < 1.0 {
					threshold = 1.0
				}
				if gap > threshold {
					lineText.WriteString(" ")
				}
			}
			lineText.WriteString(elem.text)
			lastX = elem.x
			// Better width estimation: use font size * character count * average width ratio
			lastWidth = float64(len([]rune(elem.text))) * elem.size * 0.55
			first = false
		}

		text := lineText.String()
		if strings.TrimSpace(text) != "" {
			result.WriteString(text)
			result.WriteString("\n")
		}
	}

	return result.String()
}

func pdfAbs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
