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

// LaTeX special characters that need escaping.
var latexSpecialChars = map[rune]bool{
	'{': true, '}': true, '_': true, '^': true,
	'#': true, '&': true, '$': true, '%': true, '~': true,
}

const (
	Blank     = ""
	Backslash = "\\"
	Aln       = "&"
	Brk       = "\\\\"
	FuncPlace = "{fe}"
)

// CHR maps Unicode accent characters to LaTeX format strings.
// The {0} placeholder is replaced by the content.
var CHR = map[string]string{
	// Top accents
	"\u0300": "\\grave{{{0}}}",
	"\u0301": "\\acute{{{0}}}",
	"\u0302": "\\hat{{{0}}}",
	"\u0303": "\\tilde{{{0}}}",
	"\u0304": "\\bar{{{0}}}",
	"\u0305": "\\overbar{{{0}}}",
	"\u0306": "\\breve{{{0}}}",
	"\u0307": "\\dot{{{0}}}",
	"\u0308": "\\ddot{{{0}}}",
	"\u0309": "\\ovhook{{{0}}}",
	"\u030a": "\\ocirc{{{0}}}}",
	"\u030c": "\\check{{{0}}}}",
	"\u0310": "\\candra{{{0}}}",
	"\u0312": "\\oturnedcomma{{{0}}}",
	"\u0315": "\\ocommatopright{{{0}}}",
	"\u031a": "\\droang{{{0}}}",
	"\u0338": "\\not{{{0}}}",
	"\u20d0": "\\leftharpoonaccent{{{0}}}",
	"\u20d1": "\\rightharpoonaccent{{{0}}}",
	"\u20d2": "\\vertoverlay{{{0}}}",
	"\u20d6": "\\overleftarrow{{{0}}}",
	"\u20d7": "\\vec{{{0}}}",
	"\u20db": "\\dddot{{{0}}}",
	"\u20dc": "\\ddddot{{{0}}}",
	"\u20e1": "\\overleftrightarrow{{{0}}}",
	"\u20e7": "\\annuity{{{0}}}",
	"\u20e9": "\\widebridgeabove{{{0}}}",
	"\u20f0": "\\asteraccent{{{0}}}",
	// Bottom accents
	"\u0330": "\\wideutilde{{{0}}}",
	"\u0331": "\\underbar{{{0}}}",
	"\u20e8": "\\threeunderdot{{{0}}}",
	"\u20ec": "\\underrightharpoondown{{{0}}}",
	"\u20ed": "\\underleftharpoondown{{{0}}}",
	"\u20ee": "\\underledtarrow{{{0}}}",
	"\u20ef": "\\underrightarrow{{{0}}}",
	// Over | group
	"\u23b4": "\\overbracket{{{0}}}",
	"\u23dc": "\\overparen{{{0}}}",
	"\u23de": "\\overbrace{{{0}}}",
	// Under | group
	"\u23b5": "\\underbracket{{{0}}}",
	"\u23dd": "\\underparen{{{0}}}",
	"\u23df": "\\underbrace{{{0}}}",
}

// CHR_BO maps Unicode big operator characters to LaTeX commands.
var CHR_BO = map[string]string{
	"\u2140": "\\Bbbsum",
	"\u220f": "\\prod",
	"\u2210": "\\coprod",
	"\u2211": "\\sum",
	"\u222b": "\\int",
	"\u22c0": "\\bigwedge",
	"\u22c1": "\\bigvee",
	"\u22c2": "\\bigcap",
	"\u22c3": "\\bigcup",
	"\u2a00": "\\bigodot",
	"\u2a01": "\\bigoplus",
	"\u2a02": "\\bigotimes",
}

