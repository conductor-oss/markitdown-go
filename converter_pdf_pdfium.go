//go:build !nopdfium

package markitdown

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/klippa-app/go-pdfium/webassembly"
)

var (
	pdfiumPool     pdfium.Pool
	pdfiumPoolOnce sync.Once
	pdfiumPoolErr  error
)

func initPdfiumPool() {
	pdfiumPool, pdfiumPoolErr = webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
}

// PdfConverter handles PDF files using the PDFium library via WebAssembly.
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
	pdfiumPoolOnce.Do(initPdfiumPool)
	if pdfiumPoolErr != nil {
		return nil, fmt.Errorf("init pdfium: %w", pdfiumPoolErr)
	}

	instance, err := pdfiumPool.GetInstance(30 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("get pdfium instance: %w", err)
	}
	defer instance.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read PDF: %w", err)
	}

	doc, err := instance.OpenDocument(&requests.OpenDocument{
		File: &data,
	})
	if err != nil {
		return nil, fmt.Errorf("open PDF: %w", err)
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	pageCountResp, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: doc.Document,
	})
	if err != nil {
		return nil, fmt.Errorf("get page count: %w", err)
	}

	var md strings.Builder

	for i := 0; i < pageCountResp.PageCount; i++ {
		text := c.extractStructuredPage(instance, doc, i)
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

// pdfRect represents a text rectangle with font metadata from PDFium.
type pdfRect struct {
	text     string
	left     float64
	top      float64
	right    float64
	bottom   float64
	fontSize float64
	fontName string
}

// pdfTextLine represents a line of text built from grouped rects.
type pdfTextLine struct {
	rects    []pdfRect
	top      float64
	bottom   float64
	left     float64
	fontSize float64 // dominant font size on this line
	fontName string  // dominant font name on this line
}

func (l *pdfTextLine) text() string {
	var b strings.Builder
	for _, r := range l.rects {
		b.WriteString(r.text)
	}
	return b.String()
}

// extractStructuredPage extracts text from a page with markdown formatting.
func (c *PdfConverter) extractStructuredPage(instance pdfium.Pdfium, doc *responses.OpenDocument, pageIdx int) string {
	structured, err := instance.GetPageTextStructured(&requests.GetPageTextStructured{
		Page: requests.Page{
			ByIndex: &requests.PageByIndex{
				Document: doc.Document,
				Index:    pageIdx,
			},
		},
		Mode:                   requests.GetPageTextStructuredModeRects,
		CollectFontInformation: true,
	})
	if err != nil || len(structured.Rects) == 0 {
		// Fallback to plain text
		return c.extractPlainPage(instance, doc, pageIdx)
	}

	// Convert rects to our type
	var rects []pdfRect
	for _, r := range structured.Rects {
		text := r.Text
		if strings.TrimSpace(text) == "" {
			continue
		}
		pr := pdfRect{
			text:   text,
			left:   r.PointPosition.Left,
			top:    r.PointPosition.Top,
			right:  r.PointPosition.Right,
			bottom: r.PointPosition.Bottom,
		}
		if r.FontInformation != nil {
			pr.fontSize = r.FontInformation.Size
			pr.fontName = r.FontInformation.Name
		}
		rects = append(rects, pr)
	}

	if len(rects) == 0 {
		return ""
	}

	// Group rects into lines by Y position
	lines := groupRectsIntoLines(rects)

	// Determine the body font size (most common size)
	bodySize := detectBodyFontSize(lines)

	// Render lines as markdown
	return renderMarkdownFromLines(lines, bodySize)
}

// extractPlainPage is the fallback plain text extractor.
func (c *PdfConverter) extractPlainPage(instance pdfium.Pdfium, doc *responses.OpenDocument, pageIdx int) string {
	textResp, err := instance.GetPageText(&requests.GetPageText{
		Page: requests.Page{
			ByIndex: &requests.PageByIndex{
				Document: doc.Document,
				Index:    pageIdx,
			},
		},
	})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(textResp.Text)
}

// groupRectsIntoLines groups rects by their vertical position into lines,
// sorted top-to-bottom, with rects within each line sorted left-to-right.
func groupRectsIntoLines(rects []pdfRect) []pdfTextLine {
	// Sort by top position descending (PDF coordinates: top of page = highest value)
	sort.Slice(rects, func(i, j int) bool {
		if math.Abs(rects[i].top-rects[j].top) < 2 {
			return rects[i].left < rects[j].left
		}
		return rects[i].top > rects[j].top
	})

	var lines []pdfTextLine
	for _, r := range rects {
		merged := false
		for i := range lines {
			// Same line if vertical overlap is significant
			if math.Abs(lines[i].top-r.top) < 3 {
				lines[i].rects = append(lines[i].rects, r)
				if r.left < lines[i].left {
					lines[i].left = r.left
				}
				merged = true
				break
			}
		}
		if !merged {
			lines = append(lines, pdfTextLine{
				rects:  []pdfRect{r},
				top:    r.top,
				bottom: r.bottom,
				left:   r.left,
			})
		}
	}

	// Sort lines top-to-bottom (descending top coordinate = top of page first)
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].top > lines[j].top
	})

	// Sort rects within each line left-to-right, and determine dominant font
	for i := range lines {
		sort.Slice(lines[i].rects, func(a, b int) bool {
			return lines[i].rects[a].left < lines[i].rects[b].left
		})
		lines[i].fontSize, lines[i].fontName = dominantFont(lines[i].rects)
	}

	return lines
}

