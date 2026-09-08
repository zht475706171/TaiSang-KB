// Package chunker is a thin adapter over internal/infrastructure/chunker
// (ported from WeKnora) that preserves TaiSang-KB's original public API:
//
//	Split(text, SplitOptions) ([]Chunk, error)
//
// Mapping rule:
//   - child.Content        -> Chunk.Content
//   - parent.Content + (newline + child.ContextHeader if non-empty)
//                         -> Chunk.ParentContent
//   - sequential 0..N-1   -> Chunk.ChunkIndex
//   - ApproxTokenCount    -> Chunk.TokenCount
package chunker

import (
	"errors"
	"strings"

	infrac "github.com/zht475706171/TaiSang-KB/internal/infrastructure/chunker"
)

// Chunk is the TaiSang-KB service-layer chunk shape. Kept identical to the
// previous version so worker.parse_worker.go and router.go do not change.
type Chunk struct {
	Content       string // child chunk (search unit)
	ParentContent string // parent chunk (context unit, includes ContextHeader)
	ChunkIndex    int
	TokenCount    int
}

// SplitOptions is the TaiSang-KB service-layer chunking config.
// Values are interpreted as approximate token budgets and forwarded to
// WeKnora's token-aware SplitterConfig via CharsForTokenLimit.
type SplitOptions struct {
	ParentTarget int // approx tokens per parent
	ChildTarget  int // approx tokens per child
	ChildOverlap int // approx tokens overlap between children
}

// Split splits text into parent chunks, then each parent into child chunks,
// and returns a flat list of Chunk. The adapter always uses WeKnora's
// strategy-aware SplitParentChild entrypoint with StrategyAuto so the
// heading / heuristic / legacy tiers are picked automatically per document.
func Split(text string, opts SplitOptions) ([]Chunk, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	if opts.ParentTarget <= 0 || opts.ChildTarget <= 0 {
		return nil, errors.New("invalid target sizes")
	}
	if opts.ChildOverlap < 0 {
		return nil, errors.New("overlap must be >= 0")
	}

	// Derive WeKnora SplitterConfigs from the TaiSang-KB token-budget opts.
	// WeKnora's DeriveParentChildConfigs takes character sizes; convert
	// tokens to chars via CharsForTokenLimit (LangMixed ratio 3.0, 0.9
	// safety factor). Language auto-detection happens inside WeKnora.
	parentChars := infrac.CharsForTokenLimit(opts.ParentTarget, infrac.LangMixed)
	childChars := infrac.CharsForTokenLimit(opts.ChildTarget, infrac.LangMixed)
	overlapChars := infrac.CharsForTokenLimit(opts.ChildOverlap, infrac.LangMixed)
	if parentChars <= 0 {
		parentChars = infrac.DefaultChunkSize
	}
	if childChars <= 0 {
		childChars = 384
	}

	base := infrac.SplitterConfig{
		Strategy:   infrac.StrategyAuto,
		Separators: []string{"\n\n", "\n", "。"},
	}
	parentCfg, childCfg := infrac.DeriveParentChildConfigs(base, parentChars, childChars)
	// DeriveParentChildConfigs sets child overlap to childSize/5; honor the
	// caller ChildOverlap explicitly (converted to chars).
	if overlapChars > 0 {
		childCfg.ChunkOverlap = overlapChars
	}

	result := infrac.SplitParentChild(text, parentCfg, childCfg)
	return mapParentChildToChunks(result), nil
}

// mapParentChildToChunks flattens WeKnora ParentChildResult into the
// TaiSang-KB []Chunk shape. Children whose ParentIndex is -1 (parent
// produced only one child equal to itself) get ParentContent = Content.
func mapParentChildToChunks(result infrac.ParentChildResult) []Chunk {
	if len(result.Children) == 0 {
		return nil
	}
	out := make([]Chunk, 0, len(result.Children))
	for i, child := range result.Children {
		parentContent := child.Content
		if child.ParentIndex >= 0 && child.ParentIndex < len(result.Parents) {
			parentContent = result.Parents[child.ParentIndex].Content
		}
		// Fold the child ContextHeader (Markdown heading breadcrumb) into
		// ParentContent so the existing DB column carries both the parent
		// text and the heading context. No schema change required.
		if child.ContextHeader != "" {
			if parentContent == "" {
				parentContent = child.ContextHeader
			} else {
				parentContent = parentContent + "\n\n" + child.ContextHeader
			}
		}
		lang := infrac.DetectLanguage(child.Content)
		out = append(out, Chunk{
			Content:       child.Content,
			ParentContent: parentContent,
			ChunkIndex:    i,
			TokenCount:    infrac.ApproxTokenCount(child.Content, lang),
		})
	}
	return out
}