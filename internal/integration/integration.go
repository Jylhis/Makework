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
	"bytes"
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

	merged, err := patchIDsMerged(repoPath, branch, defaultBranch)
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

// patchIDsMerged reports whether every commit unique to branch has a
// matching patch-id on defaultBranch.
func patchIDsMerged(repoPath, branch, defaultBranch string) (bool, error) {
	branchIDs, err := patchIDs(repoPath, defaultBranch+".."+branch)
	if err != nil {
		return false, err
	}
	if len(branchIDs) == 0 {
		return false, nil
	}
	defaultIDs, err := patchIDs(repoPath, branch+".."+defaultBranch)
	if err != nil {
		return false, err
	}
	have := make(map[string]struct{}, len(defaultIDs))
	for _, id := range defaultIDs {
		have[id] = struct{}{}
	}
	for _, id := range branchIDs {
		if _, ok := have[id]; !ok {
			return false, nil
		}
	}
	return true, nil
}

// patchIDs returns the patch-id of every commit in revRange, in log
// order. Each git patch-id output line is "<patch-id> <commit-sha>".
func patchIDs(repoPath, revRange string) ([]string, error) {
	logCmd := exec.Command("git", "-C", repoPath, "log", "--reverse", "-p", revRange)
	pidCmd := exec.Command("git", "-C", repoPath, "patch-id")

	pipe, err := logCmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	pidCmd.Stdin = pipe
	var out bytes.Buffer
	pidCmd.Stdout = &out

	if err := pidCmd.Start(); err != nil {
		return nil, err
	}
	if err := logCmd.Run(); err != nil {
		_ = pidCmd.Wait()
		return nil, err
	}
	if err := pidCmd.Wait(); err != nil {
		return nil, err
	}

	var ids []string
	for _, line := range strings.Split(out.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			ids = append(ids, fields[0])
		}
	}
	return ids, nil
}
