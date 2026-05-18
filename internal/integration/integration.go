// Package integration classifies a branch's relationship to a target
// (typically the default branch) so callers can decide whether the
// branch is safe to delete after removing its worktree.
//
// The check runs a five-step ladder, returning at the first match:
//
//  1. StateSameCommit — HEAD(branch) == HEAD(default).
//  2. StateAncestor   — branch is in default's history (e.g. fast-
//     forward merge or rebase).
//  3. StateNoChanges  — three-dot diff (default...branch) is empty;
//     branch has commits but no net tree change since the merge-base.
//  4. StateMergedPID  — every commit unique to branch has a matching
//     git patch-id on default (squash- / cherry-pick-merge case).
//  5. StateDiverged   — none of the above; keeping the branch.
package integration

import (
	"os/exec"
	"strings"

	"github.com/jylhis/makework/internal/repo"
)

// State enumerates how a branch relates to a target branch.
type State string

const (
	StateUnknown    State = "unknown"
	StateSameCommit State = "same_commit"
	StateAncestor   State = "ancestor"
	StateNoChanges  State = "no_changes"
	StateMergedPID  State = "patch_id"
	StateDiverged   State = "diverged"
)

// Check returns the integration State of branch relative to
// defaultBranch in the repo at repoPath. Both refs must resolve.
func Check(repoPath, branch, defaultBranch string) (State, error) {
	branchSHA, err := repo.RunGitCapture("-C", repoPath, "rev-parse", branch)
	if err != nil {
		return StateUnknown, err
	}
	defaultSHA, err := repo.RunGitCapture("-C", repoPath, "rev-parse", defaultBranch)
	if err != nil {
		return StateUnknown, err
	}
	if strings.TrimSpace(branchSHA) == strings.TrimSpace(defaultSHA) {
		return StateSameCommit, nil
	}

	ok, err := repo.IsAncestor(repoPath, branch, defaultBranch)
	if err != nil {
		return StateUnknown, err
	}
	if ok {
		return StateAncestor, nil
	}

	empty, err := diffEmpty(repoPath, defaultBranch, branch)
	if err != nil {
		return StateUnknown, err
	}
	if empty {
		return StateNoChanges, nil
	}

	merged, err := exactPatchesMerged(repoPath, branch, defaultBranch)
	if err != nil {
		return StateUnknown, err
	}
	if merged {
		return StateMergedPID, nil
	}

	return StateDiverged, nil
}

// diffEmpty reports whether `git diff --quiet <a>...<b>` exits zero,
// meaning the three-dot diff has no content.
func diffEmpty(repoPath, a, b string) (bool, error) {
	cmd := exec.Command("git", "-C", repoPath, "diff", "--quiet", a+"..."+b)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// exactPatchesMerged reports whether every commit unique to branch has
// an exact patch-text match on defaultBranch.
func exactPatchesMerged(repoPath, branch, defaultBranch string) (bool, error) {
	branchPatches, err := patchSignatures(repoPath, defaultBranch+".."+branch)
	if err != nil {
		return false, err
	}
	if len(branchPatches) == 0 {
		return false, nil
	}
	defaultPatches, err := patchSignatures(repoPath, branch+".."+defaultBranch)
	if err != nil {
		return false, err
	}
	have := make(map[string]struct{}, len(defaultPatches))
	for _, sig := range defaultPatches {
		have[sig] = struct{}{}
	}
	for _, sig := range branchPatches {
		if _, ok := have[sig]; !ok {
			return false, nil
		}
	}
	return true, nil
}

// patchSignatures returns whitespace-sensitive patch texts for commits in
// revRange, in log order.
func patchSignatures(repoPath, revRange string) ([]string, error) {
	commitList, err := repo.RunGitCapture("-C", repoPath, "rev-list", "--reverse", revRange)
	if err != nil {
		return nil, err
	}
	commits := strings.Fields(commitList)
	if len(commits) == 0 {
		return nil, nil
	}

	sigs := make([]string, 0, len(commits))
	for _, sha := range commits {
		patch, err := repo.RunGitCapture("-C", repoPath, "show", "--pretty=format:", "--no-ext-diff", sha)
		if err != nil {
			return nil, err
		}
		sigs = append(sigs, patch)
	}
	return sigs, nil
}
