package agent

import (
	"os"
	"path/filepath"

	"github.com/beroni/bazel/internal/config"
)

// grokRuntimeConfig is the ~/.grok/config.toml equivalent for a review
// process. `grok --prompt-file` is the full interactive agent in headless
// mode: it loads the user's MCP servers, Claude plugins, and skills. Codex
// `exec` does not. This file is the closest Bazel can get to
// `codex exec --ignore-user-config`.
const grokRuntimeConfig = `[models]
default = "grok-4.6"
default_reasoning_effort = "medium"

[plugins]
enabled = []

[skills]
ignore = ["~/.agents/skills", "~/.claude/skills", "~/.claude/plugins", "~/.claude/commands"]

[compat.claude]
agents = false
hooks = false
mcps = false
rules = false
skills = false

[compat.cursor]
agents = false
hooks = false
mcps = false
rules = false
skills = false

[compat.codex]
hooks = false
skills = false
`

var grokCompatOff = []string{
	"GROK_CLAUDE_SKILLS_ENABLED",
	"GROK_CLAUDE_AGENTS_ENABLED",
	"GROK_CLAUDE_HOOKS_ENABLED",
	"GROK_CLAUDE_MCPS_ENABLED",
	"GROK_CLAUDE_RULES_ENABLED",
	"GROK_CURSOR_SKILLS_ENABLED",
	"GROK_CURSOR_AGENTS_ENABLED",
	"GROK_CURSOR_HOOKS_ENABLED",
	"GROK_CURSOR_MCPS_ENABLED",
	"GROK_CURSOR_RULES_ENABLED",
}

// isolateGrok points a grok-stream step at a Bazel-owned GROK_HOME so the
// review does not inherit the user's interactive MCP/plugin/skill set.
// GROK_HOME already set on the step wins (tests and explicit config).
// The returned path is a unique runtime created for this invocation; the
// caller should remove it when the process exits. Empty means none was created.
func isolateGrok(step *config.ResolvedAgent) (string, error) {
	if step.Format != "grok-stream" {
		return "", nil
	}

	env := make(map[string]string, len(step.Env)+len(grokCompatOff)+2)
	for key, value := range step.Env {
		env[key] = value
	}
	step.Env = env

	created := ""
	if _, ok := env["GROK_HOME"]; !ok {
		dir, err := config.Dir()
		if err != nil {
			return "", err
		}
		parent := filepath.Join(dir, "grok-runtime")
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return "", err
		}
		runtime, err := os.MkdirTemp(parent, "")
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(runtime, "config.toml"), []byte(grokRuntimeConfig), 0o644); err != nil {
			os.RemoveAll(runtime)
			return "", err
		}
		if err := linkGrokAuth(runtime); err != nil {
			os.RemoveAll(runtime)
			return "", err
		}
		env["GROK_HOME"] = runtime
		created = runtime
	}
	if _, ok := env["GROK_MEMORY"]; !ok {
		env["GROK_MEMORY"] = "0"
	}
	for _, key := range grokCompatOff {
		if _, ok := env[key]; !ok {
			env[key] = "false"
		}
	}
	return created, nil
}

func linkGrokAuth(runtime string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	src := filepath.Join(home, ".grok", "auth.json")
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	dst := filepath.Join(runtime, "auth.json")
	_ = os.Remove(dst)
	if err := os.Symlink(src, dst); err == nil {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}