// dominantFont returns the font size and name that covers the most text in a line.
func dominantFont(rects []pdfRect) (float64, string) {
	type fontKey struct {
		size float64
		name string
	}
	counts := map[fontKey]int{}
	for _, r := range rects {
		k := fontKey{size: math.Round(r.fontSize*10) / 10, name: r.fontName}
		counts[k] += len(r.text)
	}
	var bestKey fontKey
	bestCount := 0
	for k, c := range counts {
		if c > bestCount {
			bestCount = c
			bestKey = k
		}
	}
	return bestKey.size, bestKey.name
}

// detectBodyFontSize finds the most common font size across all lines
// (weighted by character count), which represents the body text.
func detectBodyFontSize(lines []pdfTextLine) float64 {
	sizeCounts := map[float64]int{}
	for _, l := range lines {
		for _, r := range l.rects {
			rounded := math.Round(r.fontSize*10) / 10
			sizeCounts[rounded] += len(strings.TrimSpace(r.text))
		}
	}

	var bodySize float64
	maxCount := 0
	for size, count := range sizeCounts {
		if count > maxCount {
			maxCount = count
			bodySize = size
		}
	}
	return bodySize
}

// fontIsBold returns true if the font name suggests bold weight.
func fontIsBold(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "bold") ||
		strings.Contains(lower, "medi") || // e.g. NimbusRomNo9L-Medi
		strings.HasSuffix(lower, "-bd") ||
		strings.HasSuffix(lower, "bd")
}

// fontIsItalic returns true if the font name suggests italic style.
func fontIsItalic(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "ital") ||
		strings.Contains(lower, "obli") ||
		strings.HasSuffix(lower, "-it")
}

// allRectsAreBold returns true if all non-whitespace rects in the slice use a bold font.
func allRectsAreBold(rects []pdfRect) bool {
	for _, r := range rects {
		if strings.TrimSpace(r.text) == "" {
			continue
		}
		if !fontIsBold(r.fontName) {
			return false
		}
	}
	return true
}

// fontIsMono returns true if the font name suggests a monospace font.
func fontIsMono(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "mono") ||
		strings.Contains(lower, "courier") ||
		strings.Contains(lower, "consola") ||
		strings.HasPrefix(lower, "cmtt") || // Computer Modern Typewriter
		strings.Contains(lower, "typewriter")
}

// headingLevel determines the markdown heading level based on font size
// relative to the body size. Returns 0 for body text.
func headingLevel(fontSize, bodySize float64, isBold bool) int {
	if bodySize <= 0 {
		return 0
	}
	ratio := fontSize / bodySize
	switch {
	case ratio >= 2.0:
		return 1
	case ratio >= 1.5:
		return 2
	case ratio >= 1.1:
		// Larger-than-body text: bold gets H3, non-bold gets H4
		if isBold {
			return 3
		}
		return 4
	default:
		return 0
	}
}

