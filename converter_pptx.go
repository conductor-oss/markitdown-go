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
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"path"
	"sort"
	"strings"

	"github.com/nicholasgasior/markitdown-go/internal/ooxml"
)

// PptxConverter handles PPTX files.
type PptxConverter struct {
	markitdown *MarkItDown
}

// NewPptxConverter creates a new PptxConverter.
func NewPptxConverter(m *MarkItDown) *PptxConverter {
	return &PptxConverter{markitdown: m}
}

func (c *PptxConverter) Accepts(info StreamInfo) bool {
	if info.Extension == ".pptx" {
		return true
	}
	mime := strings.ToLower(info.MIMEType)
	return strings.HasPrefix(mime, "application/vnd.openxmlformats-officedocument.presentationml")
}

func (c *PptxConverter) Convert(reader io.ReadSeeker, info StreamInfo) (*DocumentConverterResult, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read PPTX: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open PPTX ZIP: %w", err)
	}

	// Get slide order from presentation.xml
	slideOrder, err := c.getSlideOrder(zr)
	if err != nil {
		return nil, fmt.Errorf("get slide order: %w", err)
	}

	var md strings.Builder

	for slideNum, slidePath := range slideOrder {
		md.WriteString(fmt.Sprintf("\n\n<!-- Slide number: %d -->\n", slideNum+1))

		slideData, err := ooxml.ReadFileFromZip(zr, slidePath)
		if err != nil {
			continue
		}

		slideContent := c.parseSlide(slideData)
		md.WriteString(slideContent)

		// Check for notes
		notesPath := c.getNotesPath(slidePath, zr)
		if notesPath != "" {
			notesData, err := ooxml.ReadFileFromZip(zr, notesPath)
			if err == nil {
				notes := c.extractNotesText(notesData)
				if strings.TrimSpace(notes) != "" {
					md.WriteString("\n\n### Notes:\n")
					md.WriteString(notes)
				}
			}
		}
	}

	return &DocumentConverterResult{
		Markdown: strings.TrimSpace(md.String()),
	}, nil
}

// getSlideOrder returns slide file paths in presentation order.
func (c *PptxConverter) getSlideOrder(zr *zip.Reader) ([]string, error) {
	presData, err := ooxml.ReadFileFromZip(zr, "ppt/presentation.xml")
	if err != nil {
		return nil, err
	}

	rels, _ := ooxml.ParseRelationshipsFromReader(zr, "ppt/_rels/presentation.xml.rels")

	decoder := xml.NewDecoder(bytes.NewReader(presData))
	var slideRIDs []string

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		if se, ok := tok.(xml.StartElement); ok {
			if se.Name.Local == "sldId" {
				for _, attr := range se.Attr {
					if attr.Name.Local == "id" && strings.Contains(attr.Name.Space, "relationships") {
						slideRIDs = append(slideRIDs, attr.Value)
					}
				}
			}
		}
	}

	var slidePaths []string
	for _, rid := range slideRIDs {
		if rel, ok := rels[rid]; ok {
			slidePaths = append(slidePaths, ooxml.ResolveTarget("ppt/presentation.xml", rel.Target))
		}
	}

	if len(slidePaths) == 0 {
		for _, f := range zr.File {
			if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
				slidePaths = append(slidePaths, f.Name)
			}
		}
		sort.Strings(slidePaths)
	}

	return slidePaths, nil
}

type pptxShape struct {
	top     int64
	left    int64
	text    string
	isTitle bool
	isTable bool
	table   [][]string
	isPic   bool
	altText string
}

// parseSlide parses a slide XML and extracts shapes, then formats as markdown.
func (c *PptxConverter) parseSlide(slideData []byte) string {
	shapes := c.extractShapes(slideData)

	sort.SliceStable(shapes, func(i, j int) bool {
		if shapes[i].top != shapes[j].top {
			return shapes[i].top < shapes[j].top
		}
		return shapes[i].left < shapes[j].left
	})

	var md strings.Builder
	for _, shape := range shapes {
		if shape.isPic && shape.altText != "" {
			// Picture with alt text
			md.WriteString(fmt.Sprintf("\n![%s](image)\n", sanitizeAltText(shape.altText)))
		} else if shape.isTable && len(shape.table) > 0 {
			md.WriteString(c.tableToMarkdown(shape.table))
		} else if shape.isTitle {
			text := strings.TrimSpace(shape.text)
			if text != "" {
				md.WriteString("# " + text + "\n")
			}
		} else if shape.text != "" {
			md.WriteString(shape.text + "\n")
		}
	}

	return md.String()
}

