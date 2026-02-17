package ooxml

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"
)

// Common OOXML namespaces.
const (
	NSRelationships = "http://schemas.openxmlformats.org/package/2006/relationships"
	NSContentTypes  = "http://schemas.openxmlformats.org/package/2006/content-types"

	// DOCX namespaces
	NSWordprocessingML = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	NSDrawingML        = "http://schemas.openxmlformats.org/drawingml/2006/main"
	NSRelDoc           = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	NSOMML             = "http://schemas.openxmlformats.org/officeDocument/2006/math"

	// Presentation namespaces
	NSPresentationML = "http://schemas.openxmlformats.org/presentationml/2006/main"
)

// Relationship represents an OOXML relationship.
type Relationship struct {
	ID         string `xml:"Id,attr"`
	Type       string `xml:"Type,attr"`
	Target     string `xml:"Target,attr"`
	TargetMode string `xml:"TargetMode,attr"`
}

// Relationships is the root element for .rels files.
type Relationships struct {
	XMLName       xml.Name       `xml:"Relationships"`
	Relationships []Relationship `xml:"Relationship"`
}

// ParseRelationships parses a .rels file from the ZIP.
func ParseRelationships(zf *zip.ReadCloser, relsPath string) (map[string]Relationship, error) {
	return parseRelsFromZip(zf, relsPath)
}

// ParseRelationshipsFromReader parses rels from a zip.Reader.
func ParseRelationshipsFromReader(zr *zip.Reader, relsPath string) (map[string]Relationship, error) {
	return parseRelsFromZipReader(zr, relsPath)
}

func parseRelsFromZip(zf *zip.ReadCloser, relsPath string) (map[string]Relationship, error) {
	for _, f := range zf.File {
		if f.Name == relsPath {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return decodeRels(rc)
		}
	}
	return make(map[string]Relationship), nil
}

func parseRelsFromZipReader(zr *zip.Reader, relsPath string) (map[string]Relationship, error) {
	for _, f := range zr.File {
		if f.Name == relsPath {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return decodeRels(rc)
		}
	}
	return make(map[string]Relationship), nil
}

func decodeRels(r io.Reader) (map[string]Relationship, error) {
	var rels Relationships
	if err := xml.NewDecoder(r).Decode(&rels); err != nil {
		return nil, fmt.Errorf("decode relationships: %w", err)
	}
	result := make(map[string]Relationship, len(rels.Relationships))
	for _, rel := range rels.Relationships {
		result[rel.ID] = rel
	}
	return result, nil
}

// ReadFileFromZip reads a file from a zip archive.
func ReadFileFromZip(zr *zip.Reader, name string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("file %q not found in ZIP", name)
}

// RelsPathFor returns the .rels path for a given file in the ZIP.
func RelsPathFor(filePath string) string {
	dir := path.Dir(filePath)
	base := path.Base(filePath)
	if dir == "." {
		return "_rels/" + base + ".rels"
	}
	return dir + "/_rels/" + base + ".rels"
}

// ResolveTarget resolves a relative target path against a base path.
func ResolveTarget(basePath, target string) string {
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(target, "/")
	}
	dir := path.Dir(basePath)
	return path.Join(dir, target)
}
