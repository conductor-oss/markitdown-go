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

package docxmath

import (
	"encoding/xml"
	"fmt"
	"strings"
)

const OMML_NS = "http://schemas.openxmlformats.org/officeDocument/2006/math"

// EscapeLatex escapes LaTeX special characters in a string.
func EscapeLatex(s string) string {
	s = strings.ReplaceAll(s, "\\\\", "\\")
	var result strings.Builder
	var last rune
	for _, c := range s {
		if latexSpecialChars[c] && last != '\\' {
			result.WriteRune('\\')
		}
		result.WriteRune(c)
		last = c
	}
	return result.String()
}

// getVal looks up a key in a store map, returning a default if not found.
func getVal(key string, defaultVal string, store map[string]string) string {
	if key != "" {
		if store != nil {
			if v, ok := store[key]; ok {
				return v
			}
			return key
		}
		return key
	}
	return defaultVal
}

// formatTemplate replaces named placeholders in a template string.
func formatTemplate(tmpl string, replacements map[string]string) string {
	result := tmpl
	for k, v := range replacements {
		result = strings.ReplaceAll(result, "{"+k+"}", v)
	}
	return result
}

// formatPositional replaces {0} placeholder in a format string.
func formatPositional(format string, arg string) string {
	return strings.ReplaceAll(format, "{0}", arg)
}

// OMMLElement represents a parsed OMML XML element.
type OMMLElement struct {
	XMLName  xml.Name
	Attrs    []xml.Attr     `xml:",any,attr"`
	Children []OMMLElement  `xml:",any"`
	Content  string         `xml:",chardata"`
}

// localName returns the local part of the element's name.
func (e *OMMLElement) localName() string {
	return e.XMLName.Local
}

// findChild finds the first child with the given local name.
func (e *OMMLElement) findChild(name string) *OMMLElement {
	for i := range e.Children {
		if e.Children[i].localName() == name {
			return &e.Children[i]
		}
	}
	return nil
}

// findAllChildren finds all children with the given local name.
func (e *OMMLElement) findAllChildren(name string) []*OMMLElement {
	var result []*OMMLElement
	for i := range e.Children {
		if e.Children[i].localName() == name {
			result = append(result, &e.Children[i])
		}
	}
	return result
}

// getAttrVal gets the value of an attribute by local name.
func (e *OMMLElement) getAttrVal() string {
	for _, attr := range e.Attrs {
		if attr.Name.Local == "val" {
			return attr.Value
		}
	}
	return ""
}

// findText finds text content in a specific child path.
func (e *OMMLElement) findText(name string) string {
	child := e.findChild(name)
	if child == nil {
		return ""
	}
	return child.Content
}

// prResult holds properties parsed from a Pr element.
type prResult struct {
	text   string
	chr    string
	pos    string
	begChr string
	endChr string
	typ    string // "type" in Python
	brk    string
}

// parsePr processes a properties element (*Pr tags).
func parsePr(elm *OMMLElement) *prResult {
	pr := &prResult{}
	var textParts []string

	for i := range elm.Children {
		child := &elm.Children[i]
		tag := child.localName()

		switch tag {
		case "brk":
			pr.brk = Brk
			textParts = append(textParts, Brk)
		case "chr":
			pr.chr = child.getAttrVal()
		case "pos":
			pr.pos = child.getAttrVal()
		case "begChr":
			pr.begChr = child.getAttrVal()
		case "endChr":
			pr.endChr = child.getAttrVal()
		case "type":
			pr.typ = child.getAttrVal()
		}
	}
	pr.text = strings.Join(textParts, "")
	return pr
}

// ConvertOMML converts an oMath XML element to LaTeX.
func ConvertOMML(elm *OMMLElement) string {
	return processChildren(elm, nil)
}

// ConvertOMMLString parses an OMML XML string and converts all oMath elements to LaTeX.
func ConvertOMMLString(xmlStr string) ([]string, error) {
	// Wrap in a root element for parsing
	wrapped := "<root xmlns:m=\"" + OMML_NS + "\">" + xmlStr + "</root>"
	var root OMMLElement
	if err := xml.Unmarshal([]byte(wrapped), &root); err != nil {
		return nil, fmt.Errorf("parse OMML: %w", err)
	}

	var results []string
	for i := range root.Children {
		child := &root.Children[i]
		if child.localName() == "oMath" {
			results = append(results, ConvertOMML(child))
		}
	}
	return results, nil
}

// directTags are tags whose children are processed directly.
var directTags = map[string]bool{
	"box": true, "sSub": true, "sSup": true, "sSubSup": true,
	"num": true, "den": true, "deg": true, "e": true,
}

// processChildren processes child elements and returns concatenated LaTeX.
func processChildren(elm *OMMLElement, include map[string]bool) string {
	var parts []string
	for i := range elm.Children {
		child := &elm.Children[i]
		tag := child.localName()
		if include != nil && !include[tag] {
			continue
		}
		result := processElement(child)
		if result != "" {
			parts = append(parts, result)
		}
	}
	return strings.Join(parts, "")
}

