package hooks

type HookPhase string

const (
	HookPhaseStarted  HookPhase = "started"
	HookPhaseFinished HookPhase = "finished"
)

// HookProgress reports hook execution lifecycle updates.
type HookProgress struct {
	Phase         HookPhase
	Event         Event
	HookID        string
	WorkspaceName string
	RepoName      string
	WorktreePath  string
	Reason        string
	Status        RunStatus
	LogPath       string
	Err           error
}

// RunObserver can receive live hook progress events.
type RunObserver interface {
	OnHookProgress(progress HookProgress)
}
