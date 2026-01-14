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
)

type Workspace struct {
	Root   string
	Config config.WorkspaceConfig
	State  State
}

type State struct {
	CurrentBranch string `json:"current_branch"`
}

func WorksetFile(root string) string {
	return filepath.Join(root, worksetFileName)
}

func StatePath(root string) string {
	return filepath.Join(root, ".workset", stateFileName)
}

func GitDirsPath(root string) string {
	return filepath.Join(root, ".workset", "gitdirs")
}

func RepoGitDirPath(root, repoName string) string {
	return filepath.Join(GitDirsPath(root), repoName+".git")
}

func LogsPath(root string) string {
	return filepath.Join(root, ".workset", "logs")
}

func BranchesPath(root string) string {
	return filepath.Join(root, "branches")
}

func BranchPath(root, branch string) string {
	return filepath.Join(BranchesPath(root), branch)
}

func RepoWorktreePath(root, branch, repoDir string) string {
	return filepath.Join(BranchPath(root, branch), repoDir)
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
	if err := os.MkdirAll(GitDirsPath(root), 0o755); err != nil {
		return Workspace{}, err
	}
	if err := os.MkdirAll(LogsPath(root), 0o755); err != nil {
		return Workspace{}, err
	}
	if err := os.MkdirAll(BranchPath(root, defaults.BaseBranch), 0o755); err != nil {
		return Workspace{}, err
	}

	wsConfig := config.WorkspaceConfig{
		Name:  name,
		Repos: []config.RepoConfig{},
	}
	if err := config.SaveWorkspace(WorksetFile(root), wsConfig); err != nil {
		return Workspace{}, err
	}

	state := State{CurrentBranch: defaults.BaseBranch}
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
			state = State{CurrentBranch: defaults.BaseBranch}
			if err := saveState(root, state); err != nil {
				return Workspace{}, err
			}
		} else {
			return Workspace{}, err
		}
	}

	return Workspace{
		Root:   root,
		Config: cfg,
		State:  state,
	}, nil
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
