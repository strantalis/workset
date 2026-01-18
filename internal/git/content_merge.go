package git

import (
	"errors"
	"fmt"

	ggit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/storer"
	"github.com/go-git/go-git/v6/utils/merkletrie"
)

func (c GoGitClient) IsContentMerged(repoPath, branchRef, baseRef string) (bool, error) {
	if repoPath == "" {
		return false, errors.New("repo path required")
	}
	if branchRef == "" || baseRef == "" {
		return false, errors.New("branch and base refs required")
	}
	repo, err := ggit.PlainOpenWithOptions(repoPath, &ggit.PlainOpenOptions{
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		return false, err
	}

	branchRefName := plumbing.ReferenceName(branchRef)
	baseRefName := plumbing.ReferenceName(baseRef)
	branch, err := repo.Reference(branchRefName, true)
	if err != nil {
		return false, err
	}
	base, err := repo.Reference(baseRefName, true)
	if err != nil {
		return false, err
	}
	branchCommit, err := repo.CommitObject(branch.Hash())
	if err != nil {
		return false, err
	}
	baseCommit, err := repo.CommitObject(base.Hash())
	if err != nil {
		return false, err
	}
	if branchCommit.TreeHash == baseCommit.TreeHash {
		return true, nil
	}

	mergeBases, err := branchCommit.MergeBase(baseCommit)
	if err != nil {
		return false, err
	}
	if len(mergeBases) == 0 {
		return false, nil
	}

	baseTree, err := baseCommit.Tree()
	if err != nil {
		return false, err
	}

	var lastErr error
	for _, mergeBase := range mergeBases {
		merged, err := changesAppliedInBase(mergeBase, branchCommit, baseTree)
		if err != nil {
			lastErr = err
			continue
		}
		if merged {
			return true, nil
		}
	}
	merged, err := treeSeenInHistory(baseCommit, mergeBases, branchCommit.TreeHash)
	if err != nil {
		if lastErr != nil {
			return false, lastErr
		}
		return false, err
	}
	if merged {
		return true, nil
	}
	if lastErr != nil {
		return false, lastErr
	}
	return false, nil
}

func changesAppliedInBase(mergeBase, branchCommit *object.Commit, baseTree *object.Tree) (bool, error) {
	if mergeBase == nil || branchCommit == nil || baseTree == nil {
		return false, errors.New("merge base, branch commit, and base tree required")
	}
	mergeTree, err := mergeBase.Tree()
	if err != nil {
		return false, err
	}
	branchTree, err := branchCommit.Tree()
	if err != nil {
		return false, err
	}
	changes, err := mergeTree.Diff(branchTree)
	if err != nil {
		return false, err
	}
	if len(changes) == 0 {
		return true, nil
	}
	for _, change := range changes {
		action, err := change.Action()
		if err != nil {
			return false, err
		}
		switch action {
		case merkletrie.Insert, merkletrie.Modify:
			path := change.To.Name
			baseEntry, err := baseTree.FindEntry(path)
			if err != nil {
				if errors.Is(err, object.ErrEntryNotFound) {
					return false, nil
				}
				return false, err
			}
			if baseEntry.Hash != change.To.TreeEntry.Hash || baseEntry.Mode != change.To.TreeEntry.Mode {
				return false, nil
			}
		case merkletrie.Delete:
			path := change.From.Name
			if _, err := baseTree.FindEntry(path); err == nil {
				return false, nil
			} else if !errors.Is(err, object.ErrEntryNotFound) {
				return false, err
			}
		default:
			return false, fmt.Errorf("unknown change action: %v", action)
		}
	}
	return true, nil
}

func treeSeenInHistory(baseCommit *object.Commit, mergeBases []*object.Commit, treeHash plumbing.Hash) (bool, error) {
	if baseCommit == nil {
		return false, errors.New("base commit required")
	}
	if treeHash.IsZero() {
		return false, errors.New("tree hash required")
	}
	ignore := make([]plumbing.Hash, 0, len(mergeBases))
	for _, mergeBase := range mergeBases {
		if mergeBase != nil {
			ignore = append(ignore, mergeBase.Hash)
		}
	}
	iter := object.NewCommitPreorderIter(baseCommit, nil, ignore)
	defer iter.Close()

	found := false
	err := iter.ForEach(func(commit *object.Commit) error {
		if commit.TreeHash == treeHash {
			found = true
			return storer.ErrStop
		}
		return nil
	})
	if err != nil && !errors.Is(err, storer.ErrStop) {
		return false, err
	}
	return found, nil
}
