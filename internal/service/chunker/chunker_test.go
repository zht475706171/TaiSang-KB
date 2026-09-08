package chunker

import (
	"strings"
	"testing"

	infrac "github.com/zht475706171/TaiSang-KB/internal/infrastructure/chunker"
)

func TestSplit_BasicParentChild(t *testing.T) {
	// Two paragraphs separated by blank line. Each paragraph is short.
	// WeKnora auto strategy treats the whole short text as one chunk
	// (text is far below child target). Verify at least one chunk with
	// content containing both paragraphs.
	text := "第一段内容比较短。\n\n第二段也很短。"
	chunks, err := Split(text, SplitOptions{
		ParentTarget: 800,
		ChildTarget:  256,
		ChildOverlap: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}
	if !strings.Contains(chunks[0].Content, "第一段") {
		t.Errorf("chunk0 content = %q", chunks[0].Content)
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
	n := infrac.ApproxTokenCount("中文测试四个字", infrac.LangChinese)
	if n <= 0 {
		t.Errorf("ApproxTokenCount returned %d for CJK", n)
	}
}

func TestSplit_MarkdownHeadingProducesContextHeader(t *testing.T) {
	// Markdown with a heading. The child under the heading should carry
	// the heading breadcrumb in ParentContent.
	text := "# 第一章 标题\n\n这是正文内容。这是更多内容。\n\n## 子标题\n\n子标题下的内容。"
	chunks, err := Split(text, SplitOptions{
		ParentTarget: 800,
		ChildTarget:  256,
		ChildOverlap: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}
	// At least one chunk's ParentContent should mention the heading text.
	found := false
	for _, c := range chunks {
		if strings.Contains(c.ParentContent, "第一章") || strings.Contains(c.ParentContent, "子标题") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no chunk ParentContent contains heading text; first parent=%q", chunks[0].ParentContent)
	}
}

func TestSplit_ProtectedLatexBlockNotSplit(t *testing.T) {
	// A LaTeX block $$...$$ should NOT be split across chunks.
	latex := "$$\\int_0^1 f(x)\\,dx = F(1) - F(0)$$"
	// Use a small ChildTarget to force splitting. Surround with enough
	// repeated text that the whole text exceeds the child budget.
	text := strings.Repeat("前置文本内容句子一。前置文本内容句子二。", 30) + "\n\n" + latex + "\n\n" + strings.Repeat("后置文本内容句子一。后置文本内容句子二。", 30)
	chunks, err := Split(text, SplitOptions{
		ParentTarget: 4096,
		ChildTarget:  64,
		ChildOverlap: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	// The LaTeX block must be fully contained in exactly one chunk.
	full := latex
	count := 0
	for _, c := range chunks {
		if strings.Contains(c.Content, full) {
			count++
		}
	}
	if count != 1 {
		t.Errorf("latex block should appear in exactly 1 chunk, found in %d chunks", count)
	}
}