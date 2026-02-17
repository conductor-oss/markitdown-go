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
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// IpynbConverter handles Jupyter notebook files.
type IpynbConverter struct{}

// NewIpynbConverter creates a new IpynbConverter.
func NewIpynbConverter() *IpynbConverter {
	return &IpynbConverter{}
}

func (c *IpynbConverter) Accepts(info StreamInfo) bool {
	if info.Extension == ".ipynb" {
		return true
	}
	return false
}

// notebook represents the JSON structure of a Jupyter notebook.
type notebook struct {
	Metadata notebookMetadata `json:"metadata"`
	Cells    []notebookCell   `json:"cells"`
}

type notebookMetadata struct {
	KernelSpec *kernelSpec `json:"kernelspec"`
}

type kernelSpec struct {
	Language string `json:"language"`
}

type notebookCell struct {
	CellType string          `json:"cell_type"`
	Source   json.RawMessage `json:"source"`
	Outputs []cellOutput    `json:"outputs"`
}

type cellOutput struct {
	OutputType string          `json:"output_type"`
	Text       json.RawMessage `json:"text"`
	Data       map[string]json.RawMessage `json:"data"`
}

func (c *IpynbConverter) Convert(reader io.ReadSeeker, info StreamInfo) (*DocumentConverterResult, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}

	var nb notebook
	if err := json.Unmarshal(data, &nb); err != nil {
		return nil, fmt.Errorf("parse notebook JSON: %w", err)
	}

	// Determine language from kernel spec
	language := "python"
	if nb.Metadata.KernelSpec != nil && nb.Metadata.KernelSpec.Language != "" {
		language = nb.Metadata.KernelSpec.Language
	}

	var sections []string
	var title string

	for _, cell := range nb.Cells {
		source := parseSource(cell.Source)

		switch cell.CellType {
		case "markdown":
			sections = append(sections, source)
			// Extract title from first heading
			if title == "" {
				for _, line := range strings.Split(source, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "# ") {
						title = strings.TrimPrefix(line, "# ")
						break
					}
				}
			}

		case "code":
			if strings.TrimSpace(source) != "" {
				sections = append(sections, fmt.Sprintf("```%s\n%s\n```", language, source))
			}
			// Include text outputs
			for _, output := range cell.Outputs {
				text := parseOutputText(output)
				if text != "" {
					sections = append(sections, fmt.Sprintf("```\n%s\n```", text))
				}
			}

		case "raw":
			if strings.TrimSpace(source) != "" {
				sections = append(sections, fmt.Sprintf("```\n%s\n```", source))
			}
		}
	}

	md := strings.Join(sections, "\n\n")

	return &DocumentConverterResult{
		Markdown: md,
		Title:    title,
	}, nil
}

// parseSource extracts the source string from a cell.
// Source can be a string or an array of strings.
func parseSource(raw json.RawMessage) string {
	// Try as string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Try as array of strings
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return strings.Join(arr, "")
	}
	return ""
}

// parseOutputText extracts text output from a cell output.
func parseOutputText(output cellOutput) string {
	// Try "text" field first (for stream outputs)
	if output.Text != nil {
		text := parseSource(output.Text)
		if text != "" {
			return strings.TrimRight(text, "\n")
		}
	}
	// Try data["text/plain"]
	if output.Data != nil {
		if raw, ok := output.Data["text/plain"]; ok {
			text := parseSource(raw)
			if text != "" {
				return strings.TrimRight(text, "\n")
			}
		}
	}
	return ""
}
