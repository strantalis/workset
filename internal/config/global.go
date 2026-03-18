package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	koanfyaml "github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	"gopkg.in/yaml.v3"
)

const (
	// LegacyGlobalConfigVersion represents pre-versioned configs.
	LegacyGlobalConfigVersion = 0
	// CurrentGlobalConfigVersion is the latest persisted config schema version.
	CurrentGlobalConfigVersion = 1
)

func GlobalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".workset", "config.yaml"), nil
}

type GlobalConfigLoadInfo struct {
	Path                 string
	Exists               bool
	ConfigVersion        int
	ConfigVersionPresent bool
}

type serializedWorksetGroup struct {
	Repos   []string                `yaml:"repos,omitempty" json:"repos,omitempty"`
	Threads map[string]WorkspaceRef `yaml:"threads,omitempty" json:"threads,omitempty"`
}

type serializedGlobalConfig struct {
	ConfigVersion int                               `yaml:"config_version,omitempty" json:"config_version,omitempty"`
	Defaults      Defaults                          `yaml:"defaults" json:"defaults"`
	GitHub        GitHubConfig                      `yaml:"github,omitempty" json:"github,omitempty"`
	Agent         AgentConfig                       `yaml:"agent,omitempty" json:"agent,omitempty"`
	Hooks         HooksConfig                       `yaml:"hooks,omitempty" json:"hooks,omitempty"`
	Repos         map[string]RegisteredRepo         `yaml:"repos" json:"repos"`
	Worksets      map[string]serializedWorksetGroup `yaml:"worksets,omitempty" json:"worksets,omitempty"`
}

func LoadGlobalWithInfo(path string) (GlobalConfig, GlobalConfigLoadInfo, error) {
	cfg, info, err := loadGlobal(path)
	return cfg, info, err
}

func LoadGlobal(path string) (GlobalConfig, error) {
	cfg, _, err := loadGlobal(path)
	return cfg, err
}

func loadGlobal(path string) (GlobalConfig, GlobalConfigLoadInfo, error) {
	info := GlobalConfigLoadInfo{}
	var rawData []byte
	if path == "" {
		var err error
		path, err = GlobalConfigPath()
		if err != nil {
			return GlobalConfig{}, info, err
		}
	}
	info.Path = path

	defaults := DefaultConfig()
	info.ConfigVersion = defaults.ConfigVersion

	k := koanf.New(".")
	if err := k.Load(confmap.Provider(defaultConfigMap(defaults), "."), nil); err != nil {
		return GlobalConfig{}, info, err
	}

	if _, err := os.Stat(path); err == nil {
		info.Exists = true
		readData, readErr := os.ReadFile(path)
		if readErr != nil {
			return GlobalConfig{}, info, readErr
		}
		rawData = readData
		parsedVersion, present, parseErr := parseConfigVersion(rawData)
		if parseErr != nil {
			return GlobalConfig{}, info, parseErr
		}
		info.ConfigVersionPresent = present
		if present {
			info.ConfigVersion = parsedVersion
		} else {
			info.ConfigVersion = LegacyGlobalConfigVersion
		}
		if err := k.Load(file.Provider(path), koanfyaml.Parser()); err != nil {
			return GlobalConfig{}, info, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return GlobalConfig{}, info, err
	}

	var cfg GlobalConfig
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
		return GlobalConfig{}, info, err
	}
	if len(rawData) > 0 {
		nestedWorkspaces, nestedWorksetRepos, hasNestedWorksets, err := parseNestedWorksets(rawData)
		if err != nil {
			return GlobalConfig{}, info, err
		}
		if hasNestedWorksets {
			cfg.Workspaces = nestedWorkspaces
			cfg.WorksetRepos = nestedWorksetRepos
		}
	}
	cfg.ConfigVersion = normalizeConfigVersion(info.ConfigVersion)
	if cfg.ConfigVersion > CurrentGlobalConfigVersion {
		return GlobalConfig{}, info, fmt.Errorf(
			"unsupported config_version %d (max supported %d)",
			cfg.ConfigVersion,
			CurrentGlobalConfigVersion,
		)
	}
	finalizeGlobal(&cfg, defaults)
	return cfg, info, nil
}

