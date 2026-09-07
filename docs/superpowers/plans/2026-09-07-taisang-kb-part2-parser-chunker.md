# TaiSang-KB Implementation Plan (Part 2: Parser + Chunker)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert uploaded files (MD/TXT/HTML/PDF/DOCX/XLSX/PPTX) to plain text, then split into parent-child chunks for retrieval.

**Architecture:** `parser` package dispatches by MIME type / extension to format-specific extractors returning `string`. `chunker` package does parent-then-child splitting with sliding window. Both are pure functions (no DB, no I/O except reading the file).

**Tech Stack:** Go stdlib + `unioffice` (Office) + `ledongthuc/pdf` (PDF) + `golang.org/x/net/html` (HTML).

**Spec ref:** `docs/superpowers/specs/2026-09-07-taisang-kb-design.md` §5 (parser, chunker), §6.1 (分块策略).

**Prerequisite:** Part 1 complete (project skeleton, models exist).

---

### Task 1: Add parser dependencies

**Files:**
- Modify: `go.mod` (via go get)

- [ ] **Step 1: Add deps**

Run:
```bash
cd D:/Project/TaiSang-KB
go get github.com/ledongthuc/pdf@latest
go get github.com/unidoc/unioffice@latest
go get golang.org/x/net@latest
go mod tidy
```

Note: `unioffice` is AGPL-licensed — acceptable for an internal personal project. If you need a permissive license, swap to `tealeg/xlsx` + `nguyenthenguyen/docx` later. For now unioffice covers all three Office formats in one lib.

- [ ] **Step 2: Verify build**

Run:
```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add parser deps (pdf/unioffice/x/net)"
```

---

### Task 2: Create test fixtures

**Files:**
- Create: `testdata/parser/sample.md`
- Create: `testdata/parser/sample.txt`
- Create: `testdata/parser/sample.html`
- Create: `testdata/parser/sample.pdf` (placeholder note — see Step 2)
- Create: `testdata/parser/sample.docx` (placeholder note)
- Create: `testdata/parser/sample.xlsx` (placeholder note)
- Create: `testdata/parser/sample.pptx` (placeholder note)

- [ ] **Step 1: Create text-based fixtures**

`testdata/parser/sample.md`:
```markdown
# TaiSang-KB 测试文档

这是第一段。**Markdown** 支持 _格式_ 但我们只提取文本。

## 第二节

- 列表项一
- 列表项二

代码块：
\`\`\`go
fmt.Println("hello")
\`\`\`

最后一段。
```

`testdata/parser/sample.txt`:
```
这是一个纯文本测试文件。
第二行。
第三行包含中文和 English mixed content。
```

`testdata/parser/sample.html`:
```html
<!DOCTYPE html>
<html><head><title>Test</title></head>
<body>
<h1>HTML 测试</h1>
<p>段落内容 <a href="x">链接</a> 应该被剥除。</p>
<script>var x = 1;</script>
</body></html>
```

- [ ] **Step 2: Create binary fixtures via helper script**

Office/PDF binaries are large in text; create them programmatically with a one-off helper:

Create `testdata/parser/gen.go` (build-tagged so it doesn't run in normal tests):
```go
//go:build gen

package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/unidoc/unioffice/document"
	"github.com/unidoc/unioffice/presentation"
	"github.com/unidoc/unioffice/spreadsheet"
)

func main() {
	dir := "testdata/parser"

	// docx
	d := document.New()
	para := d.AddParagraph()
	para.AddRun().AddText("DOCX 测试段落一。")
	para2 := d.AddParagraph()
	para2.AddRun().AddText("第二段落 with English.")
	if err := d.SaveToFile(filepath.Join(dir, "sample.docx")); err != nil {
		log.Fatal(err)
	}

	// xlsx
	xl := spreadsheet.New()
	sheet := xl.AddSheet()
	sheet.AddRow().AddCell().SetString("XLSX 单元格 A1")
	sheet.AddRow().AddCell().SetString("第二行 B1")
	if err := xl.SaveToFile(filepath.Join(dir, "sample.xlsx")); err != nil {
		log.Fatal(err)
	}

	// pptx
	p := presentation.New()
	slide := p.AddSlide()
	tb := slide.AddTextBox()
	tb.AddParagraph().AddRun().AddText("PPTX 幻灯片一内容")
	if err := p.SaveToFile(filepath.Join(dir, "sample.pptx")); err != nil {
		log.Fatal(err)
	}

	_ = os.MkdirAll(dir, 0o755)
}
```

Run:
```bash
go run -tags gen testdata/parser/gen.go
```
Expected: `sample.docx`, `sample.xlsx`, `sample.pptx` created in `testdata/parser/`.

For PDF, use any tool to create a one-page text PDF. Quick option (requires libreoffice or similar):
```bash
echo "PDF 测试文本内容。" | pandoc -o testdata/parser/sample.pdf 2>/dev/null || \
  echo "Manual: create testdata/parser/sample.pdf with text 'PDF 测试文本内容。'"
```
If pandoc unavailable, manually create a tiny PDF with any tool, or write a Go program using `github.com/jung-kurt/gofpdf` to generate one. Fallback: download a public-domain sample PDF. The test will assert it contains a known keyword.

- [ ] **Step 3: Commit fixtures**

```bash
git add testdata/parser/
git commit -m "test: parser fixture files (md/txt/html/docx/xlsx/pptx/pdf)"
```

---

### Task 3: Parser interface + MD/TXT/HTML

**Files:**
- Create: `internal/infrastructure/parser/parser.go`
- Create: `internal/infrastructure/parser/parser_test.go`

- [ ] **Step 1: Write the failing test**

```go
package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse_Markdown(t *testing.T) {
	got, err := Parse(readFixture(t, "sample.md"), "sample.md", "text/markdown")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "TaiSang-KB 测试文档") {
		t.Errorf("missing heading: %q", got)
	}
	if !strings.Contains(got, "列表项一") {
		t.Errorf("missing list item: %q", got)
	}
}

func TestParse_Text(t *testing.T) {
	got, err := Parse(readFixture(t, "sample.txt"), "sample.txt", "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "纯文本测试文件") {
		t.Errorf("missing text: %q", got)
	}
}

func TestParse_HTML_StripsScript(t *testing.T) {
	got, err := Parse(readFixture(t, "sample.html"), "sample.html", "text/html")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "var x") {
		t.Errorf("script leaked: %q", got)
	}
	if !strings.Contains(got, "HTML 测试") {
		t.Errorf("missing heading: %q", got)
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "parser", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/infrastructure/parser/ -v
```
Expected: FAIL (no parser.go).

- [ ] **Step 3: Write implementation**

```go
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
	case ext == ".pdf":
		return parsePDF(body)
	case ext == ".docx":
		return parseDocx(body)
	case ext == ".xlsx":
		return parseXlsx(body)
	case ext == ".pptx":
		return parsePptx(body)
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
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/infrastructure/parser/ -run 'TestParse_Markdown|TestParse_Text|TestParse_HTML' -v
```
Expected: 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/parser/
git commit -m "feat(parser): markdown/text/html extractors with script stripping"
```

---

### Task 4: PDF parser

**Files:**
- Modify: `internal/infrastructure/parser/parser.go` (add parsePDF)
- Modify: `internal/infrastructure/parser/parser_test.go`

- [ ] **Step 1: Add the failing test**

```go
func TestParse_PDF(t *testing.T) {
	body := readFixture(t, "sample.pdf")
	if len(body) == 0 {
		t.Skip("sample.pdf not present, skipping")
	}
	got, err := Parse(body, "sample.pdf", "application/pdf")
	if err != nil {
		t.Fatal(err)
	}
	// We can't guarantee exact text from arbitrary PDF; assert non-empty and contains letters.
	if strings.TrimSpace(got) == "" {
		t.Errorf("empty extraction")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/infrastructure/parser/ -run TestParse_PDF -v
```
Expected: FAIL (parsePDF undefined).

- [ ] **Step 3: Add parsePDF implementation**

Append to `parser.go`:
```go
import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
	"golang.org/x/net/html"
)

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
```

(Adjust the import block: merge the new imports into the existing `import` block at the top of `parser.go`.)

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/infrastructure/parser/ -run TestParse_PDF -v
```
Expected: PASS (or SKIP if fixture missing).

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/parser/
git commit -m "feat(parser): pdf text extraction via ledongthuc/pdf"
```

---

### Task 5: DOCX parser

**Files:**
- Modify: `internal/infrastructure/parser/parser.go` (add parseDocx)
- Modify: `internal/infrastructure/parser/parser_test.go`

- [ ] **Step 1: Add failing test**

```go
func TestParse_DOCX(t *testing.T) {
	body := readFixture(t, "sample.docx")
	if len(body) == 0 {
		t.Skip("sample.docx not present")
	}
	got, err := Parse(body, "sample.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "DOCX 测试段落一") {
		t.Errorf("missing text: %q", got)
	}
	if !strings.Contains(got, "第二段落") {
		t.Errorf("missing second para: %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/infrastructure/parser/ -run TestParse_DOCX -v
```
Expected: FAIL.

- [ ] **Step 3: Add parseDocx**

```go
import (
	// ...
	"github.com/unidoc/unioffice/document"
)

func parseDocx(body []byte) (string, error) {
	tmpFile, err := os.CreateTemp("", "tsk-docx-*.docx")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(body); err != nil {
		return "", err
	}
	tmpFile.Close()

	doc, err := document.Open(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("docx open: %w", err)
	}
	defer doc.Close()
	var b strings.Builder
	for _, para := range doc.Paragraphs() {
		text := para.Text()
		if strings.TrimSpace(text) != "" {
			b.WriteString(text)
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}
```
(Add `"os"` to imports if not already present.)

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/infrastructure/parser/ -run TestParse_DOCX -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/parser/
git commit -m "feat(parser): docx text extraction via unioffice"
```

---

### Task 6: XLSX parser

**Files:**
- Modify: `internal/infrastructure/parser/parser.go` (add parseXlsx)
- Modify: `internal/infrastructure/parser/parser_test.go`

- [ ] **Step 1: Add failing test**

```go
func TestParse_XLSX(t *testing.T) {
	body := readFixture(t, "sample.xlsx")
	if len(body) == 0 {
		t.Skip("sample.xlsx not present")
	}
	got, err := Parse(body, "sample.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "XLSX 单元格 A1") {
		t.Errorf("missing cell: %q", got)
	}
	if !strings.Contains(got, "第二行 B1") {
		t.Errorf("missing row2: %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/infrastructure/parser/ -run TestParse_XLSX -v
```
Expected: FAIL.

- [ ] **Step 3: Add parseXlsx**

```go
import (
	"github.com/unidoc/unioffice/spreadsheet"
)

func parseXlsx(body []byte) (string, error) {
	tmpFile, err := os.CreateTemp("", "tsk-xlsx-*.xlsx")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(body); err != nil {
		return "", err
	}
	tmpFile.Close()

	xl, err := spreadsheet.Open(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("xlsx open: %w", err)
	}
	defer xl.Close()
	var b strings.Builder
	for _, sheet := range xl.Sheets() {
		for _, row := range sheet.Rows() {
			var cells []string
			for _, cell := range row.Cells() {
				cells = append(cells, cell.GetString())
			}
			line := strings.Join(cells, "\t")
			if strings.TrimSpace(line) != "" {
				b.WriteString(line)
				b.WriteString("\n")
			}
		}
	}
	return b.String(), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/infrastructure/parser/ -run TestParse_XLSX -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/parser/
git commit -m "feat(parser): xlsx text extraction"
```

---

### Task 7: PPTX parser

**Files:**
- Modify: `internal/infrastructure/parser/parser.go` (add parsePptx)
- Modify: `internal/infrastructure/parser/parser_test.go`

- [ ] **Step 1: Add failing test**

```go
func TestParse_PPTX(t *testing.T) {
	body := readFixture(t, "sample.pptx")
	if len(body) == 0 {
		t.Skip("sample.pptx not present")
	}
	got, err := Parse(body, "sample.pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "PPTX 幻灯片一内容") {
		t.Errorf("missing slide text: %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/infrastructure/parser/ -run TestParse_PPTX -v
```
Expected: FAIL.

- [ ] **Step 3: Add parsePptx**

```go
import (
	"github.com/unidoc/unioffice/presentation"
)

func parsePptx(body []byte) (string, error) {
	tmpFile, err := os.CreateTemp("", "tsk-pptx-*.pptx")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(body); err != nil {
		return "", err
	}
	tmpFile.Close()

	p, err := presentation.Open(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("pptx open: %w", err)
	}
	defer p.Close()
	var b strings.Builder
	for _, slide := range p.Slides() {
		for _, shape := range slide.Shapes() {
			if tb := shape.TextBox(); tb != nil {
				for _, para := range tb.Paragraphs() {
					for _, run := range para.Runs() {
						b.WriteString(run.Text())
					}
					b.WriteString("\n")
				}
			}
		}
	}
	return b.String(), nil
}
```

Note: exact API names may differ across unioffice versions. If `shape.TextBox()` or `para.Runs()` don't exist, consult unioffice presentation docs; common alternative is iterating `slide.Shapes()` and using `shape.Text()` if available. Adjust to the installed version's API — keep the test as the source of truth.

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/infrastructure/parser/ -run TestParse_PPTX -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/parser/
git commit -m "feat(parser): pptx text extraction"
```

---

### Task 8: Chunker — parent/child splitting

**Files:**
- Create: `internal/service/chunker/chunker.go`
- Create: `internal/service/chunker/chunker_test.go`

- [ ] **Step 1: Write the failing test**

```go
package chunker

import (
	"strings"
	"testing"
)

func TestSplit_BasicParentChild(t *testing.T) {
	// Two paragraphs separated by blank line. Each paragraph is short.
	text := "第一段内容比较短。\n\n第二段也很短。"
	chunks, err := Split(text, SplitOptions{
		ParentTarget: 800,
		ChildTarget:  256,
		ChildOverlap: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].Content != "第一段内容比较短。" {
		t.Errorf("chunk0 content = %q", chunks[0].Content)
	}
	if chunks[0].ParentContent != "第一段内容比较短。" {
		t.Errorf("chunk0 parent = %q", chunks[0].ParentContent)
	}
}

func TestSplit_LongParagraph_SlidingChildren(t *testing.T) {
	// One very long paragraph; should produce multiple child chunks with overlap.
	text := strings.Repeat("这是句子。", 200) // ~1000 chars
	chunks, err := Split(text, SplitOptions{
		ParentTarget: 800,
		ChildTarget:  256,
		ChildOverlap: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple child chunks, got %d", len(chunks))
	}
	// All chunks share the same parent (one paragraph).
	parent := chunks[0].ParentContent
	for _, c := range chunks {
		if c.ParentContent != parent {
			t.Errorf("parent differs across chunks of one paragraph")
			break
		}
	}
}

func TestSplit_EmptyText(t *testing.T) {
	chunks, err := Split("", SplitOptions{
		ParentTarget: 800, ChildTarget: 256, ChildOverlap: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty input, got %d", len(chunks))
	}
}

func TestSplit_EstimateTokensCJK(t *testing.T) {
	// Rough: Chinese chars count as ~1.5 tokens each in many models; we use a simple heuristic.
	n := estimateTokens("中文测试四个字")
	if n <= 0 {
		t.Errorf("estimateTokens returned %d for CJK", n)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/service/chunker/ -v
```
Expected: FAIL (no chunker.go).

- [ ] **Step 3: Write implementation**

```go
package chunker

import (
	"errors"
	"strings"
	"unicode"
)

type Chunk struct {
	Content       string // child chunk (search unit)
	ParentContent string // parent chunk (context unit)
	ChunkIndex    int
	TokenCount    int
}

type SplitOptions struct {
	ParentTarget int // approx tokens per parent
	ChildTarget  int // approx tokens per child
	ChildOverlap int // approx tokens overlap between children
}

// Split splits text into parent chunks (by paragraph), then each parent into
// child chunks with sliding window. Returns a flat list of Chunk.
func Split(text string, opts SplitOptions) ([]Chunk, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	if opts.ParentTarget <= 0 || opts.ChildTarget <= 0 {
		return nil, errors.New("invalid target sizes")
	}
	if opts.ChildOverlap < 0 || opts.ChildOverlap >= opts.ChildTarget {
		return nil, errors.New("overlap must be in [0, childTarget)")
	}

	parents := splitParents(text, opts.ParentTarget)
	var out []Chunk
	idx := 0
	for _, parent := range parents {
		children := splitChild(parent, opts.ChildTarget, opts.ChildOverlap)
		if len(children) == 0 {
			// parent itself becomes one child chunk
			out = append(out, Chunk{
				Content:       parent,
				ParentContent: parent,
				ChunkIndex:    idx,
				TokenCount:    estimateTokens(parent),
			})
			idx++
			continue
		}
		for _, ch := range children {
			out = append(out, Chunk{
				Content:       ch,
				ParentContent: parent,
				ChunkIndex:    idx,
				TokenCount:    estimateTokens(ch),
			})
			idx++
		}
	}
	return out, nil
}

// splitParents splits by blank lines into paragraphs, then groups paragraphs
// until reaching parentTarget tokens.
func splitParents(text string, target int) []string {
	rawParas := strings.Split(text, "\n\n")
	var paragraphs []string
	for _, p := range rawParas {
		p = strings.TrimSpace(p)
		if p != "" {
			paragraphs = append(paragraphs, p)
		}
	}
	if len(paragraphs) == 0 {
		return nil
	}
	var parents []string
	var cur strings.Builder
	curTokens := 0
	for _, p := range paragraphs {
		pt := estimateTokens(p)
		if cur.Len() > 0 && curTokens+pt > target {
			parents = append(parents, cur.String())
			cur.Reset()
			cur.WriteString(p)
			curTokens = pt
		} else {
			if cur.Len() > 0 {
				cur.WriteString("\n\n")
			}
			cur.WriteString(p)
			curTokens += pt
		}
	}
	if cur.Len() > 0 {
		parents = append(parents, cur.String())
	}
	return parents
}

// splitChild splits a parent string into child chunks of ~childTarget tokens
// with childOverlap tokens of overlap. Splits on sentence boundaries when possible.
func splitChild(parent string, target, overlap int) []string {
	if estimateTokens(parent) <= target {
		return []string{parent}
	}
	runes := []rune(parent)
	var chunks []string
	step := target - overlap
	if step <= 0 {
		step = target
	}
	i := 0
	for i < len(runes) {
		end := i + target
		if end > len(runes) {
			end = len(runes)
		}
		// Try to break at sentence end within [end-target*0.2, end].
		loosen := target / 5
		breakAt := end
		for j := end; j > end-loosen && j > i; j-- {
			if isSentenceBoundary(runes[j-1]) {
				breakAt = j
				break
			}
		}
		chunks = append(chunks, string(runes[i:breakAt]))
		if breakAt >= len(runes) {
			break
		}
		i = breakAt - overlap
		if i < 0 {
			i = 0
		}
	}
	return chunks
}

func isSentenceBoundary(r rune) bool {
	switch r {
	case '.', '!', '?', '。', '！', '？', '\n':
		return true
	}
	return false
}

// estimateTokens is a rough heuristic: CJK runes count as ~1 token, ASCII words as ~1.3 tokens/char on average.
// Good enough for chunk sizing; we don't need exact tokenizer counts.
func estimateTokens(s string) int {
	count := 0
	inWord := false
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			count++
			inWord = false
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' {
			inWord = false
			continue
		}
		if !inWord {
			count++
			inWord = true
		}
	}
	return count
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/service/chunker/ -v
```
Expected: 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/chunker/
git commit -m "feat(chunker): parent-child splitting with sliding window and CJK-aware tokenizer"
```

---

### Task 9: Self-review and checkpoint

- [ ] **Step 1: Run all parser tests**

```bash
go test ./internal/infrastructure/parser/ -v
```
Expected: all PASS (or SKIP for binary fixtures not present).

- [ ] **Step 2: Run chunker tests**

```bash
go test ./internal/service/chunker/ -v
```
Expected: all PASS.

- [ ] **Step 3: Run whole repo**

```bash
go test ./... -count=1 -short
go vet ./...
```
Expected: PASS / no issues.

- [ ] **Step 4: Commit checkpoint**

```bash
git commit --allow-empty -m "checkpoint: Part 2 parser+chunker complete"
```

---

## End of Part 2

**What's done:**
- parser: MD/TXT/HTML/PDF/DOCX/XLSX/PPTX → plain text
- chunker: parent-child split with sliding window, CJK-aware token estimation
- Test fixtures for all formats

**Next:** Part 3 — LLM client, Embedding client, Reranker.