// renderMarkdownFromLines converts structured PDF lines into markdown text.
func renderMarkdownFromLines(lines []pdfTextLine, bodySize float64) string {
	var md strings.Builder
	prevWasHeading := false

	for i, line := range lines {
		rawText := strings.TrimSpace(line.text())
		if rawText == "" {
			continue
		}

		// Check if this is a superscript/footnote-sized line (skip tiny annotations)
		if line.fontSize > 0 && bodySize > 0 && line.fontSize < bodySize*0.75 {
			// Small text like footnote markers - include inline but don't make a heading
			// Only skip if it's very small and standalone
			if line.fontSize < bodySize*0.6 && len(rawText) <= 3 {
				continue
			}
		}

		isBold := fontIsBold(line.fontName)

		// Determine heading level from font size
		level := headingLevel(line.fontSize, bodySize, isBold)

		// Additional heuristic: standalone short bold lines at body size
		// are likely subheadings (e.g. "References", "Acknowledgements")
		if level == 0 && isBold && line.fontSize >= bodySize && allRectsAreBold(line.rects) {
			text := strings.TrimSpace(line.text())
			// Only treat as heading if reasonably short (not a bold paragraph)
			if len(text) < 80 {
				level = 4
			}
		}

		// Build the line text with inline formatting
		lineMarkdown := buildLineMarkdown(line.rects, bodySize)
		lineMarkdown = strings.TrimSpace(lineMarkdown)
		if lineMarkdown == "" {
			continue
		}

		if level > 0 {
			// Ensure blank line before headings
			if md.Len() > 0 {
				md.WriteString("\n")
			}
			md.WriteString(strings.Repeat("#", level))
			md.WriteString(" ")
			// Strip inline formatting from headings (heading itself implies emphasis)
			md.WriteString(stripMarkdownFormatting(lineMarkdown))
			md.WriteString("\n\n")
			prevWasHeading = true
		} else {
			// Check if there's a significant vertical gap from previous line
			// (indicating a paragraph break)
			if i > 0 && !prevWasHeading {
				prevLine := lines[i-1]
				gap := prevLine.bottom - line.top
				lineHeight := line.top - line.bottom
				if lineHeight <= 0 {
					lineHeight = bodySize
				}
				// Gap larger than ~1.5x line height suggests a paragraph break
				if gap > lineHeight*1.5 {
					md.WriteString("\n")
				}
			}

			md.WriteString(lineMarkdown)
			md.WriteString("\n")
			prevWasHeading = false
		}
	}

	return md.String()
}

// buildLineMarkdown renders a line's rects with inline markdown formatting
// (bold, italic, code) based on font properties.
func buildLineMarkdown(rects []pdfRect, bodySize float64) string {
	// Merge consecutive rects with the same formatting to avoid split markers
	type fmtRun struct {
		text   string
		bold   bool
		italic bool
		mono   bool
	}

	var runs []fmtRun
	for _, r := range rects {
		text := r.text
		if strings.TrimSpace(text) == "" {
			continue
		}

		// Skip superscript footnote markers (tiny text, typically 1-2 chars)
		if r.fontSize > 0 && bodySize > 0 && r.fontSize < bodySize*0.6 && len(strings.TrimSpace(text)) <= 3 {
			continue
		}

		run := fmtRun{
			text:   text,
			bold:   fontIsBold(r.fontName),
			italic: fontIsItalic(r.fontName),
			mono:   fontIsMono(r.fontName),
		}

		// Merge with previous run if same formatting
		if len(runs) > 0 {
			prev := &runs[len(runs)-1]
			if prev.bold == run.bold && prev.italic == run.italic && prev.mono == run.mono {
				prev.text += text
				continue
			}
		}
		runs = append(runs, run)
	}

	// Render merged runs
	var b strings.Builder
	for _, run := range runs {
		text := run.text
		if run.mono {
			b.WriteString("`")
			b.WriteString(strings.TrimSpace(text))
			b.WriteString("`")
			// Preserve trailing space if original had it
			if strings.HasSuffix(text, " ") {
				b.WriteString(" ")
			}
		} else if run.bold && run.italic {
			trimmed := strings.TrimRight(text, " ")
			b.WriteString("***")
			b.WriteString(trimmed)
			b.WriteString("***")
			if len(text) > len(trimmed) {
				b.WriteString(" ")
			}
		} else if run.bold {
			trimmed := strings.TrimRight(text, " ")
			b.WriteString("**")
			b.WriteString(trimmed)
			b.WriteString("**")
			if len(text) > len(trimmed) {
				b.WriteString(" ")
			}
		} else if run.italic {
			trimmed := strings.TrimRight(text, " ")
			b.WriteString("*")
			b.WriteString(trimmed)
			b.WriteString("*")
			if len(text) > len(trimmed) {
				b.WriteString(" ")
			}
		} else {
			b.WriteString(text)
		}
	}
	return b.String()
}

// stripMarkdownFormatting removes inline markdown markers for use in headings.
func stripMarkdownFormatting(s string) string {
	s = strings.ReplaceAll(s, "***", "")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "`", "")
	return s
}
