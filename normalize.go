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
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	reTrailingWhitespace = regexp.MustCompile(`[ \t]+\n`)
	reMultipleNewlines   = regexp.MustCompile(`\n{3,}`)
	reCRLF               = regexp.MustCompile(`\r\n?`)
)

// normalizeOutput applies post-processing to converter output matching Python behavior:
// - Normalize line endings (CRLF -> LF)
// - Strip trailing whitespace from each line
// - Collapse 3+ consecutive newlines to 2
// - Strip non-printable/control characters (keep \n, \t)
// - Ensure output is valid UTF-8
// - Trim leading/trailing whitespace from final output
func normalizeOutput(s string) string {
	// Ensure valid UTF-8
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}

	// Normalize line endings
	s = reCRLF.ReplaceAllString(s, "\n")

	// Strip non-printable/control characters (keep \n, \t)
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)

	// Strip trailing whitespace from each line
	// We add a trailing newline to ensure the last line is processed
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	s = reTrailingWhitespace.ReplaceAllString(s, "\n")

	// Collapse 3+ consecutive newlines to 2
	s = reMultipleNewlines.ReplaceAllString(s, "\n\n")

	// Trim leading/trailing whitespace
	s = strings.TrimSpace(s)

	return s
}
