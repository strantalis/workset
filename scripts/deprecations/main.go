package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	statusActive    = "active"
	statusCompleted = "completed"
)

type register struct {
	Version int            `yaml:"version"`
	Items   []registerItem `yaml:"items"`
}

type registerItem struct {
	ID            string   `yaml:"id"`
	Scope         string   `yaml:"scope"`
	Summary       string   `yaml:"summary"`
	Introduced    string   `yaml:"introduced"`
	RemoveBy      string   `yaml:"remove_by"`
	Owner         string   `yaml:"owner"`
	TrackingIssue string   `yaml:"tracking_issue"`
	Status        string   `yaml:"status"`
	Evidence      []string `yaml:"evidence"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, time.Now().UTC()))
}

func run(args []string, out io.Writer, errOut io.Writer, now time.Time) int {
	fs := flag.NewFlagSet("deprecations", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var (
		configPath string
		warnDays   int
	)

	fs.StringVar(&configPath, "config", "docs/architecture/deprecation-register.yaml", "path to deprecation register")
	fs.IntVar(&warnDays, "warn-days", 30, "warn when remove_by is within this many days")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	reg, err := loadRegister(configPath)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "deprecations: %v\n", err)
		return 1
	}

	problems, warnings := validateRegister(reg, now, warnDays)

	sort.Strings(warnings)
	for _, warning := range warnings {
		_, _ = fmt.Fprintf(out, "deprecations: warning: %s\n", warning)
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		_, _ = fmt.Fprintf(errOut, "deprecations: %d validation error(s)\n", len(problems))
		for _, problem := range problems {
			_, _ = fmt.Fprintf(errOut, "deprecations: %s\n", problem)
		}
		return 1
	}

	_, _ = fmt.Fprintf(out, "deprecations: pass (%d entries)\n", len(reg.Items))
	return 0
}

func loadRegister(path string) (register, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return register{}, fmt.Errorf("read register %q: %w", path, err)
	}
	var reg register
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return register{}, fmt.Errorf("parse register %q: %w", path, err)
	}
	return reg, nil
}

func validateRegister(reg register, now time.Time, warnDays int) ([]string, []string) {
	var (
		problems []string
		warnings []string
	)

	if reg.Version <= 0 {
		problems = append(problems, "version must be >= 1")
	}
	if len(reg.Items) == 0 {
		problems = append(problems, "items must not be empty")
		return problems, warnings
	}

	seen := map[string]struct{}{}
	today := dateOnly(now)
	warnCutoff := today.AddDate(0, 0, warnDays)

	for i, item := range reg.Items {
		prefix := fmt.Sprintf("items[%d]", i)
		id := strings.TrimSpace(item.ID)
		if id == "" {
			problems = append(problems, prefix+".id is required")
			continue
		}
		prefix = fmt.Sprintf("item %q", id)
		if _, exists := seen[id]; exists {
			problems = append(problems, prefix+" has duplicate id")
			continue
		}
		seen[id] = struct{}{}

		if strings.TrimSpace(item.Scope) == "" {
			problems = append(problems, prefix+".scope is required")
		}
		if strings.TrimSpace(item.Summary) == "" {
			problems = append(problems, prefix+".summary is required")
		}
		if strings.TrimSpace(item.Owner) == "" {
			problems = append(problems, prefix+".owner is required")
		}
		if strings.TrimSpace(item.TrackingIssue) == "" {
			problems = append(problems, prefix+".tracking_issue is required")
		}
		if len(item.Evidence) == 0 {
			problems = append(problems, prefix+".evidence must include at least one path")
		}
		if _, err := parseDate(item.Introduced); err != nil {
			problems = append(problems, fmt.Sprintf("%s.introduced: %v", prefix, err))
		}

		status := strings.TrimSpace(item.Status)
		switch status {
		case statusActive:
			removeBy, err := parseDate(item.RemoveBy)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s.remove_by: %v", prefix, err))
				continue
			}
			if today.After(removeBy) {
				problems = append(problems, fmt.Sprintf("%s is overdue (remove_by=%s, today=%s)", prefix, removeBy.Format(time.DateOnly), today.Format(time.DateOnly)))
				continue
			}
			if !warnCutoff.Before(removeBy) {
				warnings = append(warnings, fmt.Sprintf("%s is due soon (remove_by=%s)", prefix, removeBy.Format(time.DateOnly)))
			}
		case statusCompleted:
			if strings.TrimSpace(item.RemoveBy) != "" {
				if _, err := parseDate(item.RemoveBy); err != nil {
					problems = append(problems, fmt.Sprintf("%s.remove_by: %v", prefix, err))
				}
			}
		default:
			problems = append(problems, fmt.Sprintf("%s.status must be %q or %q", prefix, statusActive, statusCompleted))
		}
	}

	return problems, warnings
}

func parseDate(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, errors.New("must be set (YYYY-MM-DD)")
	}
	parsed, err := time.Parse(time.DateOnly, trimmed)
	if err != nil {
		return time.Time{}, errors.New("must be YYYY-MM-DD")
	}
	return parsed, nil
}

func dateOnly(ts time.Time) time.Time {
	return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)
}
