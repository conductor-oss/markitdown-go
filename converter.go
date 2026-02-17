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

import "io"

// StreamInfo holds metadata about the input being converted.
type StreamInfo struct {
	MIMEType  string
	Extension string
	Charset   string
	Filename  string
	LocalPath string
	URL       string
}

// DocumentConverterResult holds the output of a conversion.
type DocumentConverterResult struct {
	Markdown string
	Title    string
}

// DocumentConverter is the interface all format converters implement.
type DocumentConverter interface {
	// Accepts returns true if this converter can handle the given input.
	// It MUST NOT change the read position of reader.
	Accepts(info StreamInfo) bool

	// Convert performs the actual document-to-markdown conversion.
	Convert(reader io.ReadSeeker, info StreamInfo) (*DocumentConverterResult, error)
}
