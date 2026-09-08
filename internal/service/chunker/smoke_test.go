package chunker_test

import (
	"strings"
	"testing"

	"github.com/zht475706171/TaiSang-KB/internal/service/chunker"
)

func TestSmoke_HeadingLatexMarkdown(t *testing.T) {
	text := `# 第一章 RAG 概述

RAG 是检索增强生成的缩写。它通过检索外部知识来增强 LLM 的回答。这种方法能有效减少幻觉，提升回答的可信度。

## 数学基础

一个简单的积分公式：

$$\int_0^1 f(x)\,dx = F(1) - F(0)$$

其中 $F$ 是 $f$ 的原函数。这个公式是微积分基本定理的特例。

## 应用场景

RAG 适用于知识密集型问答、企业文档问答、个人知识库等场景。在实际工程中，RAG 系统的质量取决于切片策略、嵌入模型、检索算法等多个因素。`
	chunks, err := chunker.Split(text, chunker.SplitOptions{
		ParentTarget: 4096,
		ChildTarget:  384,
		ChildOverlap: 76,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("got %d chunks", len(chunks))
	for i, c := range chunks {
		t.Logf("chunk[%d] content_len=%d parent_len=%d token=%d", i, len(c.Content), len(c.ParentContent), c.TokenCount)
		t.Logf("  content head: %q", truncate(c.Content, 80))
		t.Logf("  parent head:  %q", truncate(c.ParentContent, 120))
	}

	// Assertion 1: ContextHeader folded into ParentContent.
	headingFound := false
	for _, c := range chunks {
		if strings.Contains(c.ParentContent, "第一章") || strings.Contains(c.ParentContent, "数学基础") || strings.Contains(c.ParentContent, "应用场景") {
			headingFound = true
			break
		}
	}
	if !headingFound {
		t.Errorf("no chunk ParentContent contains any markdown heading text")
	}

	// Assertion 2: LaTeX block intact in some chunk content.
	latex := "$$" + `\int_0^1 f(x)\,dx = F(1) - F(0)` + "$$"
	latexFound := false
	for _, c := range chunks {
		if strings.Contains(c.Content, latex) {
			latexFound = true
			break
		}
	}
	if !latexFound {
		t.Errorf("latex block not found intact in any chunk content")
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
