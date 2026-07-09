package extract

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// readZipEntry returns the decompressed named entry from a zip archive.
func readZipEntry(data []byte, name string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("not a valid OOXML (zip) file: %v", err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("OOXML entry %q not found", name)
}

// collectText streams an XML document and gathers character data inside
// elements with the given local name ("t" for both DOCX runs and XLSX
// shared strings). breakLocal ("p" for DOCX paragraphs, "si" for XLSX
// strings) inserts newlines between logical units.
func collectText(doc []byte, textLocal, breakLocal string) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(doc))
	var b strings.Builder
	depth := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("parse OOXML xml: %v", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == textLocal {
				depth++
			}
		case xml.EndElement:
			if t.Name.Local == textLocal && depth > 0 {
				depth--
			}
			if t.Name.Local == breakLocal {
				b.WriteString("\n")
			}
		case xml.CharData:
			if depth > 0 {
				b.Write(t)
			}
		}
	}
	return strings.TrimSpace(b.String()), nil
}

// ExtractDOCXText extracts paragraph text from word/document.xml.
func ExtractDOCXText(data []byte) ([]PageText, error) {
	doc, err := readZipEntry(data, "word/document.xml")
	if err != nil {
		return nil, err
	}
	text, err := collectText(doc, "t", "p")
	if err != nil {
		return nil, err
	}
	return []PageText{{Page: 1, Text: text}}, nil
}

// ExtractXLSXText extracts cell text from xl/sharedStrings.xml. Numbers and
// inline strings are not covered — good enough for text search over
// spreadsheet labels/notes.
func ExtractXLSXText(data []byte) ([]PageText, error) {
	doc, err := readZipEntry(data, "xl/sharedStrings.xml")
	if err != nil {
		return nil, err
	}
	text, err := collectText(doc, "t", "si")
	if err != nil {
		return nil, err
	}
	return []PageText{{Page: 1, Text: text}}, nil
}
