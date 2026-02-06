package guardrails

import "testing"

func mustCompileConfig(t *testing.T, cfg Config) CompiledConfig {
	t.Helper()
	compiled, err := cfg.Compile()
	if err != nil {
		t.Fatalf("compile config: %v", err)
	}
	return compiled
}

func TestEvaluateMissingAllowlist(t *testing.T) {
	cfg := mustCompileConfig(t, Config{
		Thresholds: Thresholds{Source: 1000, Tests: 1200},
		SourceExts: []string{".go"},
	})

	head := map[string]int{
		"pkg/worksetapi/github_service.go": 1200,
	}

	result := Evaluate(cfg, head, nil, false)
	if len(result.Violations) != 1 {
		t.Fatalf("violations = %d, want 1", len(result.Violations))
	}
	if result.Violations[0].Reason != ReasonMissingAllowlist {
		t.Fatalf("reason = %s, want %s", result.Violations[0].Reason, ReasonMissingAllowlist)
	}
}

func TestEvaluateAllowlistedGrowth(t *testing.T) {
	cfg := mustCompileConfig(t, Config{
		Thresholds: Thresholds{Source: 1000, Tests: 1200},
		SourceExts: []string{".go"},
		Allowlist:  []string{"pkg/sessiond/session.go"},
	})

	head := map[string]int{
		"pkg/sessiond/session.go": 2100,
	}
	base := map[string]int{
		"pkg/sessiond/session.go": 2000,
	}

	result := Evaluate(cfg, head, base, true)
	if len(result.Violations) != 1 {
		t.Fatalf("violations = %d, want 1", len(result.Violations))
	}
	if result.Violations[0].Reason != ReasonAllowlistedGrowth {
		t.Fatalf("reason = %s, want %s", result.Violations[0].Reason, ReasonAllowlistedGrowth)
	}
}

func TestEvaluateAllowlistedShrinkPasses(t *testing.T) {
	cfg := mustCompileConfig(t, Config{
		Thresholds: Thresholds{Source: 1000, Tests: 1200},
		SourceExts: []string{".go"},
		Allowlist:  []string{"pkg/sessiond/session.go"},
	})

	head := map[string]int{
		"pkg/sessiond/session.go": 2050,
	}
	base := map[string]int{
		"pkg/sessiond/session.go": 2100,
	}

	result := Evaluate(cfg, head, base, true)
	if len(result.Violations) != 0 {
		t.Fatalf("violations = %d, want 0", len(result.Violations))
	}
}
