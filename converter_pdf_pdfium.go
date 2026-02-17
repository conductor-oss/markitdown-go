//go:build !nopdfium

package markitdown

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

var (
	pdfiumPool     pdfium.Pool
	pdfiumPoolOnce sync.Once
	pdfiumPoolErr  error
)

func initPdfiumPool() {
	pdfiumPool, pdfiumPoolErr = webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
}

// PdfConverter handles PDF files using the PDFium library via WebAssembly.
type PdfConverter struct{}

// NewPdfConverter creates a new PdfConverter.
func NewPdfConverter() *PdfConverter {
	return &PdfConverter{}
}

func (c *PdfConverter) Accepts(info StreamInfo) bool {
	if info.Extension == ".pdf" {
		return true
	}
	mime := strings.ToLower(info.MIMEType)
	return strings.HasPrefix(mime, "application/pdf")
}

func (c *PdfConverter) Convert(reader io.ReadSeeker, info StreamInfo) (*DocumentConverterResult, error) {
	pdfiumPoolOnce.Do(initPdfiumPool)
	if pdfiumPoolErr != nil {
		return nil, fmt.Errorf("init pdfium: %w", pdfiumPoolErr)
	}

	instance, err := pdfiumPool.GetInstance(30 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("get pdfium instance: %w", err)
	}
	defer instance.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read PDF: %w", err)
	}

	doc, err := instance.OpenDocument(&requests.OpenDocument{
		File: &data,
	})
	if err != nil {
		return nil, fmt.Errorf("open PDF: %w", err)
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	pageCountResp, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: doc.Document,
	})
	if err != nil {
		return nil, fmt.Errorf("get page count: %w", err)
	}

	var md strings.Builder

	for i := 0; i < pageCountResp.PageCount; i++ {
		textResp, err := instance.GetPageText(&requests.GetPageText{
			Page: requests.Page{
				ByIndex: &requests.PageByIndex{
					Document: doc.Document,
					Index:    i,
				},
			},
		})
		if err != nil {
			continue
		}

		text := strings.TrimSpace(textResp.Text)
		if text == "" {
			continue
		}

		md.WriteString(text)
		md.WriteString("\n\n")
	}

	result := md.String()
	if strings.TrimSpace(result) == "" {
		return &DocumentConverterResult{
			Markdown: "[No readable text content found in PDF]",
		}, nil
	}

	return &DocumentConverterResult{
		Markdown: result,
	}, nil
}