func loadGlobalFromBytes(data []byte) (GlobalConfig, error) {
	defaults := DefaultConfig()
	version := defaults.ConfigVersion
	if len(bytes.TrimSpace(data)) > 0 {
		parsedVersion, present, err := parseConfigVersion(data)
		if err != nil {
			return GlobalConfig{}, err
		}
		if present {
			version = parsedVersion
		} else {
			version = LegacyGlobalConfigVersion
		}
	}
	version = normalizeConfigVersion(version)
	if version > CurrentGlobalConfigVersion {
		return GlobalConfig{}, fmt.Errorf(
			"unsupported config_version %d (max supported %d)",
			version,
			CurrentGlobalConfigVersion,
		)
	}
	k := koanf.New(".")
	if err := k.Load(confmap.Provider(defaultConfigMap(defaults), "."), nil); err != nil {
		return GlobalConfig{}, err
	}
	if len(bytes.TrimSpace(data)) > 0 {
		if err := k.Load(rawbytes.Provider(data), koanfyaml.Parser()); err != nil {
			return GlobalConfig{}, err
		}
	}
	var cfg GlobalConfig
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
		return GlobalConfig{}, err
	}
	if len(data) > 0 {
		nestedWorkspaces, nestedWorksetRepos, hasNestedWorksets, err := parseNestedWorksets(data)
		if err != nil {
			return GlobalConfig{}, err
		}
		if hasNestedWorksets {
			cfg.Workspaces = nestedWorkspaces
			cfg.WorksetRepos = nestedWorksetRepos
		}
	}
	cfg.ConfigVersion = version
	finalizeGlobal(&cfg, defaults)
	return cfg, nil
}

