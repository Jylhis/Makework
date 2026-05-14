// Package refshortcut resolves the pr:N and mr:N branch shortcuts by
// shelling out to gh (GitHub) or glab (GitLab). The number is strictly
// validated as digits-only before being passed to the child process,
// so user-supplied input never reaches a shell.
package refshortcut

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// prRE matches the exact shape "pr:<digits>". Anchored to reject
// extras like "pr:42; rm -rf /".
var prRE = regexp.MustCompile(`^pr:(\d+)$`)

// mrRE matches "mr:<digits>".
var mrRE = regexp.MustCompile(`^mr:(\d+)$`)

// Resolve returns the branch name for a pr:N or mr:N shortcut. When
// query is not a recognised shortcut, ok is false and err is nil —
// callers should fall through to normal branch resolution.
//
// When slug ("owner/name") is non-empty it is passed via --repo so
// gh / glab work even without a local checkout. Otherwise cwd is used
// to disambiguate the active repo; pass "" to inherit the process cwd.
func Resolve(query, cwd, slug string) (branch string, ok bool, err error) {
	if m := prRE.FindStringSubmatch(query); m != nil {
		b, err := resolvePR(m[1], cwd, slug)
		return b, true, err
	}
	if m := mrRE.FindStringSubmatch(query); m != nil {
		b, err := resolveMR(m[1], cwd, slug)
		return b, true, err
	}
	return "", false, nil
}

func resolvePR(number, cwd, slug string) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", errors.New("pr:N shortcut requires gh on PATH (install: https://cli.github.com)")
	}
	args := []string{"pr", "view", number, "--json", "headRefName"}
	if slug != "" {
		args = append(args, "--repo", slug)
	}
	out, err := runIn(cwd, "gh", args...)
	if err != nil {
		return "", fmt.Errorf("gh pr view %s: %w", number, err)
	}
	var payload struct {
		HeadRefName string `json:"headRefName"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parse gh output: %w", err)
	}
	if payload.HeadRefName == "" {
		return "", fmt.Errorf("gh returned empty headRefName for pr:%s", number)
	}
	return payload.HeadRefName, nil
}

func resolveMR(number, cwd, slug string) (string, error) {
	if _, err := exec.LookPath("glab"); err != nil {
		return "", errors.New("mr:N shortcut requires glab on PATH (install: https://gitlab.com/gitlab-org/cli)")
	}
	args := []string{"mr", "show", number, "--output=json"}
	if slug != "" {
		args = append(args, "--repo", slug)
	}
	out, err := runIn(cwd, "glab", args...)
	if err != nil {
		return "", fmt.Errorf("glab mr show %s: %w", number, err)
	}
	var payload struct {
		SourceBranch string `json:"source_branch"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parse glab output: %w", err)
	}
	if payload.SourceBranch == "" {
		return "", fmt.Errorf("glab returned empty source_branch for mr:%s", number)
	}
	return payload.SourceBranch, nil
}

func runIn(cwd, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("exit %d: %s",
				exitErr.ExitCode(), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	return out, nil
}