// processChildrenDict processes child elements and returns a map of tag->latex.
func processChildrenDict(elm *OMMLElement, include map[string]bool) map[string]string {
	result := make(map[string]string)
	for i := range elm.Children {
		child := &elm.Children[i]
		tag := child.localName()
		if include != nil && !include[tag] {
			continue
		}
		text := processElement(child)
		if text != "" {
			result[tag] = text
		}
	}
	return result
}

// processChildrenList processes child elements and returns tag/text pairs.
func processChildrenList(elm *OMMLElement, include map[string]bool) []tagText {
	var result []tagText
	for i := range elm.Children {
		child := &elm.Children[i]
		tag := child.localName()
		if include != nil && !include[tag] {
			continue
		}
		text := processElement(child)
		if text != "" {
			result = append(result, tagText{tag: tag, text: text})
		}
	}
	return result
}

type tagText struct {
	tag  string
	text string
}

// processElement dispatches processing based on element tag.
func processElement(elm *OMMLElement) string {
	tag := elm.localName()

	// Check method dispatch
	switch tag {
	case "acc":
		return doAcc(elm)
	case "r":
		return doR(elm)
	case "bar":
		return doBar(elm)
	case "sub":
		return doSub(elm)
	case "sup":
		return doSup(elm)
	case "f":
		return doF(elm)
	case "func":
		return doFunc(elm)
	case "fName":
		return doFName(elm)
	case "groupChr":
		return doGroupChr(elm)
	case "d":
		return doD(elm)
	case "rad":
		return doRad(elm)
	case "eqArr":
		return doEqArr(elm)
	case "limLow":
		return doLimLow(elm)
	case "limUpp":
		return doLimUpp(elm)
	case "lim":
		return doLim(elm)
	case "m":
		return doM(elm)
	case "mr":
		return doMr(elm)
	case "nary":
		return doNary(elm)
	}

	// Handle unknown tags
	if directTags[tag] {
		return processChildren(elm, nil)
	}
	if strings.HasSuffix(tag, "Pr") {
		pr := parsePr(elm)
		return pr.text
	}
	return ""
}

// doAcc handles accent elements.
func doAcc(elm *OMMLElement) string {
	cDict := make(map[string]string)
	var accPr *prResult
	for i := range elm.Children {
		child := &elm.Children[i]
		tag := child.localName()
		if tag == "accPr" {
			accPr = parsePr(child)
			cDict[tag] = accPr.text
		} else {
			cDict[tag] = processElement(child)
		}
	}
	latexS := getVal(accPr.chr, CHR_DEFAULT["ACC_VAL"], CHR)
	return formatPositional(latexS, cDict["e"])
}

// doBar handles bar elements.
func doBar(elm *OMMLElement) string {
	cDict := make(map[string]string)
	var barPr *prResult
	for i := range elm.Children {
		child := &elm.Children[i]
		tag := child.localName()
		if tag == "barPr" {
			barPr = parsePr(child)
			cDict[tag] = barPr.text
		} else {
			cDict[tag] = processElement(child)
		}
	}
	latexS := getVal(barPr.pos, POS_DEFAULT["BAR_VAL"], POS)
	return barPr.text + formatPositional(latexS, cDict["e"])
}

// doD handles delimiter elements.
func doD(elm *OMMLElement) string {
	cDict := make(map[string]string)
	var dPr *prResult
	for i := range elm.Children {
		child := &elm.Children[i]
		tag := child.localName()
		if tag == "dPr" {
			dPr = parsePr(child)
			cDict[tag] = dPr.text
		} else {
			cDict[tag] = processElement(child)
		}
	}
	null := D_DEFAULT["null"]
	sVal := getVal(dPr.begChr, D_DEFAULT["left"], T)
	eVal := getVal(dPr.endChr, D_DEFAULT["right"], T)

	left := null
	if sVal != "" {
		left = EscapeLatex(sVal)
	}
	right := null
	if eVal != "" {
		right = EscapeLatex(eVal)
	}

	return dPr.text + formatTemplate(D_TEMPLATE, map[string]string{
		"left":  left,
		"text":  cDict["e"],
		"right": right,
	})
}

// doSub handles subscript elements.
func doSub(elm *OMMLElement) string {
	text := processChildren(elm, nil)
	return formatPositional(SUB, text)
}

// doSup handles superscript elements.
func doSup(elm *OMMLElement) string {
	text := processChildren(elm, nil)
	return formatPositional(SUP, text)
}

// doF handles fraction elements.
func doF(elm *OMMLElement) string {
	cDict := make(map[string]string)
	var fPr *prResult
	for i := range elm.Children {
		child := &elm.Children[i]
		tag := child.localName()
		if tag == "fPr" {
			fPr = parsePr(child)
			cDict[tag] = fPr.text
		} else {
			cDict[tag] = processElement(child)
		}
	}
	latexS := getVal(fPr.typ, F_DEFAULT, F)
	return fPr.text + formatTemplate(latexS, map[string]string{
		"num": cDict["num"],
		"den": cDict["den"],
	})
}

