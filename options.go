package markitdown

// Option configures a MarkItDown instance.
type Option func(*MarkItDown)

// WithKeepDataURIs configures whether to keep full data URIs in output
// (default: false, which truncates them to data:mime/type;base64...).
func WithKeepDataURIs(keep bool) Option {
	return func(m *MarkItDown) {
		m.keepDataURIs = keep
	}
}

// WithStyleMap sets custom style mapping for DOCX conversion.
func WithStyleMap(styleMap string) Option {
	return func(m *MarkItDown) {
		m.styleMap = styleMap
	}
}