// T maps Unicode math characters to LaTeX text representations.
var T = map[string]string{
	"\u2192": "\\rightarrow ",
	// Greek letters
	"\U0001d6fc": "\\alpha ",
	"\U0001d6fd": "\\beta ",
	"\U0001d6fe": "\\gamma ",
	"\U0001d6ff": "\\theta ",
	"\U0001d700": "\\epsilon ",
	"\U0001d701": "\\zeta ",
	"\U0001d702": "\\eta ",
	"\U0001d703": "\\theta ",
	"\U0001d704": "\\iota ",
	"\U0001d705": "\\kappa ",
	"\U0001d706": "\\lambda ",
	"\U0001d707": "\\m ",
	"\U0001d708": "\\n ",
	"\U0001d709": "\\xi ",
	"\U0001d70a": "\\omicron ",
	"\U0001d70b": "\\pi ",
	"\U0001d70c": "\\rho ",
	"\U0001d70d": "\\varsigma ",
	"\U0001d70e": "\\sigma ",
	"\U0001d70f": "\\ta ",
	"\U0001d710": "\\upsilon ",
	"\U0001d711": "\\phi ",
	"\U0001d712": "\\chi ",
	"\U0001d713": "\\psi ",
	"\U0001d714": "\\omega ",
	"\U0001d715": "\\partial ",
	"\U0001d716": "\\varepsilon ",
	"\U0001d717": "\\vartheta ",
	"\U0001d718": "\\varkappa ",
	"\U0001d719": "\\varphi ",
	"\U0001d71a": "\\varrho ",
	"\U0001d71b": "\\varpi ",
	// Relation symbols
	"\u2190": "\\leftarrow ",
	"\u2191": "\\uparrow ",
	"\u2193": "\\downright ",
	"\u2194": "\\leftrightarrow ",
	"\u2195": "\\updownarrow ",
	"\u2196": "\\nwarrow ",
	"\u2197": "\\nearrow ",
	"\u2198": "\\searrow ",
	"\u2199": "\\swarrow ",
	"\u22ee": "\\vdots ",
	"\u22ef": "\\cdots ",
	"\u22f0": "\\adots ",
	"\u22f1": "\\ddots ",
	"\u2260": "\\ne ",
	"\u2264": "\\leq ",
	"\u2265": "\\geq ",
	"\u2266": "\\leqq ",
	"\u2267": "\\geqq ",
	"\u2268": "\\lneqq ",
	"\u2269": "\\gneqq ",
	"\u226a": "\\ll ",
	"\u226b": "\\gg ",
	"\u2208": "\\in ",
	"\u2209": "\\notin ",
	"\u220b": "\\ni ",
	"\u220c": "\\nni ",
	// Ordinary symbols
	"\u221e": "\\infty ",
	// Binary relations
	"\u00b1": "\\pm ",
	"\u2213": "\\mp ",
	// Italic, Latin, uppercase
	"\U0001d434": "A",
	"\U0001d435": "B",
	"\U0001d436": "C",
	"\U0001d437": "D",
	"\U0001d438": "E",
	"\U0001d439": "F",
	"\U0001d43a": "G",
	"\U0001d43b": "H",
	"\U0001d43c": "I",
	"\U0001d43d": "J",
	"\U0001d43e": "K",
	"\U0001d43f": "L",
	"\U0001d440": "M",
	"\U0001d441": "N",
	"\U0001d442": "O",
	"\U0001d443": "P",
	"\U0001d444": "Q",
	"\U0001d445": "R",
	"\U0001d446": "S",
	"\U0001d447": "T",
	"\U0001d448": "U",
	"\U0001d449": "V",
	"\U0001d44a": "W",
	"\U0001d44b": "X",
	"\U0001d44c": "Y",
	"\U0001d44d": "Z",
	// Italic, Latin, lowercase
	"\U0001d44e": "a",
	"\U0001d44f": "b",
	"\U0001d450": "c",
	"\U0001d451": "d",
	"\U0001d452": "e",
	"\U0001d453": "f",
	"\U0001d454": "g",
	"\U0001d456": "i",
	"\U0001d457": "j",
	"\U0001d458": "k",
	"\U0001d459": "l",
	"\U0001d45a": "m",
	"\U0001d45b": "n",
	"\U0001d45c": "o",
	"\U0001d45d": "p",
	"\U0001d45e": "q",
	"\U0001d45f": "r",
	"\U0001d460": "s",
	"\U0001d461": "t",
	"\U0001d462": "u",
	"\U0001d463": "v",
	"\U0001d464": "w",
	"\U0001d465": "x",
	"\U0001d466": "y",
	"\U0001d467": "z",
}

// FUNC maps function names to LaTeX templates.
var FUNC = map[string]string{
	"sin":    "\\sin({fe})",
	"cos":    "\\cos({fe})",
	"tan":    "\\tan({fe})",
	"arcsin": "\\arcsin({fe})",
	"arccos": "\\arccos({fe})",
	"arctan": "\\arctan({fe})",
	"arccot": "\\arccot({fe})",
	"sinh":   "\\sinh({fe})",
	"cosh":   "\\cosh({fe})",
	"tanh":   "\\tanh({fe})",
	"coth":   "\\coth({fe})",
	"sec":    "\\sec({fe})",
	"csc":    "\\csc({fe})",
}

// CHR_DEFAULT stores default character values.
var CHR_DEFAULT = map[string]string{
	"ACC_VAL": "\\hat{{{0}}}",
}

// POS maps bar position to LaTeX format strings.
var POS = map[string]string{
	"top": "\\overline{{{0}}}",
	"bot": "\\underline{{{0}}}",
}

// POS_DEFAULT stores default position values.
var POS_DEFAULT = map[string]string{
	"BAR_VAL": "\\overline{{{0}}}",
}

// F maps fraction types to LaTeX format strings.
var F = map[string]string{
	"bar":   "\\frac{{{num}}}{{{den}}}",
	"skw":   "^{{{num}}}/_{{{den}}}",
	"noBar": "\\genfrac{{}}{{}}{{0pt}}{{}}{{{num}}}{{{den}}}",
	"lin":   "{{{num}}}/{{{den}}}",
}

// F_DEFAULT is the default fraction format.
const F_DEFAULT = "\\frac{{{num}}}{{{den}}}"

// D_TEMPLATE is the delimiter template.
const D_TEMPLATE = "\\left{left}{text}\\right{right}"

// D_DEFAULT stores default delimiter values.
var D_DEFAULT = map[string]string{
	"left":  "(",
	"right": ")",
	"null":  ".",
}

// RAD is the radical template with degree.
const RAD = "\\sqrt[{deg}]{{{text}}}"

// RAD_DEFAULT is the radical template without degree.
const RAD_DEFAULT = "\\sqrt{{{text}}}"

// ARR is the array template.
const ARR = "\\begin{{array}}{{c}}{text}\\end{{array}}"

// LIM_FUNC maps limit function names to LaTeX templates.
var LIM_FUNC = map[string]string{
	"lim": "\\lim_{{{lim}}}",
	"max": "\\max_{{{lim}}}",
	"min": "\\min_{{{lim}}}",
}

// LIM_TO defines the replacement pair for limit arrows.
var LIM_TO = [2]string{"\\rightarrow", "\\to"}

// LIM_UPP is the upper limit template.
const LIM_UPP = "\\overset{{{lim}}}{{{text}}}"

// M_TEMPLATE is the matrix template.
const M_TEMPLATE = "\\begin{{matrix}}{text}\\end{{matrix}}"

// SUB is the subscript template.
const SUB = "_{{{0}}}"

// SUP is the superscript template.
const SUP = "^{{{0}}}"
