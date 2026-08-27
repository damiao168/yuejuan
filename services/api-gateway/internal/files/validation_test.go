package files

import (
	"errors"
	"testing"
)

func TestValidateFileTypeRequiresAllowedSniffedContentType(t *testing.T) {
	_, err := ValidateFileType("answer.pdf", "application/pdf", "text/plain; charset=utf-8", []string{".pdf"})
	if !errors.Is(err, ErrInvalidFile) {
		t.Fatalf("declared PDF must not override sniffed text content, got %v", err)
	}
}

func TestValidateFileTypeRejectsUnsupportedDeclaration(t *testing.T) {
	_, err := ValidateFileType("answer.pdf", "text/plain", "application/pdf", []string{".pdf"})
	if !errors.Is(err, ErrInvalidFile) {
		t.Fatalf("unsupported declared type must be rejected, got %v", err)
	}
}

func TestValidateFileTypeReturnsServerSniffedType(t *testing.T) {
	contentType, err := ValidateFileType("roster.csv", "application/vnd.ms-excel", "text/plain; charset=utf-8", []string{".csv"})
	if err != nil {
		t.Fatalf("valid CSV type pair was rejected: %v", err)
	}
	if contentType != "text/plain; charset=utf-8" {
		t.Fatalf("expected server-sniffed content type, got %q", contentType)
	}
}
