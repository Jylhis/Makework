package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIWorkflowCostGuards(t *testing.T) {
	t.Helper()

	ci := readWorkflow(t, "ci.yml")
	pages := readWorkflow(t, "pages.yml")

	for _, fragment := range []string{
		"group: ${{ github.workflow }}-${{ github.ref }}",
		"cancel-in-progress: true",
		"paths-ignore:",
		"  - '**/*.md'",
		"  - 'docs/**'",
		"cache: true",
		"if: github.event_name != 'pull_request' || github.event.pull_request.draft == false",
	} {
		if !strings.Contains(ci, fragment) {
			t.Fatalf("ci.yml missing fragment %q", fragment)
		}
	}

	if strings.Contains(ci, "group: ci-${{ github.workflow }}-${{ github.ref }}") {
		t.Fatal("ci.yml still uses the old concurrency group prefix")
	}

	if strings.Contains(ci, "if: github.event_name == 'push'") {
		t.Fatal("ci.yml still limits heavy jobs to push events instead of using draft/offload guards")
	}

	if strings.Contains(ci, "nix-build-cross:\n    name: nix build (${{ matrix.system }})\n    if: github.event_name == 'push'") {
		t.Fatal("ci.yml still runs the full cross-platform matrix on every push")
	}

	for _, fragment := range []string{
		"paths:",
		"  - 'docs/**'",
		"  - '.github/workflows/pages.yml'",
		"  - 'flake.nix'",
		"  - 'flake.lock'",
	} {
		if !strings.Contains(pages, fragment) {
			t.Fatalf("pages.yml missing fragment %q", fragment)
		}
	}
}

func readWorkflow(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("..", "..", ".github", "workflows", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}

	return string(content)
}
