// Package worksetapi exposes a stable, JSON-friendly API for Workset.
//
// The DTOs in this package are intended to mirror CLI JSON output. External
// callers should treat these shapes as stable, and internal code should avoid
// renaming fields or changing JSON tags without a compatibility plan.
//
// Service provides CRUD-style operations for workspaces, repos, aliases, and
// groups, plus session and exec actions.
//
// Example:
//
//	svc := worksetapi.NewService(worksetapi.Options{})
//	ctx := context.Background()
//	result, err := svc.ListWorkspaces(ctx)
//	if err != nil {
//		// handle error
//	}
//	_ = result
package worksetapi
