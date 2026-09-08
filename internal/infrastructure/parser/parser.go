package parser

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
	"golang.org/x/net/html"
)

// Parse dispatches by mimeType then file extension. Returns extracted plain text.
func Parse(body []byte, filename, mimeType string) (string, error) {
	ext := extOf(filename)
	switch {
	case isMarkdown(mimeType, ext):
		return parseMarkdown(body), nil
	case isText(mimeType, ext):
		return string(body), nil
	case isHTML(mimeType, ext):
		return parseHTML(body), nil
	case ext == ".pdf":
		return parsePDF(body)
	case ext == ".docx":
		return parseDocx(body)
	// NOTE: XLSX/PPTX branches are added in later tasks (Task 6-7)
	// when the corresponding parseXxx functions are implemented.
	default:
		return "", fmt.Errorf("unsupported file type: name=%s mime=%s", filename, mimeType)
	}
}

func extOf(name string) string {
	i := strings.LastIndex(name, ".")
	if i < 0 {
		return ""
	}
	return strings.ToLower(name[i:])
}

func isMarkdown(mime, ext string) bool {
	return mime == "text/markdown" || ext == ".md" || ext == ".markdown"
}

func isText(mime, ext string) bool {
	if mime == "text/plain" || ext == ".txt" {
		return true
	}
	return false
}

func isHTML(mime, ext string) bool {
	return mime == "text/html" || ext == ".html" || ext == ".htm"
}

func parseMarkdown(body []byte) string {
	// Strip common markdown markers; keep text. Good enough for v1.
	s := string(body)
	// Remove code fences
	s = strings.ReplaceAll(s, "```", "")
	// Strip heading hashes, list markers, emphasis
	lines := strings.Split(s, "\n")
	var out []string
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		ln = strings.TrimLeft(ln, "#")
		ln = strings.TrimSpace(ln)
		ln = strings.TrimPrefix(ln, "- ")
		ln = strings.TrimPrefix(ln, "* ")
		// strip bold/italic markers
		ln = strings.ReplaceAll(ln, "**", "")
		ln = strings.ReplaceAll(ln, "__", "")
		ln = strings.ReplaceAll(ln, "_", "")
		ln = strings.ReplaceAll(ln, "*", "")
		if ln != "" {
			out = append(out, ln)
		}
	}
	return strings.Join(out, "\n")
}

func parseHTML(body []byte) string {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return string(body) // fallback
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		if n.Type == html.TextNode {
			t := strings.TrimSpace(n.Data)
			if t != "" {
				if b.Len() > 0 {
					b.WriteString(" ")
				}
				b.WriteString(t)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return b.String()
}

func parsePDF(body []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return "", fmt.Errorf("pdf reader: %w", err)
	}
	var b strings.Builder
	n := r.NumPage()
	for i := 1; i <= n; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		b.WriteString(text)
		b.WriteString("\n")
	}
	return b.String(), nil
}

// parseDocx extracts plain text from a .docx (Office Open XML WordprocessingML)
// package using only the standard library. A .docx is a zip archive whose main
// content lives at word/document.xml; paragraphs are <w:p> and text runs hold
// <w:t> elements. We concatenate <w:t> text and separate paragraphs with "\n".
func parseDocx(body []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return "", fmt.Errorf("docx open zip: %w", err)
	}
	var docXML []byte
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			docXML, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", err
			}
			break
		}
	}
	if docXML == nil {
		return "", fmt.Errorf("docx: word/document.xml not found")
	}
	return extractDocxText(docXML), nil
}

// extractDocxText walks the document.xml token stream, appending text from
// <w:t> elements and emitting a newline at each <w:p> boundary.
func extractDocxText(data []byte) string {
	const wNS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	dec := xml.NewDecoder(bytes.NewReader(data))
	var b strings.Builder
	inT := false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" && t.Name.Space == wNS {
				inT = true
			} else if t.Name.Local == "p" && t.Name.Space == wNS {
				if b.Len() > 0 {
					b.WriteString("\n")
				}
			}
		case xml.CharData:
			if inT {
				b.Write(t)
			}
		case xml.EndElement:
			if t.Name.Local == "t" && t.Name.Space == wNS {
				inT = false
			}
		}
	}
	return b.String()
}