// sanitizeAltText cleans alt text for markdown image syntax.
func sanitizeAltText(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "[", " ")
	s = strings.ReplaceAll(s, "]", " ")
	// Collapse multiple spaces
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// extractShapes extracts all shapes from slide XML using a recursive approach.
func (c *PptxConverter) extractShapes(slideData []byte) []pptxShape {
	// Parse the entire XML into a generic tree for easier traversal
	var root xmlNode
	if err := xml.Unmarshal(slideData, &root); err != nil {
		return nil
	}

	var shapes []pptxShape
	c.walkTree(&root, &shapes)
	return shapes
}

// xmlNode is a generic XML tree node.
type xmlNode struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Children []xmlNode  `xml:",any"`
	Content  string     `xml:",chardata"`
}

func (n *xmlNode) getAttr(name string) string {
	for _, a := range n.Attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func (n *xmlNode) findChild(local string) *xmlNode {
	for i := range n.Children {
		if n.Children[i].XMLName.Local == local {
			return &n.Children[i]
		}
	}
	return nil
}

func (n *xmlNode) findAll(local string) []*xmlNode {
	var result []*xmlNode
	for i := range n.Children {
		if n.Children[i].XMLName.Local == local {
			result = append(result, &n.Children[i])
		}
	}
	return result
}

// findDeep finds first descendant with given local name.
func (n *xmlNode) findDeep(local string) *xmlNode {
	for i := range n.Children {
		if n.Children[i].XMLName.Local == local {
			return &n.Children[i]
		}
		found := n.Children[i].findDeep(local)
		if found != nil {
			return found
		}
	}
	return nil
}

// findAllDeep finds all descendants with given local name.
func (n *xmlNode) findAllDeep(local string) []*xmlNode {
	var result []*xmlNode
	for i := range n.Children {
		if n.Children[i].XMLName.Local == local {
			result = append(result, &n.Children[i])
		}
		result = append(result, n.Children[i].findAllDeep(local)...)
	}
	return result
}

// allText extracts all text content recursively.
func (n *xmlNode) allText() string {
	if n.Content != "" {
		return n.Content
	}
	var parts []string
	for i := range n.Children {
		t := n.Children[i].allText()
		if t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "")
}

// walkTree walks the XML tree and extracts shapes.
func (c *PptxConverter) walkTree(node *xmlNode, shapes *[]pptxShape) {
	local := node.XMLName.Local

	switch local {
	case "sp":
		shape := c.extractSP(node)
		if shape != nil {
			*shapes = append(*shapes, *shape)
		}
	case "pic":
		shape := c.extractPic(node)
		if shape != nil {
			*shapes = append(*shapes, *shape)
		}
	case "graphicFrame":
		shape := c.extractGraphicFrame(node)
		if shape != nil {
			*shapes = append(*shapes, *shape)
		}
	case "grpSp":
		// Group shape - recurse into children
		for i := range node.Children {
			c.walkTree(&node.Children[i], shapes)
		}
	default:
		for i := range node.Children {
			c.walkTree(&node.Children[i], shapes)
		}
	}
}

// extractSP extracts a shape element.
func (c *PptxConverter) extractSP(node *xmlNode) *pptxShape {
	shape := &pptxShape{
		top:  math.MaxInt64,
		left: math.MaxInt64,
	}

	// Check for title placeholder
	nvSpPr := node.findChild("nvSpPr")
	if nvSpPr != nil {
		nvPr := nvSpPr.findChild("nvPr")
		if nvPr != nil {
			ph := nvPr.findChild("ph")
			if ph != nil {
				phType := ph.getAttr("type")
				if phType == "title" || phType == "ctrTitle" {
					shape.isTitle = true
				}
			}
		}
	}

	// Get position
	c.extractPosition(node, shape)

	// Extract text from txBody
	txBody := node.findChild("txBody")
	if txBody != nil {
		shape.text = c.extractTextFromTxBody(txBody)
	}

	if strings.TrimSpace(shape.text) == "" {
		return nil
	}

	return shape
}

// extractPic extracts a picture element.
func (c *PptxConverter) extractPic(node *xmlNode) *pptxShape {
	shape := &pptxShape{
		top:   math.MaxInt64,
		left:  math.MaxInt64,
		isPic: true,
	}

	// Extract alt text from nvPicPr/cNvPr
	nvPicPr := node.findChild("nvPicPr")
	if nvPicPr != nil {
		cNvPr := nvPicPr.findChild("cNvPr")
		if cNvPr != nil {
			shape.altText = cNvPr.getAttr("descr")
		}
	}

	c.extractPosition(node, shape)

	if shape.altText == "" {
		return nil
	}

	return shape
}

// extractGraphicFrame extracts a graphic frame (tables, charts).
func (c *PptxConverter) extractGraphicFrame(node *xmlNode) *pptxShape {
	shape := &pptxShape{
		top:  math.MaxInt64,
		left: math.MaxInt64,
	}

	c.extractPosition(node, shape)

	// Look for table
	tbl := node.findDeep("tbl")
	if tbl != nil {
		shape.isTable = true
		shape.table = c.extractTable(tbl)
		if len(shape.table) > 0 {
			return shape
		}
	}

	return nil
}

// extractPosition extracts position from spPr/xfrm/off.
func (c *PptxConverter) extractPosition(node *xmlNode, shape *pptxShape) {
	spPr := node.findChild("spPr")
	if spPr == nil {
		return
	}
	xfrm := spPr.findChild("xfrm")
	if xfrm == nil {
		return
	}
	off := xfrm.findChild("off")
	if off == nil {
		return
	}
	if x := off.getAttr("x"); x != "" {
		var v int64
		fmt.Sscanf(x, "%d", &v)
		shape.left = v
	}
	if y := off.getAttr("y"); y != "" {
		var v int64
		fmt.Sscanf(y, "%d", &v)
		shape.top = v
	}
}

// extractTextFromTxBody extracts text from a txBody element.
func (c *PptxConverter) extractTextFromTxBody(txBody *xmlNode) string {
	var parts []string
	for _, p := range txBody.findAll("p") {
		var lineText []string
		for _, r := range p.findAllDeep("t") {
			t := r.allText()
			if t != "" {
				lineText = append(lineText, t)
			}
		}
		if len(lineText) > 0 {
			parts = append(parts, strings.Join(lineText, ""))
		}
	}
	return strings.Join(parts, "\n")
}

// extractTable extracts a table from a tbl element.
func (c *PptxConverter) extractTable(tbl *xmlNode) [][]string {
	var rows [][]string
	for _, tr := range tbl.findAll("tr") {
		var row []string
		for _, tc := range tr.findAll("tc") {
			// Extract text from cell's txBody
			txBody := tc.findChild("txBody")
			if txBody != nil {
				cellText := c.extractTextFromTxBody(txBody)
				row = append(row, strings.TrimSpace(cellText))
			} else {
				row = append(row, "")
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// tableToMarkdown converts a 2D table to markdown.
func (c *PptxConverter) tableToMarkdown(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}

	var htmlBuf strings.Builder
	htmlBuf.WriteString("<html><body><table>")
	for i, row := range rows {
		htmlBuf.WriteString("<tr>")
		tag := "td"
		if i == 0 {
			tag = "th"
		}
		for _, cell := range row {
			htmlBuf.WriteString("<" + tag + ">" + escapeHTMLText(cell) + "</" + tag + ">")
		}
		htmlBuf.WriteString("</tr>")
	}
	htmlBuf.WriteString("</table></body></html>")

	htmlConv := NewHTMLConverter(c.markitdown)
	result, err := htmlConv.ConvertString(htmlBuf.String())
	if err != nil {
		return renderMarkdownTable(rows)
	}
	return strings.TrimSpace(result.Markdown) + "\n"
}

// getNotesPath returns the notes slide path for a given slide.
func (c *PptxConverter) getNotesPath(slidePath string, zr *zip.Reader) string {
	relsPath := ooxml.RelsPathFor(slidePath)
	rels, err := ooxml.ParseRelationshipsFromReader(zr, relsPath)
	if err != nil {
		return ""
	}

	for _, rel := range rels {
		if strings.Contains(rel.Type, "notesSlide") {
			return ooxml.ResolveTarget(slidePath, rel.Target)
		}
	}
	return ""
}

// extractNotesText extracts text content from a notes slide.
func (c *PptxConverter) extractNotesText(data []byte) string {
	var root xmlNode
	if err := xml.Unmarshal(data, &root); err != nil {
		return ""
	}

	// Find all txBody elements in the notes
	var parts []string
	for _, txBody := range root.findAllDeep("txBody") {
		text := c.extractTextFromTxBody(txBody)
		text = strings.TrimSpace(text)
		if text != "" {
			parts = append(parts, text)
		}
	}

	return strings.Join(parts, "\n")
}

// Helper to resolve path
func init() {
	_ = path.Join // ensure path is imported
}
