package parser

import (
	"bytes"
	"fmt"
	"strings"

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
	// NOTE: PDF/DOCX/XLSX/PPTX branches are added in later tasks (Task 4-7)
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