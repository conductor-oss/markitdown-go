package markitdown

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/nicholasgasior/markitdown-go/internal/docxmath"
	"github.com/nicholasgasior/markitdown-go/internal/ooxml"
)

// DocxConverter handles DOCX files.
type DocxConverter struct {
	markitdown *MarkItDown
}

// NewDocxConverter creates a new DocxConverter.
func NewDocxConverter(m *MarkItDown) *DocxConverter {
	return &DocxConverter{markitdown: m}
}

func (c *DocxConverter) Accepts(info StreamInfo) bool {
	if info.Extension == ".docx" {
		return true
	}
	mime := strings.ToLower(info.MIMEType)
	return strings.HasPrefix(mime, "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
}

func (c *DocxConverter) Convert(reader io.ReadSeeker, info StreamInfo) (*DocumentConverterResult, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read DOCX: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open DOCX ZIP: %w", err)
	}

	// Parse relationships for hyperlinks
	rels, _ := ooxml.ParseRelationshipsFromReader(zr, "word/_rels/document.xml.rels")

	// Parse numbering definitions for lists
	numbering := c.parseNumbering(zr)

	// Parse comments
	comments := c.parseComments(zr)

	// Parse styles for heading detection
	styles := c.parseStyles(zr)

	// Read and parse document.xml
	docData, err := ooxml.ReadFileFromZip(zr, "word/document.xml")
	if err != nil {
		return nil, fmt.Errorf("read document.xml: %w", err)
	}

	// Pre-process: convert OMML math to LaTeX
	docData = c.preProcessMath(docData)

	// Parse the document body to HTML
	htmlStr := c.documentToHTML(docData, rels, numbering, comments, styles, zr)

	// Convert HTML to markdown via the HTML converter
	htmlConv := NewHTMLConverter(c.markitdown)
	result, err := htmlConv.ConvertString(htmlStr)
	if err != nil {
		return nil, fmt.Errorf("convert DOCX HTML to markdown: %w", err)
	}

	return result, nil
}

// styleInfo holds style information for a document style.
type styleInfo struct {
	name    string
	styleID string
}

// numberingLevel holds numbering level info.
type numberingLevel struct {
	numFmt string
	start  int
}

// numberingDef holds a numbering definition.
type numberingDef struct {
	abstractNumID string
	levels        map[int]numberingLevel
}

