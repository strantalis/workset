package guardrails

import "sort"

type ViolationReason string

const (
	ReasonMissingAllowlist  ViolationReason = "missing_allowlist"
	ReasonAllowlistedGrowth ViolationReason = "allowlisted_growth"
)

type FileStat struct {
	Path      string
	LOC       int
	Threshold int
}

type Violation struct {
	Path      string
	Reason    ViolationReason
	HeadLOC   int
	BaseLOC   int
	Threshold int
}

type EvaluationResult struct {
	Checked    []FileStat
	Oversized  []FileStat
	Violations []Violation
}

// Evaluate applies LOC guardrails and ratchet checks.
func Evaluate(cfg CompiledConfig, headLOC map[string]int, baseLOC map[string]int, hasBase bool) EvaluationResult {
	paths := make([]string, 0, len(headLOC))
	for p := range headLOC {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	result := EvaluationResult{
		Checked: make([]FileStat, 0, len(paths)),
	}

	for _, p := range paths {
		loc := headLOC[p]
		threshold := cfg.ThresholdFor(p)
		stat := FileStat{
			Path:      p,
			LOC:       loc,
			Threshold: threshold,
		}
		result.Checked = append(result.Checked, stat)

		if loc <= threshold {
			continue
		}
		result.Oversized = append(result.Oversized, stat)

		if !cfg.IsAllowlisted(p) {
			result.Violations = append(result.Violations, Violation{
				Path:      p,
				Reason:    ReasonMissingAllowlist,
				HeadLOC:   loc,
				Threshold: threshold,
			})
			continue
		}

		if !hasBase {
			continue
		}
		base, ok := baseLOC[p]
		if !ok {
			continue
		}
		if base > threshold && loc > base {
			result.Violations = append(result.Violations, Violation{
				Path:      p,
				Reason:    ReasonAllowlistedGrowth,
				HeadLOC:   loc,
				BaseLOC:   base,
				Threshold: threshold,
			})
		}
	}

	return result
}
