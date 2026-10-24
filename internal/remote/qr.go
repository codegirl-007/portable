package remote

import (
	"fmt"
	"io"

	qrcode "github.com/skip2/go-qrcode"
)

// WriteQR prints a compact terminal QR code for content.
func WriteQR(w io.Writer, content string) error {
	q, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return fmt.Errorf("qr: %w", err)
	}
	q.DisableBorder = false
	_, err = io.WriteString(w, q.ToSmallString(true))
	return err
}
