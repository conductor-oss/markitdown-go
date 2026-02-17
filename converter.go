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
