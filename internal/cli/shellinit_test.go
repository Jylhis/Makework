package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// writeHookFile captures `mw init <shell>` output to a file and returns its path.
func writeHookFile(t *testing.T, shell, dir string) string {
	t.Helper()
	out, err := captureOutput(t, "init", shell)
	if err != nil {
		t.Fatalf("mw init %s: %v (out: %s)", shell, err, out)
	}
	path := filepath.Join(dir, "mw-hook."+shell)
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		t.Fatalf("write hook file: %v", err)
	}
	return path
}

func TestInitZshHookIsIdempotent(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not available")
	}
	dir := t.TempDir()
	hook := writeHookFile(t, "zsh", dir)

	// Source twice; ensure chpwd_functions only contains one entry.
	script := "source " + shellQuote(hook) + "\nsource " + shellQuote(hook) + "\nprint -r -- \"${chpwd_functions[@]}\""
	cmd := exec.Command("zsh", "-f", "-c", script)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("zsh: %v", err)
	}
	entries := strings.Fields(strings.TrimSpace(string(out)))
	count := 0
	for _, e := range entries {
		if e == "_makework_hook" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 _makework_hook in chpwd_functions, got %d: %v", count, entries)
	}
}

func TestInitBashHookIsIdempotent(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	dir := t.TempDir()
	hook := writeHookFile(t, "bash", dir)

	// Source twice; ensure PROMPT_COMMAND mentions _makework_hook only once.
	script := "source " + shellQuote(hook) + "\nsource " + shellQuote(hook) + "\nprintf '%s\\n' \"$PROMPT_COMMAND\""
	cmd := exec.Command("bash", "--noprofile", "--norc", "-c", script)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("bash: %v", err)
	}
	promptCmd := strings.TrimSpace(string(out))
	if got := strings.Count(promptCmd, "_makework_hook"); got != 1 {
		t.Errorf("expected exactly 1 _makework_hook in PROMPT_COMMAND, got %d in %q", got, promptCmd)
	}
}

func TestInitZshHookActuallyFiresOnChpwd(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not available")
	}
	dir := t.TempDir()
	hook := writeHookFile(t, "zsh", dir)

	// Stub `mw` with a script that writes to a log, shadow it via PATH, source
	// the hook, then cd and confirm the stub ran.
	stubDir := filepath.Join(dir, "stub")
	if err := os.MkdirAll(stubDir, 0o755); err != nil {
		t.Fatalf("mkdir stub: %v", err)
	}
	logPath := filepath.Join(dir, "visits.log")
	stub := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + shellQuote(logPath) + "\n"
	stubPath := filepath.Join(stubDir, "mw")
	if err := os.WriteFile(stubPath, []byte(stub), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	target := t.TempDir()

	script := "export PATH=" + shellQuote(stubDir) + ":$PATH\n" +
		"source " + shellQuote(hook) + "\n" +
		"cd " + shellQuote(target) + "\n" +
		"wait 2>/dev/null\n"
	cmd := exec.Command("zsh", "-f", "-c", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("zsh: %v\n%s", err, out)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("stub log not written — hook did not fire: %v", err)
	}
	if !strings.Contains(string(data), "visit "+target) {
		t.Errorf("expected 'visit %s' in stub log, got %q", target, data)
	}
}
