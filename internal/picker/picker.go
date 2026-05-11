// Package picker presents a small list of items to the user and
// returns the chosen entry. When `fzf` is on PATH it is used for a
// fuzzy-filterable TUI; otherwise the picker falls back to a numbered
// stdin prompt.
//
// This is intentionally minimal — no native TUI dependency, no preview
// panes, no multi-select. It exists so commands like `mw go` (no args)
// can offer "pick a worktree" without pulling in a bubbletea-sized
// dependency surface.
package picker

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// Item is one selectable entry. Label is the displayed text, Sub a
// short hint (rendered after a tab when non-empty), Value an opaque
// payload the caller attaches.
type Item struct {
	Label string
	Sub   string
	Value any
}

// ErrCancelled means the user dismissed the picker without choosing.
var ErrCancelled = errors.New("picker: cancelled")

// ErrNoItems means the caller passed a zero-length slice.
var ErrNoItems = errors.New("picker: no items to choose from")

// Pick presents items and returns the selection. If fzf is on PATH and
// out is a terminal it is used; otherwise a numbered stdin prompt is
// shown.
func Pick(items []Item, title string, in io.Reader, out io.Writer) (Item, error) {
	if len(items) == 0 {
		return Item{}, ErrNoItems
	}
	if hasFzf() {
		if got, err := pickViaFzf(items, title); err == nil {
			return got, nil
		} else if !errors.Is(err, errFzfUnavailable) {
			return Item{}, err
		}
	}
	return pickViaStdin(items, title, in, out)
}

var errFzfUnavailable = errors.New("fzf not available")

func hasFzf() bool {
	_, err := exec.LookPath("fzf")
	return err == nil
}

// pickViaFzf pipes the items into fzf and parses the selected line
// back into an Item. The line format is "<index>\t<label>\t<sub>"
// so that we can recover the index after fzf returns the selected
// line verbatim.
func pickViaFzf(items []Item, title string) (Item, error) {
	args := []string{"--with-nth=2..", "--delimiter=\t", "--ansi"}
	if title != "" {
		args = append(args, "--header="+title)
	}
	cmd := exec.Command("fzf", args...)

	var stdin strings.Builder
	for i, it := range items {
		fmt.Fprintf(&stdin, "%d\t%s\t%s\n", i, it.Label, it.Sub)
	}
	cmd.Stdin = strings.NewReader(stdin.String())

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
			return Item{}, ErrCancelled
		}
		return Item{}, fmt.Errorf("fzf: %w", err)
	}
	line := strings.TrimRight(string(out), "\n")
	idx, err := strconv.Atoi(strings.SplitN(line, "\t", 2)[0])
	if err != nil || idx < 0 || idx >= len(items) {
		return Item{}, fmt.Errorf("fzf returned bad index %q", line)
	}
	return items[idx], nil
}

// pickViaStdin prints a numbered list and reads a 1-based index from
// in. Returns ErrCancelled on empty input.
func pickViaStdin(items []Item, title string, in io.Reader, out io.Writer) (Item, error) {
	if title != "" {
		fmt.Fprintln(out, title)
	}
	for i, it := range items {
		if it.Sub != "" {
			fmt.Fprintf(out, "  %d. %s\t%s\n", i+1, it.Label, it.Sub)
		} else {
			fmt.Fprintf(out, "  %d. %s\n", i+1, it.Label)
		}
	}
	fmt.Fprintf(out, "Pick [1-%d]: ", len(items))

	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		return Item{}, ErrCancelled
	}
	raw := strings.TrimSpace(scanner.Text())
	if raw == "" {
		return Item{}, ErrCancelled
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return Item{}, fmt.Errorf("not a number: %q", raw)
	}
	if n < 1 || n > len(items) {
		return Item{}, fmt.Errorf("out of range: %d (have 1..%d)", n, len(items))
	}
	return items[n-1], nil
}
