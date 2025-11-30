package utils

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
)

// DocumentExtractor extracts text from various document formats
type DocumentExtractor struct{}

// NewDocumentExtractor creates a new document extractor
func NewDocumentExtractor() *DocumentExtractor {
	return &DocumentExtractor{}
}

// ExtractText extracts text from a file based on its extension
func (e *DocumentExtractor) ExtractText(file multipart.File, header *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))

	// Read file content
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, file); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	content := buf.Bytes()

	switch ext {
	case ".txt":
		return string(content), nil

	case ".pdf":
		// For PDF extraction, you would integrate a PDF library
		// For now, we return a message indicating Gemini should process it
		// In production, consider using libraries like:
		// - github.com/ledongthuc/pdf
		// - github.com/pdfcpu/pdfcpu
		return e.extractPDFBasic(content)

	case ".doc", ".docx":
		// For Word documents, you would integrate a docx library
		// For now, return basic extraction
		// In production, consider using:
		// - github.com/unidoc/unioffice
		// - github.com/nguyenthenguyen/docx
		return e.extractDocxBasic(content)

	default:
		// Try treating as plain text
		return string(content), nil
	}
}

// extractPDFBasic provides basic PDF text extraction
// In production, use a proper PDF library
func (e *DocumentExtractor) extractPDFBasic(content []byte) (string, error) {
	// Basic approach: look for text between BT and ET markers
	// This is a simplified version - real PDF parsing is more complex

	text := string(content)

	// Remove binary content markers
	if strings.Contains(text, "%PDF") {
		// This is a valid PDF, but we need a proper parser
		// For now, extract any readable ASCII text
		var cleanText strings.Builder
		for _, r := range text {
			if r >= 32 && r <= 126 || r == '\n' || r == '\r' || r == '\t' {
				cleanText.WriteRune(r)
			}
		}

		extracted := cleanText.String()

		// If we got very little text, indicate that this PDF needs proper parsing
		if len(extracted) < 100 {
			return "[PDF document - please paste CV text directly for best results]", nil
		}

		return extracted, nil
	}

	return string(content), nil
}

// extractDocxBasic provides basic DOCX text extraction
// In production, use a proper DOCX library
func (e *DocumentExtractor) extractDocxBasic(content []byte) (string, error) {
	// DOCX files are ZIP archives containing XML
	// The main text is usually in word/document.xml

	// For basic extraction, look for text content
	text := string(content)

	// Check if it looks like a DOCX/ZIP file
	if len(content) > 4 && content[0] == 'P' && content[1] == 'K' {
		// This is a ZIP file (DOCX)
		// For now, extract any readable text between XML tags
		// A proper implementation would unzip and parse XML

		return "[DOCX document - please paste CV text directly for best results]", nil
	}

	// Legacy .doc format
	var cleanText strings.Builder
	for _, r := range text {
		if r >= 32 && r <= 126 || r == '\n' || r == '\r' || r == '\t' {
			cleanText.WriteRune(r)
		}
	}

	return cleanText.String(), nil
}

// IsSupportedFormat checks if the file format is supported
func (e *DocumentExtractor) IsSupportedFormat(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	supportedFormats := []string{".txt", ".pdf", ".doc", ".docx"}

	for _, format := range supportedFormats {
		if ext == format {
			return true
		}
	}
	return false
}
