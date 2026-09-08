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