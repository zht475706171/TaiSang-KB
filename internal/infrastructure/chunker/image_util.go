package chunker

import "regexp"

// unwrapLinkedImages is inlined from WeKnora docparser.UnwrapLinkedImages
// to avoid pulling in the entire docparser package. It unwraps markdown
// linked images (![alt](url)) into plain images.
var linkedImageRe = regexp.MustCompile(`!\[(.*?)\]\(([^()\s]*(?:\([^)]*\)[^()\s]*)*)\)`)

func unwrapLinkedImages(markdown string) string {
	return linkedImageRe.ReplaceAllString(markdown, "![$1]($2)")
}