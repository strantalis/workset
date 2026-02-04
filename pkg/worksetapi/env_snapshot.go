package worksetapi

// EnvSnapshotResultJSON reports whether a login-shell env snapshot updated values.
type EnvSnapshotResultJSON struct {
	Updated     bool     `json:"updated"`
	AppliedKeys []string `json:"appliedKeys,omitempty"`
}
