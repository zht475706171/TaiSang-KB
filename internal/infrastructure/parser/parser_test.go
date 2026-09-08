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
		// Missing fixtures are not fatal here; callers that want to skip on
		// absent fixtures (e.g. PDF/DOCX/XLSX/PPTX) check len(body) == 0.
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	return b
}

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