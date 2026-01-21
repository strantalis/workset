package main

import "os/exec"

func (a *App) GetAgentAvailability() map[string]bool {
	agents := map[string][]string{
		"codex":    {"codex"},
		"claude":   {"claude"},
		"opencode": {"opencode"},
		"pi":       {"pi"},
		"cursor":   {"cursor"},
	}

	results := make(map[string]bool, len(agents))
	for id, bins := range agents {
		available := false
		for _, bin := range bins {
			if _, err := exec.LookPath(bin); err == nil {
				available = true
				break
			}
		}
		results[id] = available
	}
	return results
}
