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

	fs.StringVar(&configPath, "config", "docs-dev/architecture/deprecation-register.yaml", "path to deprecation register")
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
		itemProblems, itemWarnings := validateRegisterItem(i, item, seen, today, warnCutoff)
		problems = append(problems, itemProblems...)
		warnings = append(warnings, itemWarnings...)
	}

	return problems, warnings
}

func validateRegisterItem(
	index int,
	item registerItem,
	seen map[string]struct{},
	today time.Time,
	warnCutoff time.Time,
) ([]string, []string) {
	idPrefix := fmt.Sprintf("items[%d]", index)
	id := strings.TrimSpace(item.ID)
	if id == "" {
		return []string{idPrefix + ".id is required"}, nil
	}
	prefix := fmt.Sprintf("item %q", id)
	if _, exists := seen[id]; exists {
		return []string{prefix + " has duplicate id"}, nil
	}
	seen[id] = struct{}{}

	problems := validateRegisterItemFields(prefix, item)
	statusProblems, statusWarnings := validateRegisterItemStatus(prefix, item, today, warnCutoff)
	problems = append(problems, statusProblems...)
	return problems, statusWarnings
}

func validateRegisterItemFields(prefix string, item registerItem) []string {
	requiredFields := map[string]string{
		"scope":          item.Scope,
		"summary":        item.Summary,
		"owner":          item.Owner,
		"tracking_issue": item.TrackingIssue,
	}
	problems := make([]string, 0, len(requiredFields)+2)
	for field, value := range requiredFields {
		if strings.TrimSpace(value) == "" {
			problems = append(problems, fmt.Sprintf("%s.%s is required", prefix, field))
		}
	}
	if len(item.Evidence) == 0 {
		problems = append(problems, prefix+".evidence must include at least one path")
	}
	if _, err := parseDate(item.Introduced); err != nil {
		problems = append(problems, fmt.Sprintf("%s.introduced: %v", prefix, err))
	}
	return problems
}

func validateRegisterItemStatus(
	prefix string,
	item registerItem,
	today time.Time,
	warnCutoff time.Time,
) ([]string, []string) {
	switch strings.TrimSpace(item.Status) {
	case statusActive:
		return validateActiveStatus(prefix, item, today, warnCutoff)
	case statusCompleted:
		return validateCompletedStatus(prefix, item), nil
	default:
		return []string{
			fmt.Sprintf("%s.status must be %q or %q", prefix, statusActive, statusCompleted),
		}, nil
	}
}

func validateActiveStatus(
	prefix string,
	item registerItem,
	today time.Time,
	warnCutoff time.Time,
) ([]string, []string) {
	removeBy, err := parseDate(item.RemoveBy)
	if err != nil {
		return []string{fmt.Sprintf("%s.remove_by: %v", prefix, err)}, nil
	}
	if today.After(removeBy) {
		return []string{
			fmt.Sprintf(
				"%s is overdue (remove_by=%s, today=%s)",
				prefix,
				removeBy.Format(time.DateOnly),
				today.Format(time.DateOnly),
			),
		}, nil
	}
	if !warnCutoff.Before(removeBy) {
		return nil, []string{
			fmt.Sprintf("%s is due soon (remove_by=%s)", prefix, removeBy.Format(time.DateOnly)),
		}
	}
	return nil, nil
}

func validateCompletedStatus(prefix string, item registerItem) []string {
	if strings.TrimSpace(item.RemoveBy) == "" {
		return nil
	}
	if _, err := parseDate(item.RemoveBy); err != nil {
		return []string{fmt.Sprintf("%s.remove_by: %v", prefix, err)}
	}
	return nil
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
