package demodata

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
)

// pdfLineHeight and the margins lay out the one-page documents (A4, points).
const (
	pdfPageWidth  = 595
	pdfPageHeight = 842
	pdfMargin     = 56
	pdfLineHeight = 16
)

// renderPDF returns a one-page PDF holding the title, the lines and the footer,
// in Helvetica (WinAnsi encoding, enough for French, English and German).
func renderPDF(title string, lines []string, footer string) []byte {
	var stream strings.Builder
	y := pdfPageHeight - pdfMargin
	write := func(size, x, y int, text string) {
		fmt.Fprintf(&stream, "BT /F1 %d Tf %d %d Td (%s) Tj ET\n", size, x, y, pdfText(text))
	}
	write(16, pdfMargin, y, title)
	y -= 2 * pdfLineHeight
	for _, line := range lines {
		write(11, pdfMargin, y, line)
		y -= pdfLineHeight
	}
	write(8, pdfMargin, pdfMargin/2, footer)

	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>", pdfPageWidth, pdfPageHeight),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", stream.Len(), stream.String()),
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return out.Bytes()
}

// winAnsiExtras maps the typographic characters outside Latin-1 to WinAnsi.
var winAnsiExtras = map[rune]byte{'—': 0x97, '–': 0x96, '’': 0x92, '‘': 0x91, '“': 0x93, '”': 0x94, '…': 0x85, '€': 0x80, '«': 0xAB, '»': 0xBB}

// pdfText encodes s as a PDF literal string body in WinAnsi encoding.
func pdfText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '(' || r == ')' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r >= 0x20 && r < 0x7F:
			b.WriteRune(r)
		case winAnsiExtras[r] != 0:
			fmt.Fprintf(&b, "\\%03o", winAnsiExtras[r])
		case r >= 0xA0 && r <= 0xFF:
			fmt.Fprintf(&b, "\\%03o", r)
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}

// renderPhoto returns a small PNG standing for a photograph: a night sky over a
// street, its colours drawn from seed so each photo has its own bytes.
func renderPhoto(seed int) []byte {
	const width, height = 320, 240
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			c := color.RGBA{R: uint8(20 + y/8), G: uint8(30 + y/6), B: uint8(60 + y/4 + seed%40), A: 255}
			if y > height*2/3 {
				c = color.RGBA{R: 70, G: 70, B: uint8(75 + seed%20), A: 255}
			}
			if x > 150 && x < 156 && y > 60 {
				c = color.RGBA{R: 40, G: 40, B: 45, A: 255}
			}
			img.Set(x, y, c)
		}
	}
	var out bytes.Buffer
	_ = png.Encode(&out, img) // encoding an in-memory RGBA image does not fail
	return out.Bytes()
}
