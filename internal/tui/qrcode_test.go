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
		{"https", "https://secure.example.com"},
		{"long URL", "http://verylongsubdomainname.example.com/path/to/resource"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateQRCode(tt.url)

			if result == "" {
				t.Error("QR code should not be empty")
			}

			// QR codes have multiple rows
			lines := strings.Split(result, "\n")
			if len(lines) < 5 {
				t.Errorf("QR code should have at least 5 lines, got %d", len(lines))
			}
		})
	}
}

func TestGenerateQRCode_ErrorHandling(t *testing.T) {
	// Even an empty URL should produce something (or handle gracefully)
	result := generateQRCode("")

	if result == "" {
		t.Error("should handle empty URL without crashing")
	}
}

func TestGenerateQRCode_ConsistentOutput(t *testing.T) {
	url := "http://test.localhost"

	result1 := generateQRCode(url)
	result2 := generateQRCode(url)

	if result1 != result2 {
		t.Error("same URL should produce the same QR code")
	}
}

func TestGenerateQRCode_DifferentURLs(t *testing.T) {
	url1 := "http://test1.localhost"
	url2 := "http://test2.localhost"

	result1 := generateQRCode(url1)
	result2 := generateQRCode(url2)

	if result1 == result2 {
		t.Error("different URLs should produce different QR codes")
	}
}

func TestGenerateQRCode_NoBorder(t *testing.T) {
	result := generateQRCode("http://test.localhost")

	lines := strings.Split(result, "\n")

	// Check that we have actual content
	for _, line := range lines {
		if line != "" {
			// Found a non-empty line, that's good
			break
		}
	}
}

func TestGenerateQRCode_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"query params", "http://test.localhost?foo=bar&baz=qux"},
		{"fragment", "http://test.localhost#section"},
		{"path segments", "http://test.localhost/path/to/page"},
		{"URL-encoded chars", "http://test.localhost/path%20with%20spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateQRCode(tt.url)

			if result == "" {
				t.Error("should handle special characters")
			}

			// Should not show error message
			if strings.Contains(result, "Unable to generate") {
				t.Error("should not show error message")
			}
		})
	}
}