func SaveGlobal(path string, cfg GlobalConfig) error {
	if path == "" {
		var err error
		path, err = GlobalConfigPath()
		if err != nil {
			return err
		}
	}
	cfg.EnsureMaps()
	cfg.ConfigVersion = ensureCurrentConfigVersion(cfg.ConfigVersion)
	cfg = sanitizeGlobalForSave(cfg)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if info, err := os.Stat(path); err == nil {
		existing, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path+".bak", existing, info.Mode().Perm()); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := yaml.Marshal(toSerializedGlobalConfig(cfg))
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func sanitizeGlobalForSave(cfg GlobalConfig) GlobalConfig {
	for name, ref := range cfg.Workspaces {
		ref.RepoOverrides = normalizeRepoList(ref.RepoOverrides)
		cfg.Workspaces[name] = ref
	}
	for workset, repos := range cfg.WorksetRepos {
		cfg.WorksetRepos[workset] = normalizeRepoList(repos)
	}
	cfg.ConfigVersion = ensureCurrentConfigVersion(cfg.ConfigVersion)
	return cfg
}

func toSerializedGlobalConfig(cfg GlobalConfig) serializedGlobalConfig {
	worksets := map[string]serializedWorksetGroup{}
	for threadName, ref := range cfg.Workspaces {
		normalizedThread := strings.TrimSpace(threadName)
		if normalizedThread == "" {
			continue
		}
		worksetName := strings.TrimSpace(ref.Workset)
		if worksetName == "" {
			worksetName = normalizedThread
		}
		ref.RepoOverrides = normalizeRepoList(ref.RepoOverrides)
		group := worksets[worksetName]
		if group.Threads == nil {
			group.Threads = map[string]WorkspaceRef{}
		}
		group.Repos = normalizeRepoList(cfg.WorksetRepos[worksetName])
		group.Threads[normalizedThread] = ref
		worksets[worksetName] = group
	}
	for worksetName, repos := range cfg.WorksetRepos {
		normalizedWorksetName := strings.TrimSpace(worksetName)
		if normalizedWorksetName == "" {
			continue
		}
		group := worksets[normalizedWorksetName]
		group.Repos = normalizeRepoList(repos)
		worksets[normalizedWorksetName] = group
	}
	return serializedGlobalConfig{
		ConfigVersion: cfg.ConfigVersion,
		Defaults:      cfg.Defaults,
		GitHub:        cfg.GitHub,
		Agent:         cfg.Agent,
		Hooks:         cfg.Hooks,
		Repos:         cfg.Repos,
		Worksets:      worksets,
	}
}

// CanonicalGlobalForOutput returns the canonical persisted config shape for display/export.
func CanonicalGlobalForOutput(cfg GlobalConfig) any {
	cfg.EnsureMaps()
	cfg = sanitizeGlobalForSave(cfg)
	return toSerializedGlobalConfig(cfg)
}

func parseNestedWorksets(raw []byte) (map[string]WorkspaceRef, map[string][]string, bool, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil, false, nil
	}
	var root map[string]any
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil, nil, false, err
	}
	worksetsValue, ok := root["worksets"]
	if !ok {
		return nil, nil, false, nil
	}
	worksetsMap, ok := asStringAnyMap(worksetsValue)
	if !ok {
		return nil, nil, false, nil
	}
	hasNested := false
	for _, rawGroup := range worksetsMap {
		groupMap, ok := asStringAnyMap(rawGroup)
		if !ok {
			continue
		}
		if len(groupMap) == 0 {
			hasNested = true
			break
		}
		if _, ok := groupMap["threads"]; ok {
			hasNested = true
			break
		}
		if _, ok := groupMap["repos"]; ok {
			hasNested = true
			break
		}
	}
	if !hasNested {
		return nil, nil, false, nil
	}
	var serialized struct {
		Worksets map[string]serializedWorksetGroup `yaml:"worksets"`
	}
	if err := yaml.Unmarshal(raw, &serialized); err != nil {
		return nil, nil, false, err
	}
	flattened := map[string]WorkspaceRef{}
	worksetRepos := map[string][]string{}
	for worksetName, group := range serialized.Worksets {
		normalizedWorksetName := strings.TrimSpace(worksetName)
		if normalizedWorksetName != "" {
			worksetRepos[normalizedWorksetName] = normalizeRepoList(group.Repos)
		}
		for threadName, ref := range group.Threads {
			normalizedThread := strings.TrimSpace(threadName)
			if normalizedThread == "" {
				continue
			}
			if strings.TrimSpace(ref.Workset) == "" && normalizedWorksetName != normalizedThread {
				ref.Workset = normalizedWorksetName
			}
			ref.RepoOverrides = normalizeRepoList(ref.RepoOverrides)
			flattened[normalizedThread] = ref
		}
	}
	return flattened, worksetRepos, true, nil
}

func normalizeRepoList(repos []string) []string {
	if len(repos) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(repos))
	for _, repo := range repos {
		trimmed := strings.TrimSpace(repo)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func asStringAnyMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[any]any:
		out := map[string]any{}
		for key, val := range typed {
			keyText, ok := key.(string)
			if !ok {
				continue
			}
			out[keyText] = val
		}
		return out, true
	default:
		return nil, false
	}
}

func defaultConfigMap(defaults GlobalConfig) map[string]any {
	return map[string]any{
		"defaults.remote":                 defaults.Defaults.Remote,
		"defaults.base_branch":            defaults.Defaults.BaseBranch,
		"defaults.thread":                 defaults.Defaults.Thread,
		"defaults.workset_root":           defaults.Defaults.WorksetRoot,
		"defaults.repo_store_root":        defaults.Defaults.RepoStoreRoot,
		"defaults.agent":                  defaults.Defaults.Agent,
		"defaults.agent_model":            defaults.Defaults.AgentModel,
		"defaults.terminal_idle_timeout":  defaults.Defaults.TerminalIdleTimeout,
		"defaults.terminal_protocol_log":  defaults.Defaults.TerminalProtocolLog,
		"defaults.terminal_debug_overlay": defaults.Defaults.TerminalDebugOverlay,
		"defaults.terminal_keybindings":   defaults.Defaults.TerminalKeybindings,
		"github.cli_path":                 defaults.GitHub.CLIPath,
		"hooks.enabled":                   defaults.Hooks.Enabled,
		"hooks.on_error":                  defaults.Hooks.OnError,
		"hooks.repo_hooks.trusted_repos":  defaults.Hooks.RepoHooks.TrustedRepos,
	}
}

