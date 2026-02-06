package guardrails

import (
	"path"
	"strings"
)

type commentState struct {
	inBlockComment bool
	inHTMLComment  bool
}

// CountLOC returns non-blank, non-comment-only lines.
func CountLOC(filePath string, content []byte) int {
	ext := strings.ToLower(path.Ext(filePath))
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	state := commentState{}

	count := 0
	for _, line := range lines {
		if lineHasCode(strings.TrimSpace(line), ext, &state) {
			count++
		}
	}

	return count
}

func lineHasCode(line string, ext string, state *commentState) bool {
	remaining := line
	for remaining != "" {
		var consumed bool
		remaining, consumed = consumeActiveComment(remaining, state)
		if consumed {
			continue
		}

		idx, kind := findCommentStart(remaining, ext)
		if idx < 0 {
			return true
		}

		before := strings.TrimSpace(remaining[:idx])
		if before != "" {
			return true
		}

		remaining = consumeComment(remaining, idx, kind, state)
	}
	return false
}

func consumeActiveComment(remaining string, state *commentState) (string, bool) {
	if state.inBlockComment {
		return consumeUntilToken(remaining, "*/", &state.inBlockComment), true
	}
	if state.inHTMLComment {
		return consumeUntilToken(remaining, "-->", &state.inHTMLComment), true
	}
	return remaining, false
}

func consumeUntilToken(remaining string, token string, active *bool) string {
	end := strings.Index(remaining, token)
	if end < 0 {
		return ""
	}
	*active = false
	return strings.TrimSpace(remaining[end+len(token):])
}

func findCommentStart(remaining string, ext string) (int, string) {
	idxLine := strings.Index(remaining, "//")
	idxBlock := strings.Index(remaining, "/*")
	idxHTML := -1
	if ext == ".svelte" {
		idxHTML = strings.Index(remaining, "<!--")
	}
	return firstCommentMarker(idxLine, idxBlock, idxHTML)
}

func consumeComment(remaining string, idx int, kind string, state *commentState) string {
	switch kind {
	case "line":
		return ""
	case "block":
		return consumeBlockComment(remaining[idx+2:], state)
	case "html":
		return consumeHTMLComment(remaining[idx+4:], state)
	default:
		return ""
	}
}

func consumeBlockComment(after string, state *commentState) string {
	end := strings.Index(after, "*/")
	if end < 0 {
		state.inBlockComment = true
		return ""
	}
	return strings.TrimSpace(after[end+2:])
}

func consumeHTMLComment(after string, state *commentState) string {
	end := strings.Index(after, "-->")
	if end < 0 {
		state.inHTMLComment = true
		return ""
	}
	return strings.TrimSpace(after[end+3:])
}

func firstCommentMarker(idxLine int, idxBlock int, idxHTML int) (int, string) {
	idx := -1
	kind := ""

	set := func(next int, nextKind string) {
		if next < 0 {
			return
		}
		if idx < 0 || next < idx {
			idx = next
			kind = nextKind
		}
	}

	set(idxLine, "line")
	set(idxBlock, "block")
	set(idxHTML, "html")
	return idx, kind
}
