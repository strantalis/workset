package main

const (
	EventHooksProgress = "hooks:progress"

	EventGitHubOperation = "github:operation"

	EventSessiondRestarted = "sessiond:restarted"

	EventTerminalData          = "terminal:data"
	EventTerminalBootstrap     = "terminal:bootstrap"
	EventTerminalBootstrapDone = "terminal:bootstrap_done"
	EventTerminalLifecycle     = "terminal:lifecycle"
	EventTerminalModes         = "terminal:modes"
	EventTerminalKitty         = "terminal:kitty"

	EventWorkspacePopoutOpened = "workspace:popout-opened"
	EventWorkspacePopoutClosed = "workspace:popout-closed"

	EventRepoDiffSummary      = "repodiff:summary"
	EventRepoDiffLocalSummary = "repodiff:local-summary"
	EventRepoDiffLocalStatus  = "repodiff:local-status"
	EventRepoDiffPRStatus     = "repodiff:pr-status"
	EventRepoDiffPRReviews    = "repodiff:pr-reviews"
)