func finalizeGlobal(cfg *GlobalConfig, defaults GlobalConfig) {
	cfg.EnsureMaps()
	cfg.ConfigVersion = normalizeConfigVersion(cfg.ConfigVersion)
	for name, ref := range cfg.Workspaces {
		ref.RepoOverrides = normalizeRepoList(ref.RepoOverrides)
		cfg.Workspaces[name] = ref
	}
	for workset, repos := range cfg.WorksetRepos {
		normalizedName := strings.TrimSpace(workset)
		normalizedRepos := normalizeRepoList(repos)
		if normalizedName == "" {
			delete(cfg.WorksetRepos, workset)
			continue
		}
		if normalizedName != workset {
			delete(cfg.WorksetRepos, workset)
		}
		cfg.WorksetRepos[normalizedName] = normalizedRepos
	}
	if cfg.Defaults.Remote == "" {
		cfg.Defaults.Remote = defaults.Defaults.Remote
	}
	if cfg.Defaults.BaseBranch == "" {
		cfg.Defaults = defaults.Defaults
	}
	if cfg.Defaults.Thread == "" {
		cfg.Defaults.Thread = defaults.Defaults.Thread
	}
	if cfg.Defaults.WorksetRoot == "" {
		cfg.Defaults.WorksetRoot = defaults.Defaults.WorksetRoot
	}
	if cfg.Defaults.RepoStoreRoot == "" {
		cfg.Defaults.RepoStoreRoot = defaults.Defaults.RepoStoreRoot
	}
	if cfg.Defaults.Agent == "" {
		cfg.Defaults.Agent = defaults.Defaults.Agent
	}
	if cfg.Defaults.TerminalIdleTimeout == "" {
		cfg.Defaults.TerminalIdleTimeout = defaults.Defaults.TerminalIdleTimeout
	}
	if cfg.Defaults.TerminalProtocolLog == "" {
		cfg.Defaults.TerminalProtocolLog = defaults.Defaults.TerminalProtocolLog
	}
	if cfg.Defaults.TerminalDebugOverlay == "" {
		cfg.Defaults.TerminalDebugOverlay = defaults.Defaults.TerminalDebugOverlay
	}
	if cfg.Defaults.TerminalKeybindings == nil {
		cfg.Defaults.TerminalKeybindings = defaults.Defaults.TerminalKeybindings
	}
	if cfg.Hooks.OnError == "" {
		cfg.Hooks.OnError = defaults.Hooks.OnError
	}
	if cfg.Hooks.RepoHooks.TrustedRepos == nil {
		cfg.Hooks.RepoHooks.TrustedRepos = defaults.Hooks.RepoHooks.TrustedRepos
	}
	if cfg.Hooks.Items == nil {
		cfg.Hooks.Items = defaults.Hooks.Items
	}
}

func normalizeConfigVersion(value int) int {
	if value < LegacyGlobalConfigVersion {
		return LegacyGlobalConfigVersion
	}
	return value
}

func ensureCurrentConfigVersion(value int) int {
	value = normalizeConfigVersion(value)
	if value < CurrentGlobalConfigVersion {
		return CurrentGlobalConfigVersion
	}
	return value
}

func parseConfigVersion(raw []byte) (int, bool, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return 0, false, nil
	}
	var root map[string]any
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return 0, false, err
	}
	value, ok := root["config_version"]
	if !ok {
		return 0, false, nil
	}
	switch parsed := value.(type) {
	case int:
		return parsed, true, nil
	case int64:
		return int(parsed), true, nil
	case float64:
		return int(parsed), true, nil
	case string:
		trimmed := strings.TrimSpace(parsed)
		if trimmed == "" {
			return 0, true, nil
		}
		numeric, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, false, errors.New("config_version must be an integer")
		}
		return numeric, true, nil
	default:
		return 0, false, errors.New("config_version must be an integer")
	}
}
