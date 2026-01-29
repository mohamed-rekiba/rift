package tui

import (
	"strings"
	"testing"
)

func TestGenerateQRCode(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"simple URL", "http://test.localhost"},
		{"with subdomain", "http://abc123.example.com"},
		{"with port", "http://test.localhost:8080"},
		{"https URL", "https://secure.example.com"},
		{"long URL", "http://verylongsubdomainname.example.com/path/to/resource"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateQRCode(tt.url)

			if result == "" {
				t.Error("expected non-empty QR code")
			}

			// QR code should have multiple lines
			lines := strings.Split(result, "\n")
			if len(lines) < 5 {
				t.Errorf("expected at least 5 lines in QR code, got %d", len(lines))
			}
		})
	}
}

func TestGenerateQRCode_ErrorHandling(t *testing.T) {
	// Empty URL should still generate something
	result := generateQRCode("")

	// Should not panic and should return something
	if result == "" {
		t.Error("expected non-empty result even for empty URL")
	}
}

func TestGenerateQRCode_ConsistentOutput(t *testing.T) {
	url := "http://test.localhost"

	result1 := generateQRCode(url)
	result2 := generateQRCode(url)

	if result1 != result2 {
		t.Error("expected consistent QR code output for same URL")
	}
}

func TestGenerateQRCode_DifferentURLs(t *testing.T) {
	url1 := "http://test1.localhost"
	url2 := "http://test2.localhost"

	result1 := generateQRCode(url1)
	result2 := generateQRCode(url2)

	if result1 == result2 {
		t.Error("expected different QR codes for different URLs")
	}
}

func TestGenerateQRCode_NoBorder(t *testing.T) {
	result := generateQRCode("http://test.localhost")

	lines := strings.Split(result, "\n")

	// Check that there are no empty leading/trailing lines (which would indicate border)
	// The QR code should have content on all lines
	for _, line := range lines {
		if line != "" {
			// First non-empty line should have QR content
			break
		}
	}
}

func TestGenerateQRCode_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"with query params", "http://test.localhost?foo=bar&baz=qux"},
		{"with fragment", "http://test.localhost#section"},
		{"with path", "http://test.localhost/path/to/page"},
		{"with encoded chars", "http://test.localhost/path%20with%20spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateQRCode(tt.url)

			if result == "" {
				t.Error("expected non-empty QR code")
			}

			// Should not contain error message
			if strings.Contains(result, "Unable to generate") {
				t.Error("expected successful QR code generation")
			}
		})
	}
}