func (c *DocxConverter) parseStyles(zr *zip.Reader) map[string]styleInfo {
	styles := make(map[string]styleInfo)
	data, err := ooxml.ReadFileFromZip(zr, "word/styles.xml")
	if err != nil {
		return styles
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	var currentStyleID string
	var inStyle bool
	var inName bool

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			local := t.Name.Local
			if local == "style" {
				inStyle = true
				for _, attr := range t.Attr {
					if attr.Name.Local == "styleId" {
						currentStyleID = attr.Value
					}
				}
			} else if inStyle && local == "name" {
				inName = true
				for _, attr := range t.Attr {
					if attr.Name.Local == "val" {
						styles[currentStyleID] = styleInfo{
							name:    attr.Value,
							styleID: currentStyleID,
						}
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "style" {
				inStyle = false
				currentStyleID = ""
			}
			if t.Name.Local == "name" {
				inName = false
			}
		}
		_ = inName
	}
	return styles
}

func (c *DocxConverter) parseNumbering(zr *zip.Reader) map[string]numberingDef {
	numbering := make(map[string]numberingDef)
	data, err := ooxml.ReadFileFromZip(zr, "word/numbering.xml")
	if err != nil {
		return numbering
	}

	// Simple XML parsing for numbering definitions
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var currentNumID string
	var inNum bool

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "num" {
				inNum = true
				for _, attr := range t.Attr {
					if attr.Name.Local == "numId" {
						currentNumID = attr.Value
					}
				}
			} else if inNum && t.Name.Local == "abstractNumId" {
				for _, attr := range t.Attr {
					if attr.Name.Local == "val" {
						numbering[currentNumID] = numberingDef{abstractNumID: attr.Value}
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "num" {
				inNum = false
			}
		}
	}
	return numbering
}

type docxComment struct {
	id     string
	author string
	text   string
}

func (c *DocxConverter) parseComments(zr *zip.Reader) map[string]docxComment {
	comments := make(map[string]docxComment)
	data, err := ooxml.ReadFileFromZip(zr, "word/comments.xml")
	if err != nil {
		return comments
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	var current docxComment
	var inComment bool
	var depth int
	var textBuf strings.Builder

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "comment" {
				inComment = true
				depth = 0
				textBuf.Reset()
				for _, attr := range t.Attr {
					switch attr.Name.Local {
					case "id":
						current.id = attr.Value
					case "author":
						current.author = attr.Value
					}
				}
			}
			if inComment {
				depth++
			}
		case xml.CharData:
			if inComment {
				textBuf.Write(t)
			}
		case xml.EndElement:
			if t.Name.Local == "comment" && inComment {
				current.text = strings.TrimSpace(textBuf.String())
				comments[current.id] = current
				inComment = false
				current = docxComment{}
			}
			if inComment {
				depth--
			}
		}
	}
	return comments
}

// preProcessMath replaces OMML math elements with LaTeX in the document XML.
func (c *DocxConverter) preProcessMath(docData []byte) []byte {
	content := string(docData)

	// Process oMathPara (block math) first
	content = c.replaceOMMLBlocks(content, "m:oMathPara", true)

	// Process standalone oMath (inline math)
	content = c.replaceOMMLBlocks(content, "m:oMath", false)

	return []byte(content)
}

func (c *DocxConverter) replaceOMMLBlocks(content string, tagName string, block bool) string {
	openTag := "<" + tagName
	closeTag := "</" + tagName + ">"

	for {
		start := strings.Index(content, openTag)
		if start == -1 {
			break
		}
		end := strings.Index(content[start:], closeTag)
		if end == -1 {
			break
		}
		end = start + end + len(closeTag)

		xmlBlock := content[start:end]

		// Try to convert the OMML to LaTeX
		latex := c.convertOMMLBlock(xmlBlock)
		if latex != "" {
			var replacement string
			if block {
				replacement = "<w:p><w:r><w:t>$$" + latex + "$$</w:t></w:r></w:p>"
			} else {
				replacement = "<w:r><w:t>$" + latex + "$</w:t></w:r>"
			}
			content = content[:start] + replacement + content[end:]
		} else {
			// Skip this block if conversion failed
			break
		}
	}
	return content
}

func (c *DocxConverter) convertOMMLBlock(xmlBlock string) string {
	// Wrap the block in a document root with namespaces for proper parsing
	wrapped := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">` +
		xmlBlock + `</w:document>`

	var root docxmath.OMMLElement
	if err := xml.Unmarshal([]byte(wrapped), &root); err != nil {
		return ""
	}

	// Find oMath elements
	var latexParts []string
	findOMath(&root, &latexParts)

	if len(latexParts) == 0 {
		return ""
	}
	return strings.Join(latexParts, " ")
}

func findOMath(elm *docxmath.OMMLElement, results *[]string) {
	if elm.XMLName.Local == "oMath" {
		latex := docxmath.ConvertOMML(elm)
		if latex != "" {
			*results = append(*results, latex)
		}
		return
	}
	for i := range elm.Children {
		findOMath(&elm.Children[i], results)
	}
}

// documentToHTML converts the document.xml content to HTML.
func (c *DocxConverter) documentToHTML(docData []byte, rels map[string]ooxml.Relationship, numbering map[string]numberingDef, comments map[string]docxComment, styles map[string]styleInfo, zr *zip.Reader) string {
	var html strings.Builder
	html.WriteString("<html><body>")

	decoder := xml.NewDecoder(bytes.NewReader(docData))

	type state struct {
		inParagraph bool
		inRun       bool
		inText      bool
		inTable     bool
		inTableRow  bool
		inTableCell bool
		bold        bool
		italic      bool
		underline   bool
		strike      bool
		styleID     string
		hyperRef    string
		inHyper     bool
		listNumID   string
		listLevel   int
		inList      bool
	}

	var s state
	var textBuf strings.Builder
	var paragraphs []string
	var currentPara strings.Builder
	var tableRows [][]string
	var currentRow []string
	var cellContent strings.Builder
	var commentRefs []string

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			local := t.Name.Local
			switch local {
			case "p":
				s.inParagraph = true
				currentPara.Reset()
				s.bold = false
				s.italic = false
				s.underline = false
				s.strike = false
				s.styleID = ""
				s.listNumID = ""
				s.listLevel = 0
				s.inList = false
				commentRefs = nil

			case "pPr":
				// We'll parse style and list info from nested elements

			case "pStyle":
				for _, attr := range t.Attr {
					if attr.Name.Local == "val" {
						s.styleID = attr.Value
					}
				}

			case "numPr":
				s.inList = true

			case "numId":
				for _, attr := range t.Attr {
					if attr.Name.Local == "val" {
						s.listNumID = attr.Value
					}
				}

			case "ilvl":
				for _, attr := range t.Attr {
					if attr.Name.Local == "val" {
						level := 0
						fmt.Sscanf(attr.Value, "%d", &level)
						s.listLevel = level
					}
				}

			case "r":
				s.inRun = true
				s.bold = false
				s.italic = false
				s.underline = false
				s.strike = false

			case "rPr":
				// Run properties - will be parsed from children

			case "b":
				if s.inRun {
					s.bold = true
					// Check for val="0" which means NOT bold
					for _, attr := range t.Attr {
						if attr.Name.Local == "val" && attr.Value == "0" {
							s.bold = false
						}
					}
				}

			case "i":
				if s.inRun {
					s.italic = true
					for _, attr := range t.Attr {
						if attr.Name.Local == "val" && attr.Value == "0" {
							s.italic = false
						}
					}
				}

			case "u":
				if s.inRun {
					s.underline = true
				}

			case "strike":
				if s.inRun {
					s.strike = true
				}

			case "t":
				s.inText = true
				textBuf.Reset()

			case "tab":
				if s.inRun {
					currentPara.WriteString("\t")
				}

			case "br":
				if s.inRun {
					currentPara.WriteString("<br/>")
				}

			case "hyperlink":
				s.inHyper = true
				for _, attr := range t.Attr {
					if attr.Name.Space == "http://schemas.openxmlformats.org/officeDocument/2006/relationships" && attr.Name.Local == "id" {
						if rel, ok := rels[attr.Value]; ok {
							s.hyperRef = rel.Target
						}
					}
				}

			case "tbl":
				s.inTable = true
				tableRows = nil

			case "tr":
				s.inTableRow = true
				currentRow = nil

			case "tc":
				s.inTableCell = true
				cellContent.Reset()

			case "commentReference":
				for _, attr := range t.Attr {
					if attr.Name.Local == "id" {
						commentRefs = append(commentRefs, attr.Value)
					}
				}

			case "drawing", "pict":
				// Try to extract images
				imgData := c.extractImage(t, decoder, zr)
				if imgData != "" {
					if s.inTableCell {
						cellContent.WriteString(imgData)
					} else {
						currentPara.WriteString(imgData)
					}
				}
			}

		case xml.CharData:
			if s.inText {
				textBuf.Write(t)
			}

		case xml.EndElement:
			local := t.Name.Local
			switch local {
			case "t":
				if s.inText {
					text := textBuf.String()
					text = escapeHTMLText(text)

					// Apply formatting
					if s.bold {
						text = "<b>" + text + "</b>"
					}
					if s.italic {
						text = "<i>" + text + "</i>"
					}
					if s.strike {
						text = "<s>" + text + "</s>"
					}

					if s.inHyper && s.hyperRef != "" {
						text = `<a href="` + escapeHTMLAttr(s.hyperRef) + `">` + text + "</a>"
					}

					if s.inTableCell {
						cellContent.WriteString(text)
					} else {
						currentPara.WriteString(text)
					}
					s.inText = false
				}

			case "r":
				s.inRun = false
				s.bold = false
				s.italic = false

			case "hyperlink":
				s.inHyper = false
				s.hyperRef = ""

			case "p":
				paraText := currentPara.String()

				// Add comment annotations
				for _, commentID := range commentRefs {
					if comment, ok := comments[commentID]; ok {
						paraText += fmt.Sprintf(" [comment by %s: %s]", comment.author, comment.text)
					}
				}

				if s.inTableCell {
					cellContent.WriteString(paraText)
				} else {
					// Determine heading level from style
					headingLevel := c.getHeadingLevel(s.styleID, styles)

					if headingLevel > 0 {
						tag := fmt.Sprintf("h%d", headingLevel)
						paraText = "<" + tag + ">" + paraText + "</" + tag + ">"
					} else if s.inList && s.listNumID != "0" {
						// List item
						paraText = "<li>" + paraText + "</li>"
					} else if paraText != "" {
						paraText = "<p>" + paraText + "</p>"
					}

					if paraText != "" {
						paragraphs = append(paragraphs, paraText)
					}
				}
				s.inParagraph = false
				s.styleID = ""

			case "tc":
				currentRow = append(currentRow, cellContent.String())
				s.inTableCell = false

			case "tr":
				tableRows = append(tableRows, currentRow)
				s.inTableRow = false

			case "tbl":
				// Render table as HTML
				if len(tableRows) > 0 {
					var tableBuf strings.Builder
					tableBuf.WriteString("<table>")
					for i, row := range tableRows {
						tableBuf.WriteString("<tr>")
						tag := "td"
						if i == 0 {
							tag = "th"
						}
						for _, cell := range row {
							tableBuf.WriteString("<" + tag + ">" + cell + "</" + tag + ">")
						}
						tableBuf.WriteString("</tr>")
					}
					tableBuf.WriteString("</table>")
					paragraphs = append(paragraphs, tableBuf.String())
				}
				s.inTable = false
			}
		}
	}

	for _, p := range paragraphs {
		html.WriteString(p)
		html.WriteString("\n")
	}

	html.WriteString("</body></html>")
	return html.String()
}

