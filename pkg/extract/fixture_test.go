package extract

import (
	"bytes"
	"fmt"
)

// buildFixturePDF produces a minimal valid multi-page PDF with one line of
// Helvetica text per page. Structure validated against poppler's pdftotext.
func buildFixturePDF(pages []string) []byte {
	type obj struct{ body []byte }
	var objs []obj
	kids := ""
	// Object numbering: 1=Catalog, 2=Pages, then per page i: (3+2i)=Page, (4+2i)=Contents; last = Font.
	fontNum := 3 + 2*len(pages)
	for i := range pages {
		kids += fmt.Sprintf("%d 0 R ", 3+2*i)
	}
	objs = append(objs, obj{[]byte("<< /Type /Catalog /Pages 2 0 R >>")})
	objs = append(objs, obj{[]byte(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids, len(pages)))})
	for i, text := range pages {
		objs = append(objs, obj{[]byte(fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents %d 0 R /Resources << /Font << /F1 %d 0 R >> >> >>",
			4+2*i, fontNum))})
		stream := fmt.Sprintf("BT /F1 24 Tf 72 700 Td (%s) Tj ET", text)
		objs = append(objs, obj{[]byte(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))})
	}
	objs = append(objs, obj{[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")})

	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n", i+1)
		out.Write(o.body)
		out.WriteString("\nendobj\n")
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return out.Bytes()
}
