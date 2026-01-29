package tui

import (
	"strings"

	"github.com/skip2/go-qrcode"
)

func generateQRCode(url string) string {
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return "Unable to generate QR code"
	}

	qr.DisableBorder = true

	ascii := qr.ToSmallString(false)

	lines := strings.Split(ascii, "\n")
	var formattedLines []string
	for _, line := range lines {
		if line != "" {
			formattedLines = append(formattedLines, line)
		}
	}

	return strings.Join(formattedLines, "\n")
}
