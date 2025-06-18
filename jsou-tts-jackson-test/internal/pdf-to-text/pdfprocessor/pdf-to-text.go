package pdfprocessor

import (
	"fmt"
	"log"
	"strings"

	"github.com/dslipak/pdf"
)

// ExtractTextFromFilePath takes the file path to a PDF document and extracts
// all readable text from it. It returns the concatenated text and any error encountered.
func ExtractTextFromPDFFilePath(filePath string) (string, error) {
	pdfReader, err := pdf.Open(filePath) // Open the PDF directly from the file path
	if err != nil {
		return "", fmt.Errorf("failed to open PDF file %s for extraction: %w", filePath, err)
	}

	var extractedText strings.Builder
	numPages := pdfReader.NumPage()
	if numPages == 0 {
		return "", nil // No pages, no text
	}

	for i := 1; i <= numPages; i++ {
		page := pdfReader.Page(i)
		text, err := page.GetPlainText(nil) // nil for fonts to use default text extraction
		if err != nil {
			log.Printf("Warning: Failed to extract text from page %d of %s: %v", i, filePath, err)
			continue // Continue with other pages even if one fails
		}
		extractedText.WriteString(text)
	}

	return extractedText.String(), nil
}
