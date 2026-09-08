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