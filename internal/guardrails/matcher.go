package guardrails

import (
	"fmt"
	"regexp"
	"strings"
)

type matcher struct {
	pattern string
	regex   *regexp.Regexp
}

func compilePatterns(patterns []string) ([]matcher, error) {
	matchers := make([]matcher, 0, len(patterns))
	for _, raw := range patterns {
		pattern := strings.TrimSpace(raw)
		if pattern == "" {
			continue
		}

		rx, err := regexp.Compile(globToRegex(pattern))
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		matchers = append(matchers, matcher{
			pattern: pattern,
			regex:   rx,
		})
	}
	return matchers, nil
}

func anyMatch(matchers []matcher, p string) bool {
	for _, m := range matchers {
		if m.regex.MatchString(p) {
			return true
		}
	}
	return false
}

func globToRegex(pattern string) string {
	var b strings.Builder
	b.WriteString("^")

	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		switch ch {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i++
				continue
			}
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			b.WriteByte('\\')
			b.WriteByte(ch)
		default:
			b.WriteByte(ch)
		}
	}

	b.WriteString("$")
	return b.String()
}
