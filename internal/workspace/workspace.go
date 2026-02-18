package workspace

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/strantalis/workset/internal/config"
)

const (
	worksetFileName = "workset.yaml"
	stateFileName   = "state.json"
	branchMetaFile  = ".workset-branch"
	agentsFileName  = "AGENTS.md"
	claudeFileName  = "CLAUDE.md"
)

type Workspace struct {
	Root   string
	Config config.WorkspaceConfig
	State  State
}

type State struct {
	CurrentBranch string                      `json:"current_branch"`
	Sessions      map[string]SessionState     `json:"sessions,omitempty"`
	PullRequests  map[string]PullRequestState `json:"pull_requests,omitempty"`
}

type SessionState struct {
	Backend      string   `json:"backend,omitempty"`
	Name         string   `json:"name,omitempty"`
	Command      []string `json:"command,omitempty"`
	StartedAt    string   `json:"started_at,omitempty"`
	LastAttached string   `json:"last_attached,omitempty"`
}

type PullRequestState struct {
	Repo       string `json:"repo"`
	Number     int    `json:"number"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	Body       string `json:"body,omitempty"`
	State      string `json:"state"`
	Draft      bool   `json:"draft"`
	BaseRepo   string `json:"base_repo"`
	BaseBranch string `json:"base_branch"`
	HeadRepo   string `json:"head_repo"`
	HeadBranch string `json:"head_branch"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

func WorksetFile(root string) string {
	return filepath.Join(root, worksetFileName)
}

func AgentsFile(root string) string {
	return filepath.Join(root, agentsFileName)
}

func ClaudeFile(root string) string {
	return filepath.Join(root, claudeFileName)
}

func StatePath(root string) string {
	return filepath.Join(root, ".workset", stateFileName)
}

func LogsPath(root string) string {
	return filepath.Join(root, ".workset", "logs")
}

func WorktreesPath(root string) string {
	return filepath.Join(root, "worktrees")
}

func WorktreeBranchPath(root, branch string) string {
	return filepath.Join(WorktreesPath(root), WorktreeDirName(branch))
}

func BranchMetaPath(root, branch string) string {
	return filepath.Join(WorktreeBranchPath(root, branch), branchMetaFile)
}

func RepoWorktreePath(root, branch, repoDir string) string {
	if UseBranchDirs(root) {
		return filepath.Join(WorktreeBranchPath(root, branch), repoDir)
	}
	return filepath.Join(root, repoDir)
}

func WorktreeName(branch string) string {
	if branch == "" {
		branch = "branch"
	}
	sanitized := sanitizeWorktreeName(branch)
	hash := shortHash(branch)
	return sanitized + "-" + hash
}

func FindRoot(start string) (string, error) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, worksetFileName)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("workset.yaml not found from %s", start)
		}
		dir = parent
	}
}

func Init(root, name string, defaults config.Defaults) (Workspace, error) {
	if root == "" {
		return Workspace{}, errors.New("workspace root required")
	}

	if _, err := os.Stat(WorksetFile(root)); err == nil {
		return Workspace{}, fmt.Errorf("workset.yaml already exists at %s", WorksetFile(root))
	} else if !errors.Is(err, os.ErrNotExist) {
		return Workspace{}, err
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return Workspace{}, err
	}
	if err := os.MkdirAll(LogsPath(root), 0o755); err != nil {
		return Workspace{}, err
	}

	wsConfig := config.WorkspaceConfig{
		Name:  name,
		Repos: []config.RepoConfig{},
	}
	if err := config.SaveWorkspace(WorksetFile(root), wsConfig); err != nil {
		return Workspace{}, err
	}

	branch := WorkspaceBranchName(name)
	if branch == "" {
		branch = defaults.BaseBranch
	}
	state := State{CurrentBranch: branch}
	if err := saveState(root, state); err != nil {
		return Workspace{}, err
	}
	if err := UpdateAgentsFile(root, wsConfig, state); err != nil {
		return Workspace{}, err
	}

	return Workspace{
		Root:   root,
		Config: wsConfig,
		State:  state,
	}, nil
}

