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

// splitParents splits by blank lines into paragraphs. Each non-empty paragraph
// becomes one parent. The target parameter is reserved for future merging
// logic; current semantics: paragraphs stay independent so that short
// paragraphs yield separate chunks (matching TDD expectations), while a long
// paragraph stays a single parent whose children share the same context.
func splitParents(text string, target int) []string {
	_ = target // reserved for future parent-merging heuristic
	rawParas := strings.Split(text, "\n\n")
	var parents []string
	for _, p := range rawParas {
		p = strings.TrimSpace(p)
		if p != "" {
			parents = append(parents, p)
		}
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