package worksetapi

import "context"

// ReloadLoginEnv re-reads the login shell environment and applies changes.
func (s *Service) ReloadLoginEnv(ctx context.Context) (EnvSnapshotResultJSON, error) {
	return reloadLoginEnv(ctx)
}
