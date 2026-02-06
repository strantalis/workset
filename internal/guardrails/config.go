package guardrails

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config describes LOC and allowlist policies for source files.
type Config struct {
	Thresholds     Thresholds `yaml:"thresholds"`
	SourceExts     []string   `yaml:"source_extensions"`
	TestPatterns   []string   `yaml:"test_patterns"`
	IgnorePatterns []string   `yaml:"ignore"`
	Allowlist      []string   `yaml:"allowlist"`
}

// Thresholds are the effective line limits for source and test files.
type Thresholds struct {
	Source int `yaml:"source"`
	Tests  int `yaml:"tests"`
}

// CompiledConfig stores regex-compiled path policies.
type CompiledConfig struct {
	Config

	ignoreMatchers    []matcher
	allowlistMatchers []matcher
	testMatchers      []matcher
}

func LoadConfig(configPath string) (Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("read guardrails config %q: %w", configPath, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode guardrails config %q: %w", configPath, err)
	}

	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Compile() (CompiledConfig, error) {
	ignoreMatchers, err := compilePatterns(c.IgnorePatterns)
	if err != nil {
		return CompiledConfig{}, err
	}

	allowlistMatchers, err := compilePatterns(c.Allowlist)
	if err != nil {
		return CompiledConfig{}, err
	}

	testMatchers, err := compilePatterns(c.TestPatterns)
	if err != nil {
		return CompiledConfig{}, err
	}

	return CompiledConfig{
		Config:            c,
		ignoreMatchers:    ignoreMatchers,
		allowlistMatchers: allowlistMatchers,
		testMatchers:      testMatchers,
	}, nil
}

func (c *Config) applyDefaults() {
	if c.Thresholds.Source == 0 {
		c.Thresholds.Source = 1000
	}
	if c.Thresholds.Tests == 0 {
		c.Thresholds.Tests = 1200
	}
	if len(c.SourceExts) == 0 {
		c.SourceExts = []string{".go", ".ts", ".svelte"}
	}
	if len(c.TestPatterns) == 0 {
		c.TestPatterns = []string{
			"*_test.go",
			"*.test.ts",
			"*.spec.ts",
			"*.test.svelte",
			"*.spec.svelte",
		}
	}
}

func (c Config) validate() error {
	if c.Thresholds.Source <= 0 {
		return errors.New("guardrails threshold.source must be > 0")
	}
	if c.Thresholds.Tests <= 0 {
		return errors.New("guardrails threshold.tests must be > 0")
	}

	for _, ext := range c.SourceExts {
		if !strings.HasPrefix(ext, ".") {
			return fmt.Errorf("source extension %q must start with '.'", ext)
		}
	}

	return nil
}

func normalizePath(p string) string {
	return strings.ReplaceAll(filepath.Clean(p), string(filepath.Separator), "/")
}

func (c CompiledConfig) IsSourceFile(p string) bool {
	ext := strings.ToLower(path.Ext(normalizePath(p)))
	for _, allowed := range c.SourceExts {
		if ext == strings.ToLower(allowed) {
			return true
		}
	}
	return false
}

func (c CompiledConfig) IsIgnored(p string) bool {
	return anyMatch(c.ignoreMatchers, normalizePath(p))
}

func (c CompiledConfig) IsAllowlisted(p string) bool {
	return anyMatch(c.allowlistMatchers, normalizePath(p))
}

func (c CompiledConfig) IsTestFile(p string) bool {
	normalized := normalizePath(p)
	if anyMatch(c.testMatchers, normalized) {
		return true
	}
	return anyMatch(c.testMatchers, path.Base(normalized))
}

func (c CompiledConfig) ThresholdFor(p string) int {
	if c.IsTestFile(p) {
		return c.Thresholds.Tests
	}
	return c.Thresholds.Source
}
