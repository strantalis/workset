package guardrails

import "testing"

func TestCompiledConfigPathPolicies(t *testing.T) {
	cfg := Config{
		Thresholds: Thresholds{
			Source: 1000,
			Tests:  1200,
		},
		SourceExts:     []string{".go", ".ts", ".svelte"},
		TestPatterns:   []string{"*_test.go", "*.spec.ts"},
		IgnorePatterns: []string{"**/node_modules/**", "wails-ui/workset/frontend/dist/**"},
		Allowlist:      []string{"pkg/sessiond/session.go", "wails-ui/workset/frontend/src/lib/components/*.svelte"},
	}

	compiled, err := cfg.Compile()
	if err != nil {
		t.Fatalf("compile config: %v", err)
	}

	if !compiled.IsSourceFile("pkg/sessiond/session.go") {
		t.Fatalf("expected .go file to be a source file")
	}
	if compiled.IsSourceFile("README.md") {
		t.Fatalf("did not expect .md file to be a source file")
	}
	if !compiled.IsTestFile("internal/foo/bar_test.go") {
		t.Fatalf("expected *_test.go to match test pattern")
	}
	if !compiled.IsIgnored("wails-ui/workset/frontend/dist/assets/main.js") {
		t.Fatalf("expected frontend dist file to be ignored")
	}
	if !compiled.IsAllowlisted("wails-ui/workset/frontend/src/lib/components/RepoDiff.svelte") {
		t.Fatalf("expected svelte component to match allowlist pattern")
	}
}

func TestThresholdFor(t *testing.T) {
	cfg := Config{
		Thresholds: Thresholds{
			Source: 1000,
			Tests:  1200,
		},
		SourceExts:   []string{".go"},
		TestPatterns: []string{"*_test.go"},
	}

	compiled, err := cfg.Compile()
	if err != nil {
		t.Fatalf("compile config: %v", err)
	}

	if got := compiled.ThresholdFor("pkg/worksetapi/sessions.go"); got != 1000 {
		t.Fatalf("threshold for source file = %d, want 1000", got)
	}
	if got := compiled.ThresholdFor("pkg/worksetapi/sessions_test.go"); got != 1200 {
		t.Fatalf("threshold for test file = %d, want 1200", got)
	}
}
