package markitdown

import (
	"os"
	"strings"
	"testing"
)

// testVector defines a test case matching the Python test vectors.
type testVector struct {
	filename       string
	mustInclude    []string
	mustNotInclude []string
}

// generalTestVectors matches the Python GENERAL_TEST_VECTORS.
var generalTestVectors = []testVector{
	{
		filename: "test.docx",
		mustInclude: []string{
			"314b0a30-5b04-470b-b9f7-eed2c2bec74a",
			"49e168b7-d2ae-407f-a055-2167576f39a1",
			"## d666f1f7-46cb-42bd-9a39-9a39cf2a509f",
			"# Abstract",
			"# Introduction",
			"AutoGen: Enabling Next-Gen LLM Applications via Multi-Agent Conversation",
		},
	},
	{
		filename: "test.xlsx",
		mustInclude: []string{
			"09060124-b5e7-4717-9d07-3c046eb",
			"6ff4173b-42a5-4784-9b19-f49caff4d93d",
			"affc7dad-52dc-4b98-9b5d-51e65d8a8ad0",
		},
	},
	{
		filename: "test.xls",
		mustInclude: []string{
			"09060124-b5e7-4717-9d07-3c046eb",
			"6ff4173b-42a5-4784-9b19-f49caff4d93d",
			"affc7dad-52dc-4b98-9b5d-51e65d8a8ad0",
		},
	},
	{
		filename: "test.pptx",
		mustInclude: []string{
			"2cdda5c8-e50e-4db4-b5f0-9722a649f455",
			"04191ea8-5c73-4215-a1d3-1cfb43aaaf12",
			"44bf7d06-5e7a-4a40-a2e1-a2e42ef28c8a",
			"1b92870d-e3b5-4e65-8153-919f4ff45592",
			"AutoGen: Enabling Next-Gen LLM Applications via Multi-Agent Conversation",
		},
	},
	{
		filename: "test.pdf",
		mustInclude: []string{
			// Note: ledongthuc/pdf library doesn't always detect word boundaries,
			// so we check for key phrases that may appear with or without spaces
			"contemporaneous",
			"multi-agent",
			"LLM",
		},
	},
	{
		filename: "test_blog.html",
		mustInclude: []string{
			"Large language models (LLMs) are powerful tools",
		},
	},
	{
		filename: "test_mskanji.csv",
		mustInclude: []string{
			"佐藤太郎",
			"三木英子",
			"髙橋淳",
		},
	},
	{
		filename: "test.json",
		mustInclude: []string{
			"5b64c88c-b3c3-4510-bcb8-da0b200602d8",
			"9700dc99-6685-40b4-9a3a-5e406dcb37f3",
		},
	},
	{
		filename: "test_rss.xml",
		mustInclude: []string{
			"The Official Microsoft Blog",
			"Ignite 2024",
		},
		mustNotInclude: []string{
			"<rss",
			"<feed",
		},
	},
	{
		filename: "test_notebook.ipynb",
		mustInclude: []string{
			"# Test Notebook",
			"```python",
			`print("markitdown")`,
			"```",
			"## Code Cell Below",
		},
		mustNotInclude: []string{
			"nbformat",
			"nbformat_minor",
		},
	},
	{
		filename: "test.epub",
		mustInclude: []string{
			"Test Author",
			"A test EPUB document for MarkItDown testing",
			"Chapter 1",
			"test",
		},
	},
}

func TestConvertFile(t *testing.T) {
	m := New()

	for _, tv := range generalTestVectors {
		t.Run(tv.filename, func(t *testing.T) {
			path := "testdata/" + tv.filename
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Skipf("test fixture %s not found", path)
			}

			result, err := m.ConvertFile(path)
			if err != nil {
				t.Fatalf("ConvertFile(%s) error: %v", tv.filename, err)
			}

			if result == nil {
				t.Fatalf("ConvertFile(%s) returned nil result", tv.filename)
			}

			md := result.Markdown

			for _, s := range tv.mustInclude {
				if !strings.Contains(md, s) {
					t.Errorf("ConvertFile(%s): expected output to contain %q\nGot (first 2000 chars):\n%s", tv.filename, s, truncate(md, 2000))
				}
			}

			for _, s := range tv.mustNotInclude {
				if strings.Contains(md, s) {
					t.Errorf("ConvertFile(%s): expected output NOT to contain %q", tv.filename, s)
				}
			}
		})
	}
}

func TestConvertReader(t *testing.T) {
	m := New()

	// Test CSV with charset hint (Japanese)
	t.Run("csv_with_charset", func(t *testing.T) {
		path := "testdata/test_mskanji.csv"
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Skip("test fixture not found")
		}

		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		result, err := m.ConvertReader(f, StreamInfo{
			Extension: ".csv",
			MIMEType:  "text/csv",
			Charset:   "cp932",
		})
		if err != nil {
			t.Fatalf("ConvertReader error: %v", err)
		}

		for _, expected := range []string{"佐藤太郎", "三木英子", "髙橋淳", "名前", "年齢", "住所"} {
			if !strings.Contains(result.Markdown, expected) {
				t.Errorf("expected output to contain %q", expected)
			}
		}
	})
}

