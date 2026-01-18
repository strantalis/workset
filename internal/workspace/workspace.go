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

	"github.com/strantalis/workset/internal/config"
)

const (
	worksetFileName = "workset.yaml"
	stateFileName   = "state.json"
	branchMetaFile  = ".workset-branch"
)

type Workspace struct {
	Root   string
	Config config.WorkspaceConfig
	State  State
}

type State struct {
	CurrentBranch string                  `json:"current_branch"`
	Sessions      map[string]SessionState `json:"sessions,omitempty"`
}

type SessionState struct {
	Backend      string   `json:"backend,omitempty"`
	Name         string   `json:"name,omitempty"`
	Command      []string `json:"command,omitempty"`
	StartedAt    string   `json:"started_at,omitempty"`
	LastAttached string   `json:"last_attached,omitempty"`
}

func WorksetFile(root string) string {
	return filepath.Join(root, worksetFileName)
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

	branch := name
	if branch == "" {
		branch = defaults.BaseBranch
	}
	state := State{CurrentBranch: branch}
	if err := saveState(root, state); err != nil {
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
			branch := cfg.Name
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

	if cfg.Name != "" && state.CurrentBranch != cfg.Name && !UseBranchDirs(root) {
		state = State{CurrentBranch: cfg.Name}
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

func shortHash(input string) string {
	sum := sha1.Sum([]byte(input))
	return hex.EncodeToString(sum[:4])
}
