package hooks

type Event string

const (
	EventWorktreeCreated Event = "worktree.created"
)

const (
	OnErrorFail = "fail"
	OnErrorWarn = "warn"
)

type Hook struct {
	ID      string            `yaml:"id" json:"id"`
	On      []Event           `yaml:"on" json:"on"`
	Run     []string          `yaml:"run" json:"run"`
	Cwd     string            `yaml:"cwd,omitempty" json:"cwd,omitempty"`
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	OnError string            `yaml:"on_error,omitempty" json:"on_error,omitempty"`
}

type File struct {
	Hooks []Hook `yaml:"hooks" json:"hooks"`
}

type RunResult struct {
	HookID  string
	Status  RunStatus
	LogPath string
	Err     error
}

type RunStatus string

const (
	RunStatusOK      RunStatus = "ok"
	RunStatusFailed  RunStatus = "failed"
	RunStatusSkipped RunStatus = "skipped"
)

type RunReport struct {
	Event   Event
	Results []RunResult
}

type HookFailedError struct {
	HookID  string
	LogPath string
	Err     error
}

func (e HookFailedError) Error() string {
	if e.LogPath == "" {
		return "hook " + e.HookID + " failed"
	}
	return "hook " + e.HookID + " failed (log: " + e.LogPath + ")"
}