func TestNormalization(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "trailing whitespace",
			input: "hello   \nworld   \n",
			want:  "hello\nworld",
		},
		{
			name:  "multiple newlines",
			input: "hello\n\n\n\n\nworld",
			want:  "hello\n\nworld",
		},
		{
			name:  "crlf",
			input: "hello\r\nworld\r\n",
			want:  "hello\nworld",
		},
		{
			name:  "control characters",
			input: "hello\x00world\x01test",
			want:  "helloworldtest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeOutput(tt.input)
			if got != tt.want {
				t.Errorf("normalizeOutput(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestConverterAccepts(t *testing.T) {
	tests := []struct {
		name      string
		converter DocumentConverter
		info      StreamInfo
		want      bool
	}{
		{"pdf by ext", NewPdfConverter(), StreamInfo{Extension: ".pdf"}, true},
		{"pdf by mime", NewPdfConverter(), StreamInfo{MIMEType: "application/pdf"}, true},
		{"pdf wrong ext", NewPdfConverter(), StreamInfo{Extension: ".txt"}, false},
		{"csv by ext", NewCsvConverter(), StreamInfo{Extension: ".csv"}, true},
		{"csv by mime", NewCsvConverter(), StreamInfo{MIMEType: "text/csv"}, true},
		{"html by ext", NewHTMLConverter(nil), StreamInfo{Extension: ".html"}, true},
		{"html by mime", NewHTMLConverter(nil), StreamInfo{MIMEType: "text/html"}, true},
		{"plaintext txt", NewPlainTextConverter(), StreamInfo{Extension: ".txt"}, true},
		{"plaintext json", NewPlainTextConverter(), StreamInfo{Extension: ".json"}, true},
		{"plaintext md", NewPlainTextConverter(), StreamInfo{Extension: ".md"}, true},
		{"rss by ext", NewRSSConverter(), StreamInfo{Extension: ".rss"}, true},
		{"rss xml", NewRSSConverter(), StreamInfo{Extension: ".xml"}, true},
		{"ipynb by ext", NewIpynbConverter(), StreamInfo{Extension: ".ipynb"}, true},
		{"docx by ext", NewDocxConverter(nil), StreamInfo{Extension: ".docx"}, true},
		{"pptx by ext", NewPptxConverter(nil), StreamInfo{Extension: ".pptx"}, true},
		{"xlsx by ext", NewXlsxConverter(), StreamInfo{Extension: ".xlsx"}, true},
		{"xls by ext", NewXlsConverter(), StreamInfo{Extension: ".xls"}, true},
		{"epub by ext", NewEpubConverter(nil), StreamInfo{Extension: ".epub"}, true},
		{"zip by ext", NewZipConverter(nil), StreamInfo{Extension: ".zip"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.converter.Accepts(tt.info)
			if got != tt.want {
				t.Errorf("Accepts() = %v, want %v", got, tt.want)
			}
		})
	}
}

// goldenTestFiles lists files that should match golden output exactly.
var goldenTestFiles = []string{
	"test.docx",
	"test.xlsx",
	"test.xls",
	"test.pptx",
	"test.pdf",
	"test_blog.html",
	"test_mskanji.csv",
	"test.json",
	"test_rss.xml",
	"test_notebook.ipynb",
	"test.epub",
}

func TestGoldenFiles(t *testing.T) {
	m := New()

	for _, filename := range goldenTestFiles {
		t.Run(filename, func(t *testing.T) {
			inputPath := "testdata/" + filename
			goldenPath := "testdata/golden/" + filename + ".md"

			if _, err := os.Stat(inputPath); os.IsNotExist(err) {
				t.Skipf("test fixture %s not found", inputPath)
			}
			if _, err := os.Stat(goldenPath); os.IsNotExist(err) {
				t.Skipf("golden file %s not found", goldenPath)
			}

			result, err := m.ConvertFile(inputPath)
			if err != nil {
				t.Fatalf("ConvertFile(%s) error: %v", filename, err)
			}

			golden, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden file: %v", err)
			}

			want := string(golden)
			got := result.Markdown

			if got != want {
				// Show a useful diff summary
				gotLines := strings.Split(got, "\n")
				wantLines := strings.Split(want, "\n")

				t.Errorf("ConvertFile(%s) output does not match golden file %s", filename, goldenPath)
				t.Errorf("Got %d lines (%d bytes), want %d lines (%d bytes)",
					len(gotLines), len(got), len(wantLines), len(want))

				// Find first difference
				maxLines := len(gotLines)
				if len(wantLines) < maxLines {
					maxLines = len(wantLines)
				}
				for i := 0; i < maxLines; i++ {
					if gotLines[i] != wantLines[i] {
						t.Errorf("First diff at line %d:\n  got:  %q\n  want: %q", i+1, truncate(gotLines[i], 200), truncate(wantLines[i], 200))
						break
					}
				}
				if len(gotLines) != len(wantLines) {
					t.Errorf("Line count differs: got %d, want %d", len(gotLines), len(wantLines))
				}
			}
		})
	}
}

// updateGolden can be set to regenerate golden files.
// Run with: go test -run TestUpdateGolden -v
func TestUpdateGolden(t *testing.T) {
	if os.Getenv("UPDATE_GOLDEN") == "" {
		t.Skip("set UPDATE_GOLDEN=1 to regenerate golden files")
	}

	m := New()
	for _, filename := range goldenTestFiles {
		inputPath := "testdata/" + filename
		goldenPath := "testdata/golden/" + filename + ".md"

		if _, err := os.Stat(inputPath); os.IsNotExist(err) {
			t.Logf("skipping %s (not found)", inputPath)
			continue
		}

		result, err := m.ConvertFile(inputPath)
		if err != nil {
			t.Errorf("ConvertFile(%s) error: %v", filename, err)
			continue
		}

		if err := os.MkdirAll("testdata/golden", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, []byte(result.Markdown), 0o644); err != nil {
			t.Errorf("write golden file: %v", err)
			continue
		}
		t.Logf("Updated %s (%d bytes)", goldenPath, len(result.Markdown))
	}
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
