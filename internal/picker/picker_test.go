package picker

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestPickStdinFallbackSelectsByNumber: with no TTY/fzf, the picker
// prints a numbered list and reads the chosen number from in.
func TestPickStdinFallbackSelectsByNumber(t *testing.T) {
	items := []Item{
		{Label: "alpha", Value: 1},
		{Label: "beta", Value: 2},
		{Label: "gamma", Value: 3},
	}
	var out bytes.Buffer
	in := strings.NewReader("2\n")

	got, err := pickViaStdin(items, "pick one", in, &out)
	if err != nil {
		t.Fatalf("pickViaStdin: %v", err)
	}
	if got.Label != "beta" {
		t.Errorf("got %q; want beta", got.Label)
	}
	if !strings.Contains(out.String(), "1. alpha") {
		t.Errorf("expected numbered list in output:\n%s", out.String())
	}
}

// TestPickStdinFallbackRejectsOutOfRange: invalid number → error.
func TestPickStdinFallbackRejectsOutOfRange(t *testing.T) {
	items := []Item{{Label: "only"}}
	var out bytes.Buffer

	for _, input := range []string{"0\n", "42\n", "not-a-number\n"} {
		t.Run("input="+strings.TrimSpace(input), func(t *testing.T) {
			_, err := pickViaStdin(items, "", strings.NewReader(input), &out)
			if err == nil {
				t.Errorf("expected error for input %q", input)
			}
		})
	}
}

// TestPickStdinFallbackCancellation: empty input or EOF cancels.
func TestPickStdinFallbackCancellation(t *testing.T) {
	items := []Item{{Label: "x"}}
	var out bytes.Buffer
	_, err := pickViaStdin(items, "", strings.NewReader(""), &out)
	if !errors.Is(err, ErrCancelled) {
		t.Errorf("expected ErrCancelled on empty input, got %v", err)
	}
}

// TestPickEmptyItems: zero items → ErrNoItems.
func TestPickEmptyItems(t *testing.T) {
	var out bytes.Buffer
	_, err := Pick(nil, "", strings.NewReader(""), &out)
	if !errors.Is(err, ErrNoItems) {
		t.Errorf("expected ErrNoItems, got %v", err)
	}
}