// getHeadingLevel returns the heading level (1-6) for a style, or 0 if not a heading.
func (c *DocxConverter) getHeadingLevel(styleID string, styles map[string]styleInfo) int {
	if styleID == "" {
		return 0
	}

	// Check common heading style patterns
	lower := strings.ToLower(styleID)

	// Direct heading patterns: "Heading1", "heading1", "Heading 1"
	for i := 1; i <= 6; i++ {
		patterns := []string{
			fmt.Sprintf("heading%d", i),
			fmt.Sprintf("heading %d", i),
		}
		for _, p := range patterns {
			if strings.ToLower(styleID) == p || lower == p {
				return i
			}
		}
	}

	// Check style name from styles.xml
	if si, ok := styles[styleID]; ok {
		nameLower := strings.ToLower(si.name)
		for i := 1; i <= 6; i++ {
			if nameLower == fmt.Sprintf("heading %d", i) {
				return i
			}
		}
	}

	return 0
}

// extractImage attempts to extract an image from a drawing or pict element.
func (c *DocxConverter) extractImage(startElem xml.StartElement, decoder *xml.Decoder, zr *zip.Reader) string {
	// We need to consume the entire drawing/pict element
	// and look for embedded image references
	depth := 1
	var embedID string
	var altText string

	for depth > 0 {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			// Look for blip element (embedded image reference)
			if t.Name.Local == "blip" {
				for _, attr := range t.Attr {
					if attr.Name.Local == "embed" {
						embedID = attr.Value
					}
				}
			}
			// Look for docPr element (description/alt text)
			if t.Name.Local == "docPr" {
				for _, attr := range t.Attr {
					if attr.Name.Local == "descr" {
						altText = attr.Value
					}
				}
			}
		case xml.EndElement:
			depth--
		}
	}

	if embedID == "" {
		return ""
	}

	// Resolve the relationship to find the image file
	rels, _ := ooxml.ParseRelationshipsFromReader(zr, "word/_rels/document.xml.rels")
	rel, ok := rels[embedID]
	if !ok {
		return ""
	}

	// Read the image from the ZIP
	imgPath := "word/" + rel.Target
	imgData, err := ooxml.ReadFileFromZip(zr, imgPath)
	if err != nil {
		// Try without word/ prefix
		imgData, err = ooxml.ReadFileFromZip(zr, rel.Target)
		if err != nil {
			return ""
		}
	}

	// Determine content type from extension
	ext := strings.ToLower(path.Ext(rel.Target))
	contentType := "image/png"
	switch ext {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".gif":
		contentType = "image/gif"
	case ".bmp":
		contentType = "image/bmp"
	case ".svg":
		contentType = "image/svg+xml"
	}

	// Encode as data URI
	b64 := base64.StdEncoding.EncodeToString(imgData)
	src := fmt.Sprintf("data:%s;base64,%s", contentType, b64)

	if altText == "" {
		altText = path.Base(rel.Target)
	}

	return fmt.Sprintf(`<img src="%s" alt="%s"/>`, src, escapeHTMLAttr(altText))
}

func escapeHTMLText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func escapeHTMLAttr(s string) string {
	s = escapeHTMLText(s)
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