// doFunc handles function-apply elements.
func doFunc(elm *OMMLElement) string {
	cDict := processChildrenDict(elm, nil)
	funcName := cDict["fName"]
	return strings.ReplaceAll(funcName, FuncPlace, cDict["e"])
}

// doFName handles function name elements.
func doFName(elm *OMMLElement) string {
	var parts []string
	for _, tt := range processChildrenList(elm, nil) {
		if tt.tag == "r" {
			if f, ok := FUNC[tt.text]; ok {
				parts = append(parts, f)
			} else {
				parts = append(parts, tt.text)
			}
		} else {
			parts = append(parts, tt.text)
		}
	}
	t := strings.Join(parts, "")
	if !strings.Contains(t, FuncPlace) {
		t += FuncPlace
	}
	return t
}

// doGroupChr handles group-character elements.
func doGroupChr(elm *OMMLElement) string {
	cDict := make(map[string]string)
	var groupPr *prResult
	for i := range elm.Children {
		child := &elm.Children[i]
		tag := child.localName()
		if tag == "groupChrPr" {
			groupPr = parsePr(child)
			cDict[tag] = groupPr.text
		} else {
			cDict[tag] = processElement(child)
		}
	}
	latexS := getVal(groupPr.chr, "", CHR)
	return groupPr.text + formatPositional(latexS, cDict["e"])
}

// doRad handles radical elements.
func doRad(elm *OMMLElement) string {
	cDict := processChildrenDict(elm, nil)
	text := cDict["e"]
	deg := cDict["deg"]
	if deg != "" {
		return formatTemplate(RAD, map[string]string{"deg": deg, "text": text})
	}
	return formatTemplate(RAD_DEFAULT, map[string]string{"text": text})
}

// doEqArr handles equation array elements.
func doEqArr(elm *OMMLElement) string {
	include := map[string]bool{"e": true}
	var parts []string
	for _, tt := range processChildrenList(elm, include) {
		parts = append(parts, tt.text)
	}
	return formatTemplate(ARR, map[string]string{"text": strings.Join(parts, Brk)})
}

// doLimLow handles lower-limit elements.
func doLimLow(elm *OMMLElement) string {
	include := map[string]bool{"e": true, "lim": true}
	tDict := processChildrenDict(elm, include)
	latexS, ok := LIM_FUNC[tDict["e"]]
	if !ok {
		return tDict["e"] + "_{" + tDict["lim"] + "}"
	}
	return formatTemplate(latexS, map[string]string{"lim": tDict["lim"]})
}

// doLimUpp handles upper-limit elements.
func doLimUpp(elm *OMMLElement) string {
	include := map[string]bool{"e": true, "lim": true}
	tDict := processChildrenDict(elm, include)
	return formatTemplate(LIM_UPP, map[string]string{
		"lim":  tDict["lim"],
		"text": tDict["e"],
	})
}

// doLim handles limit content elements.
func doLim(elm *OMMLElement) string {
	text := processChildren(elm, nil)
	return strings.ReplaceAll(text, LIM_TO[0], LIM_TO[1])
}

// doM handles matrix elements.
func doM(elm *OMMLElement) string {
	var rows []string
	for _, tt := range processChildrenList(elm, nil) {
		if tt.tag == "mPr" {
			continue
		}
		if tt.tag == "mr" {
			rows = append(rows, tt.text)
		}
	}
	return formatTemplate(M_TEMPLATE, map[string]string{"text": strings.Join(rows, Brk)})
}

// doMr handles matrix row elements.
func doMr(elm *OMMLElement) string {
	include := map[string]bool{"e": true}
	var parts []string
	for _, tt := range processChildrenList(elm, include) {
		parts = append(parts, tt.text)
	}
	return strings.Join(parts, Aln)
}

// doNary handles n-ary operator elements.
func doNary(elm *OMMLElement) string {
	var res []string
	bo := ""
	for _, tt := range processChildrenList(elm, nil) {
		if tt.tag == "naryPr" {
			// The naryPr contains the big operator character
			pr := parsePr(elm.findChild("naryPr"))
			bo = getVal(pr.chr, "", CHR_BO)
		} else {
			res = append(res, tt.text)
		}
	}
	return bo + strings.Join(res, "")
}

// doR handles text run elements.
func doR(elm *OMMLElement) string {
	// Find the <t> child element
	t := elm.findChild("t")
	if t == nil {
		return ""
	}
	text := t.Content
	var parts []string
	for _, ch := range text {
		s := string(ch)
		if rep, ok := T[s]; ok {
			parts = append(parts, rep)
		} else {
			parts = append(parts, s)
		}
	}
	return EscapeLatex(strings.Join(parts, ""))
}
