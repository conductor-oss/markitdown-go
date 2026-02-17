package markitdown

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/nicholasgasior/markitdown-go/internal/ooxml"
)

// EpubConverter handles EPUB files.
type EpubConverter struct {
	markitdown *MarkItDown
}

// NewEpubConverter creates a new EpubConverter.
func NewEpubConverter(m *MarkItDown) *EpubConverter {
	return &EpubConverter{markitdown: m}
}

func (c *EpubConverter) Accepts(info StreamInfo) bool {
	if info.Extension == ".epub" {
		return true
	}
	mime := strings.ToLower(info.MIMEType)
	return strings.HasPrefix(mime, "application/epub") ||
		strings.HasPrefix(mime, "application/epub+zip") ||
		strings.HasPrefix(mime, "application/x-epub+zip")
}

func (c *EpubConverter) Convert(reader io.ReadSeeker, info StreamInfo) (*DocumentConverterResult, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read EPUB: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open EPUB ZIP: %w", err)
	}

	// Find OPF file path from container.xml
	opfPath, err := c.findOPFPath(zr)
	if err != nil {
		return nil, fmt.Errorf("find OPF: %w", err)
	}

	// Parse OPF for metadata, manifest, and spine
	metadata, manifest, spine, err := c.parseOPF(zr, opfPath)
	if err != nil {
		return nil, fmt.Errorf("parse OPF: %w", err)
	}

	var md strings.Builder

	// Add metadata
	if metadata.title != "" {
		md.WriteString(fmt.Sprintf("# %s\n\n", metadata.title))
	}
	if len(metadata.authors) > 0 {
		md.WriteString(fmt.Sprintf("**Authors:** %s\n\n", strings.Join(metadata.authors, ", ")))
	}
	if metadata.language != "" {
		md.WriteString(fmt.Sprintf("**Language:** %s\n\n", metadata.language))
	}
	if metadata.publisher != "" {
		md.WriteString(fmt.Sprintf("**Publisher:** %s\n\n", metadata.publisher))
	}
	if metadata.date != "" {
		md.WriteString(fmt.Sprintf("**Date:** %s\n\n", metadata.date))
	}
	if metadata.description != "" {
		md.WriteString(fmt.Sprintf("**Description:** %s\n\n", metadata.description))
	}

	// Process spine items in reading order
	opfDir := path.Dir(opfPath)
	htmlConv := NewHTMLConverter(c.markitdown)

	for _, itemRef := range spine {
		item, ok := manifest[itemRef]
		if !ok {
			continue
		}

		// Resolve file path relative to OPF directory
		filePath := item.href
		if opfDir != "." && !strings.HasPrefix(filePath, "/") {
			filePath = opfDir + "/" + filePath
		}

		// Read file from ZIP
		fileData, err := ooxml.ReadFileFromZip(zr, filePath)
		if err != nil {
			continue
		}

		// Determine if it's HTML/XHTML content
		ext := strings.ToLower(path.Ext(filePath))
		isHTML := ext == ".html" || ext == ".htm" || ext == ".xhtml" ||
			strings.Contains(item.mediaType, "html") || strings.Contains(item.mediaType, "xhtml")

		if isHTML {
			result, err := htmlConv.ConvertString(string(fileData))
			if err == nil && strings.TrimSpace(result.Markdown) != "" {
				md.WriteString(result.Markdown)
				md.WriteString("\n\n")
			}
		}
	}

	return &DocumentConverterResult{
		Markdown: md.String(),
		Title:    metadata.title,
	}, nil
}

type epubMetadata struct {
	title       string
	authors     []string
	language    string
	publisher   string
	date        string
	description string
	identifier  string
}

type manifestItem struct {
	id        string
	href      string
	mediaType string
}

// findOPFPath finds the OPF file path from META-INF/container.xml.
func (c *EpubConverter) findOPFPath(zr *zip.Reader) (string, error) {
	data, err := ooxml.ReadFileFromZip(zr, "META-INF/container.xml")
	if err != nil {
		return "", err
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		if se, ok := tok.(xml.StartElement); ok {
			if se.Name.Local == "rootfile" {
				for _, attr := range se.Attr {
					if attr.Name.Local == "full-path" {
						return attr.Value, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("rootfile not found in container.xml")
}

// parseOPF parses the OPF file for metadata, manifest, and spine.
func (c *EpubConverter) parseOPF(zr *zip.Reader, opfPath string) (epubMetadata, map[string]manifestItem, []string, error) {
	data, err := ooxml.ReadFileFromZip(zr, opfPath)
	if err != nil {
		return epubMetadata{}, nil, nil, err
	}

	var meta epubMetadata
	manifest := make(map[string]manifestItem)
	var spine []string

	decoder := xml.NewDecoder(bytes.NewReader(data))

	var inMetadata bool
	var currentTag string

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			local := t.Name.Local

			switch local {
			case "metadata":
				inMetadata = true

			case "title":
				if inMetadata {
					currentTag = "title"
				}

			case "creator":
				if inMetadata {
					currentTag = "creator"
				}

			case "language":
				if inMetadata {
					currentTag = "language"
				}

			case "publisher":
				if inMetadata {
					currentTag = "publisher"
				}

			case "date":
				if inMetadata {
					currentTag = "date"
				}

			case "description":
				if inMetadata {
					currentTag = "description"
				}

			case "identifier":
				if inMetadata {
					currentTag = "identifier"
				}

			case "item":
				var item manifestItem
				for _, attr := range t.Attr {
					switch attr.Name.Local {
					case "id":
						item.id = attr.Value
					case "href":
						item.href = attr.Value
					case "media-type":
						item.mediaType = attr.Value
					}
				}
				if item.id != "" {
					manifest[item.id] = item
				}

			case "itemref":
				for _, attr := range t.Attr {
					if attr.Name.Local == "idref" {
						spine = append(spine, attr.Value)
					}
				}
			}

		case xml.CharData:
			if inMetadata {
				text := strings.TrimSpace(string(t))
				switch currentTag {
				case "title":
					meta.title = text
				case "creator":
					if text != "" {
						meta.authors = append(meta.authors, text)
					}
				case "language":
					meta.language = text
				case "publisher":
					meta.publisher = text
				case "date":
					meta.date = text
				case "description":
					meta.description = text
				case "identifier":
					meta.identifier = text
				}
			}

		case xml.EndElement:
			if t.Name.Local == "metadata" {
				inMetadata = false
			}
			currentTag = ""
		}
	}

	return meta, manifest, spine, nil
}
