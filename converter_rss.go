package markitdown

import (
	"fmt"
	"io"
	"strings"

	"github.com/mmcdole/gofeed"
)

// RSSConverter handles RSS and Atom feed files.
type RSSConverter struct{}

// NewRSSConverter creates a new RSSConverter.
func NewRSSConverter() *RSSConverter {
	return &RSSConverter{}
}

func (c *RSSConverter) Accepts(info StreamInfo) bool {
	switch info.Extension {
	case ".rss", ".atom":
		return true
	}
	mime := strings.ToLower(info.MIMEType)
	switch {
	case strings.HasPrefix(mime, "application/rss"),
		strings.HasPrefix(mime, "application/atom"),
		strings.HasPrefix(mime, "application/rss+xml"),
		strings.HasPrefix(mime, "application/atom+xml"):
		return true
	}
	// For .xml and generic XML mime types, we'll try to parse
	if info.Extension == ".xml" ||
		strings.HasPrefix(mime, "text/xml") ||
		strings.HasPrefix(mime, "application/xml") {
		return true
	}
	return false
}

func (c *RSSConverter) Convert(reader io.ReadSeeker, info StreamInfo) (*DocumentConverterResult, error) {
	fp := gofeed.NewParser()
	feed, err := fp.Parse(reader)
	if err != nil {
		return nil, fmt.Errorf("parse feed: %w", err)
	}

	var b strings.Builder
	title := feed.Title

	// Feed title as H1
	if feed.Title != "" {
		fmt.Fprintf(&b, "# %s\n", feed.Title)
	}

	// Feed description
	if feed.Description != "" {
		fmt.Fprintf(&b, "%s\n", feed.Description)
	}

	b.WriteString("\n")

	// Feed items
	for _, item := range feed.Items {
		// Item title as H2
		if item.Title != "" {
			fmt.Fprintf(&b, "## %s\n", item.Title)
		}

		// Publication date
		if item.Published != "" {
			fmt.Fprintf(&b, "Published: %s\n\n", item.Published)
		} else if item.Updated != "" {
			fmt.Fprintf(&b, "Updated: %s\n\n", item.Updated)
		}

		// Item content
		content := item.Content
		if content == "" {
			content = item.Description
		}
		if content != "" {
			// If content looks like HTML, convert it
			if strings.Contains(content, "<") && strings.Contains(content, ">") {
				md, err := convertHTMLToMarkdown(content)
				if err == nil {
					content = md
				}
			}
			b.WriteString(content)
			b.WriteString("\n")
		}

		b.WriteString("\n")
	}

	return &DocumentConverterResult{
		Markdown: b.String(),
		Title:    title,
	}, nil
}
