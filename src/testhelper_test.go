package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
)

// buildMessage builds a minimal multipart/mixed RFC 5322 message with one attachment.
func buildMessage(filename string, data []byte) []byte {
	boundary := "testboundary001"
	encoded := base64.StdEncoding.EncodeToString(data)

	var b strings.Builder
	fmt.Fprintf(&b, "From: sender@example.com\r\n")
	fmt.Fprintf(&b, "To: recipient@example.com\r\n")
	fmt.Fprintf(&b, "Subject: test message\r\n")
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Content-Type: multipart/mixed; boundary=%q\r\n", boundary)
	fmt.Fprintf(&b, "\r\n")
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	fmt.Fprintf(&b, "Content-Type: text/plain\r\n")
	fmt.Fprintf(&b, "\r\n")
	fmt.Fprintf(&b, "body text\r\n")
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	fmt.Fprintf(&b, "Content-Type: application/octet-stream\r\n")
	fmt.Fprintf(&b, "Content-Disposition: attachment; filename=%q\r\n", filename)
	fmt.Fprintf(&b, "Content-Transfer-Encoding: base64\r\n")
	fmt.Fprintf(&b, "\r\n")

	// Wrap base64 at 76 chars per line as per MIME spec
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		fmt.Fprintf(&b, "%s\r\n", encoded[i:end])
	}

	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return []byte(b.String())
}

// buildMessageRawFilename is like buildMessage but writes the Content-Disposition
// filename parameter verbatim, allowing RFC 2047 encoded-word tokens to be used.
func buildMessageRawFilename(rawFilename string, data []byte) []byte {
	boundary := "testboundary001"
	encoded := base64.StdEncoding.EncodeToString(data)

	var b strings.Builder
	fmt.Fprintf(&b, "From: sender@example.com\r\n")
	fmt.Fprintf(&b, "To: recipient@example.com\r\n")
	fmt.Fprintf(&b, "Subject: test message\r\n")
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Content-Type: multipart/mixed; boundary=%q\r\n", boundary)
	fmt.Fprintf(&b, "\r\n")
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	fmt.Fprintf(&b, "Content-Type: text/plain\r\n")
	fmt.Fprintf(&b, "\r\n")
	fmt.Fprintf(&b, "body text\r\n")
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	fmt.Fprintf(&b, "Content-Type: application/octet-stream\r\n")
	fmt.Fprintf(&b, "Content-Disposition: attachment; filename=%s\r\n", rawFilename)
	fmt.Fprintf(&b, "Content-Transfer-Encoding: base64\r\n")
	fmt.Fprintf(&b, "\r\n")

	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		fmt.Fprintf(&b, "%s\r\n", encoded[i:end])
	}

	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return []byte(b.String())
}

// ensure buildMessage compiles even without usages in this file
var _ = bytes.NewReader