func Load(root string, defaults config.Defaults) (Workspace, error) {
	cfg, err := config.LoadWorkspace(WorksetFile(root))
	if err != nil {
		return Workspace{}, err
	}

	state, err := loadState(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			branch := WorkspaceBranchName(cfg.Name)
			if branch == "" {
				branch = defaults.BaseBranch
			}
			state = State{CurrentBranch: branch}
			if err := saveState(root, state); err != nil {
				return Workspace{}, err
			}
		} else {
			return Workspace{}, err
		}
	}

	desiredBranch := WorkspaceBranchName(cfg.Name)
	if desiredBranch == "" {
		desiredBranch = defaults.BaseBranch
	}
	if desiredBranch != "" && state.CurrentBranch != desiredBranch && !UseBranchDirs(root) {
		state = State{CurrentBranch: desiredBranch}
		if err := saveState(root, state); err != nil {
			return Workspace{}, err
		}
	}

	return Workspace{
		Root:   root,
		Config: cfg,
		State:  state,
	}, nil
}

func UseBranchDirs(root string) bool {
	info, err := os.Stat(WorktreesPath(root))
	if err != nil {
		return false
	}
	return info.IsDir()
}

func saveState(root string, state State) error {
	if err := os.MkdirAll(filepath.Dir(StatePath(root)), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(StatePath(root), data, 0o644)
}

func SaveState(root string, state State) error {
	return saveState(root, state)
}

func EnsureSessionState(state *State) {
	if state.Sessions == nil {
		state.Sessions = map[string]SessionState{}
	}
}

func loadState(root string) (State, error) {
	data, err := os.ReadFile(StatePath(root))
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, err
	}
	if state.CurrentBranch == "" {
		return State{}, errors.New("current_branch missing in state.json")
	}
	return state, nil
}

func LoadState(root string) (State, error) {
	return loadState(root)
}

func WorktreeDirName(branch string) string {
	if branch == "" {
		return "branch"
	}
	return strings.ReplaceAll(branch, "/", "__")
}

func WorkspaceDirName(name string) string {
	if name == "" {
		return ""
	}
	dir := strings.ReplaceAll(name, "/", "__")
	if sep := string(os.PathSeparator); sep != "/" {
		dir = strings.ReplaceAll(dir, sep, "__")
	}
	return dir
}

// WorkspaceBranchName derives a git-safe branch name from the workspace name.
// Valid git branch names are preserved as-is to avoid changing existing behavior.
func WorkspaceBranchName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	if isGitSafeBranchName(trimmed) {
		return trimmed
	}
	sanitized := sanitizeWorkspaceBranchName(trimmed)
	if sanitized == "" {
		return "workspace-" + shortHash(trimmed)
	}
	if sanitized == trimmed {
		return sanitized
	}
	return sanitized + "-" + shortHash(trimmed)
}

func BranchNameFromDir(name string) string {
	if name == "" {
		return ""
	}
	return strings.ReplaceAll(name, "__", "/")
}

func WriteBranchMeta(root, branch string) error {
	if root == "" || branch == "" {
		return nil
	}
	path := BranchMetaPath(root, branch)
	return os.WriteFile(path, []byte(branch), 0o644)
}

func ReadBranchMeta(dir string) (string, bool, error) {
	path := filepath.Join(dir, branchMetaFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	name := strings.TrimSpace(string(data))
	if name == "" {
		return "", false, nil
	}
	return name, true, nil
}

func sanitizeWorktreeName(branch string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range branch {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "branch"
	}
	return s
}

func sanitizeWorkspaceBranchName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '/':
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteByte('-')
		default:
			b.WriteByte('-')
		}
	}
	candidate := strings.Trim(b.String(), "/-")
	if candidate == "" {
		return ""
	}
	rawParts := strings.Split(candidate, "/")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		part = strings.Trim(part, "-")
		if part == "" {
			continue
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "/")
}

func isGitSafeBranchName(name string) bool {
	if hasInvalidGitBranchShape(name) {
		return false
	}
	if hasInvalidGitBranchParts(strings.Split(name, "/")) {
		return false
	}
	for _, r := range name {
		if isInvalidGitBranchRune(r) {
			return false
		}
	}
	return true
}

func hasInvalidGitBranchShape(name string) bool {
	if name == "" || name == "@" || strings.HasPrefix(name, "-") {
		return true
	}
	if strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") || strings.Contains(name, "//") {
		return true
	}
	if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return true
	}
	return strings.Contains(name, "..") || strings.Contains(name, "@{")
}

func hasInvalidGitBranchParts(parts []string) bool {
	for _, part := range parts {
		if part == "" {
			return true
		}
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return true
		}
	}
	return false
}

func isInvalidGitBranchRune(r rune) bool {
	if r < 32 || r == 127 || unicode.IsSpace(r) {
		return true
	}
	switch r {
	case '~', '^', ':', '?', '*', '[', '\\':
		return true
	default:
		return false
	}
}

func shortHash(input string) string {
	sum := sha1.Sum([]byte(input))
	return hex.EncodeToString(sum[:4])
}
