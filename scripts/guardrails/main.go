package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strantalis/workset/internal/guardrails"
)

type options struct {
	configPath string
	baseSHA    string
	headSHA    string
}

func main() {
	opts := parseFlags()

	cfg, err := guardrails.LoadConfig(opts.configPath)
	if err != nil {
		exitf("load config: %v", err)
	}
	compiled, err := cfg.Compile()
	if err != nil {
		exitf("compile config: %v", err)
	}

	headLOC, err := loadHeadLOC(compiled)
	if err != nil {
		exitf("scan tracked files: %v", err)
	}

	baseLOC := map[string]int{}
	hasBase := strings.TrimSpace(opts.baseSHA) != ""
	if hasBase {
		baseLOC, err = loadBaseLOC(opts.baseSHA, compiled, headLOC)
		if err != nil {
			exitf("load base file stats: %v", err)
		}
	}

	result := guardrails.Evaluate(compiled, headLOC, baseLOC, hasBase)
	printSummary(result, opts)
	if len(result.Violations) > 0 {
		os.Exit(1)
	}
}

func parseFlags() options {
	var opts options
	flag.StringVar(&opts.configPath, "config", "guardrails.yml", "path to guardrail policy config")
	flag.StringVar(&opts.baseSHA, "base-sha", "", "base git revision for ratchet checks")
	flag.StringVar(&opts.headSHA, "head-sha", "", "head git revision (for display only)")
	flag.Parse()
	return opts
}

func loadHeadLOC(cfg guardrails.CompiledConfig) (map[string]int, error) {
	paths, err := gitTrackedFiles()
	if err != nil {
		return nil, err
	}

	headLOC := make(map[string]int, len(paths))
	for _, p := range paths {
		if !cfg.IsSourceFile(p) || cfg.IsIgnored(p) {
			continue
		}

		data, err := os.ReadFile(filepath.Clean(p))
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", p, err)
		}
		headLOC[p] = guardrails.CountLOC(p, data)
	}

	return headLOC, nil
}

func loadBaseLOC(baseSHA string, cfg guardrails.CompiledConfig, head map[string]int) (map[string]int, error) {
	baseLOC := make(map[string]int)

	type candidate struct {
		path      string
		threshold int
		loc       int
	}

	candidates := make([]candidate, 0, len(head))
	for p, loc := range head {
		threshold := cfg.ThresholdFor(p)
		if loc <= threshold || !cfg.IsAllowlisted(p) {
			continue
		}
		candidates = append(candidates, candidate{
			path:      p,
			threshold: threshold,
			loc:       loc,
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].path < candidates[j].path
	})

	for _, c := range candidates {
		data, found, err := gitShow(baseSHA, c.path)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		baseLOC[c.path] = guardrails.CountLOC(c.path, data)
	}

	return baseLOC, nil
}

func gitTrackedFiles() ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files -z: %w", err)
	}

	items := bytes.Split(out, []byte{0})
	paths := make([]string, 0, len(items))
	for _, it := range items {
		p := strings.TrimSpace(string(it))
		if p == "" {
			continue
		}
		paths = append(paths, filepath.ToSlash(p))
	}
	return paths, nil
}

func gitShow(rev string, p string) ([]byte, bool, error) {
	spec := fmt.Sprintf("%s:%s", rev, p)
	cmd := exec.Command("git", "show", spec)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, true, nil
	}

	msg := strings.ToLower(string(out))
	if strings.Contains(msg, "exists on disk, but not in") ||
		strings.Contains(msg, "does not exist in") ||
		strings.Contains(msg, "path '") && strings.Contains(msg, "does not exist") {
		return nil, false, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil, false, fmt.Errorf("git show %s failed: %s", spec, strings.TrimSpace(string(out)))
	}
	return nil, false, fmt.Errorf("git show %s: %w", spec, err)
}

func printSummary(result guardrails.EvaluationResult, opts options) {
	out := os.Stdout
	head := opts.headSHA
	if head == "" {
		head = "working-tree"
	}
	if opts.baseSHA != "" {
		_, _ = fmt.Fprintf(out, "guardrails: checked %d source files against %q (base=%s, head=%s)\n", len(result.Checked), opts.configPath, opts.baseSHA, head)
	} else {
		_, _ = fmt.Fprintf(out, "guardrails: checked %d source files against %q\n", len(result.Checked), opts.configPath)
	}

	if len(result.Violations) == 0 {
		_, _ = fmt.Fprintf(out, "guardrails: pass (%d oversized allowlisted files tracked)\n", len(result.Oversized))
		return
	}

	_, _ = fmt.Fprintf(out, "guardrails: %d violation(s)\n", len(result.Violations))
	for _, v := range result.Violations {
		switch v.Reason {
		case guardrails.ReasonMissingAllowlist:
			_, _ = fmt.Fprintf(out, "- %s: LOC=%d exceeds threshold=%d and is not allowlisted\n", v.Path, v.HeadLOC, v.Threshold)
		case guardrails.ReasonAllowlistedGrowth:
			_, _ = fmt.Fprintf(out, "- %s: allowlisted oversized file grew (base=%d -> head=%d, threshold=%d)\n", v.Path, v.BaseLOC, v.HeadLOC, v.Threshold)
		default:
			_, _ = fmt.Fprintf(out, "- %s: LOC=%d threshold=%d reason=%s\n", v.Path, v.HeadLOC, v.Threshold, v.Reason)
		}
	}
}

func exitf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "guardrails: "+format+"\n", args...)
	os.Exit(2)
}
