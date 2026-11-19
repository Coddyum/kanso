package demo

import (
	"bytes"
	"fmt"
	"strings"
)

// pdf writes a one-page PDF whose text any reader finds, PDFKit included:
// Helvetica, WinAnsi encoding, one line under the other.
func pdf(lines []string) []byte {
	var content strings.Builder
	content.WriteString("BT /F1 12 Tf 56 780 Td 17 TL\n")
	for _, l := range lines {
		content.WriteString("(" + winAnsi(l) + ") '\n")
	}
	content.WriteString("ET")
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", content.Len(), content.String()),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, o := range objects {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return b.Bytes()
}

// winAnsi writes s for a PDF string: Latin-1 and the euro sign as WinAnsi bytes
// (octal escapes), the parentheses and backslash escaped, anything else « ? ».
func winAnsi(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '(' || r == ')' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == 0x20AC:
			b.WriteString(`\200`)
		case r >= 0x20 && r < 0x7F:
			b.WriteRune(r)
		case r >= 0xA0 && r <= 0xFF:
			fmt.Fprintf(&b, `\%03o`, r)
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}
