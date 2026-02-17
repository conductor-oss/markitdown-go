package markitdown

import (
	"errors"
	"fmt"
	"strings"
)

// UnsupportedFormatError is returned when no converter can handle the input format.
type UnsupportedFormatError struct {
	Extension string
	MIMEType  string
}

func (e *UnsupportedFormatError) Error() string {
	parts := []string{"unsupported format"}
	if e.Extension != "" {
		parts = append(parts, fmt.Sprintf("extension=%q", e.Extension))
	}
	if e.MIMEType != "" {
		parts = append(parts, fmt.Sprintf("mime=%q", e.MIMEType))
	}
	return strings.Join(parts, " ")
}

// FailedConversionAttempt records a converter that accepted but failed.
type FailedConversionAttempt struct {
	Converter string
	Err       error
}

// ConversionError is returned when a converter accepted the input but failed to convert it.
type ConversionError struct {
	Attempts []FailedConversionAttempt
}

func (e *ConversionError) Error() string {
	if len(e.Attempts) == 0 {
		return "conversion failed"
	}
	var b strings.Builder
	b.WriteString("conversion failed after ")
	fmt.Fprintf(&b, "%d attempt(s):", len(e.Attempts))
	for _, a := range e.Attempts {
		fmt.Fprintf(&b, "\n  %s: %v", a.Converter, a.Err)
	}
	return b.String()
}

func (e *ConversionError) Unwrap() error {
	if len(e.Attempts) > 0 {
		return e.Attempts[len(e.Attempts)-1].Err
	}
	return nil
}

// IsUnsupportedFormat reports whether the error is an UnsupportedFormatError.
func IsUnsupportedFormat(err error) bool {
	var target *UnsupportedFormatError
	return errors.As(err, &target)
}
