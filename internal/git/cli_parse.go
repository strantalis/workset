package git

import (
	"fmt"
	"strings"
)

type nameStatusEntry struct {
	status  string
	oldPath string
	newPath string
}

func splitNull(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	parts := strings.Split(string(data), "\x00")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func parseNameStatus(data []byte) ([]nameStatusEntry, error) {
	parts := splitNull(data)
	if len(parts) == 0 {
		return nil, nil
	}
	entries := make([]nameStatusEntry, 0)
	for i := 0; i < len(parts); {
		status := parts[i]
		i++
		if status == "" {
			continue
		}
		code := status[:1]
		switch code {
		case "R", "C":
			if i+1 >= len(parts) {
				return nil, fmt.Errorf("invalid name-status entry for %s", status)
			}
			entries = append(entries, nameStatusEntry{
				status:  code,
				oldPath: parts[i],
				newPath: parts[i+1],
			})
			i += 2
		default:
			if i >= len(parts) {
				return nil, fmt.Errorf("invalid name-status entry for %s", status)
			}
			entries = append(entries, nameStatusEntry{
				status:  code,
				oldPath: parts[i],
			})
			i++
		}
	}
	return entries, nil
}

type treeEntry struct {
	mode string
	hash string
}
