package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beroni/bazel/internal/config"
)

func TestIsolateGrokSkipsOtherFormats(t *testing.T) {
	step := config.ResolvedAgent{Format: "codex-json"}
	created, err := isolateGrok(&step)
	if err != nil {
		t.Fatal(err)
	}
	if created != "" {
		t.Errorf("codex-json created a grok runtime: %q", created)
	}
	if step.Env["GROK_HOME"] != "" {
		t.Errorf("codex-json should not get a GROK_HOME: %v", step.Env)
	}
}

func TestIsolateGrokRespectsExistingHome(t *testing.T) {
	step := config.ResolvedAgent{
		Format: "grok-stream",
		Env:    map[string]string{"GROK_HOME": "/already/set"},
	}
	created, err := isolateGrok(&step)
	if err != nil {
		t.Fatal(err)
	}
	if created != "" {
		t.Errorf("explicit GROK_HOME still created a runtime: %q", created)
	}
	if step.Env["GROK_HOME"] != "/already/set" {
		t.Errorf("explicit GROK_HOME was overwritten: %v", step.Env)
	}
	if step.Env["GROK_MEMORY"] != "0" {
		t.Errorf("GROK_MEMORY = %q, want 0", step.Env["GROK_MEMORY"])
	}
}

func TestIsolateGrokWritesRuntimeHome(t *testing.T) {
	root := t.TempDir()
	t.Setenv("BAZEL_HOME", root)

	step := config.ResolvedAgent{Format: "grok-stream"}
	created, err := isolateGrok(&step)
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(root, "grok-runtime")
	if created == "" || !strings.HasPrefix(created, parent+string(os.PathSeparator)) {
		t.Fatalf("created runtime = %q, want under %q", created, parent)
	}
	if step.Env["GROK_HOME"] != created {
		t.Fatalf("GROK_HOME = %q, want %q", step.Env["GROK_HOME"], created)
	}
	if step.Env["GROK_MEMORY"] != "0" {
		t.Errorf("GROK_MEMORY = %q, want 0", step.Env["GROK_MEMORY"])
	}
	body, err := os.ReadFile(filepath.Join(created, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, needle := range []string{
		"default_reasoning_effort = \"medium\"",
		"skills = false",
		"mcps = false",
		"~/.agents/skills",
	} {
		if !strings.Contains(text, needle) {
			t.Errorf("runtime config missing %q:\n%s", needle, text)
		}
	}
	if step.Env["GROK_CLAUDE_MCPS_ENABLED"] != "false" {
		t.Errorf("claude MCP compat should be off: %v", step.Env)
	}
}

func TestIsolateGrokGivesEachInvocationItsOwnHome(t *testing.T) {
	root := t.TempDir()
	t.Setenv("BAZEL_HOME", root)

	a := config.ResolvedAgent{Format: "grok-stream"}
	b := config.ResolvedAgent{Format: "grok-stream"}
	homeA, err := isolateGrok(&a)
	if err != nil {
		t.Fatal(err)
	}
	homeB, err := isolateGrok(&b)
	if err != nil {
		t.Fatal(err)
	}
	if homeA == "" || homeA == homeB {
		t.Errorf("concurrent homes collided: %q %q", homeA, homeB)
	}
}

func TestIsolateGrokDoesNotMutateSharedEnvironment(t *testing.T) {
	root := t.TempDir()
	t.Setenv("BAZEL_HOME", root)

	shared := map[string]string{"GROK_MCP_STARTUP_TIMEOUT_SECS": "2"}
	step := config.ResolvedAgent{Format: "grok-stream", Env: shared}
	if _, err := isolateGrok(&step); err != nil {
		t.Fatal(err)
	}
	if _, ok := shared["GROK_HOME"]; ok {
		t.Errorf("shared environment was mutated: %v", shared)
	}
	if _, ok := shared["GROK_CLAUDE_MCPS_ENABLED"]; ok {
		t.Errorf("shared environment was mutated: %v", shared)
	}

	step.Env["ONLY_THIS_INVOCATION"] = "true"
	if _, ok := shared["ONLY_THIS_INVOCATION"]; ok {
		t.Errorf("step environment still aliases shared environment: %v", shared)
	}
}
