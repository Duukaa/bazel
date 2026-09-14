package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beroni/bazel/internal/gh"
)

func TestPrepareFetchesThePRBaseBranch(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "commands.log")
	writeCommand(t, bin, "gh", `
printf 'gh %s\n' "$*" >> "$BAZEL_TEST_LOG"
`)
	writeCommand(t, bin, "git", `
printf 'git %s\n' "$*" >> "$BAZEL_TEST_LOG"
`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BAZEL_TEST_LOG", log)

	ws, err := Prepare(context.Background(), gh.PR{
		Repo:        "acme/api",
		Number:      42,
		BaseRefName: "release/v1.2",
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	defer ws.Remove()

	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	want := "git fetch origin +refs/heads/release/v1.2:refs/remotes/origin/release/v1.2"
	if !strings.Contains(got, want) {
		t.Errorf("base fetch missing:\n%s\nwant %s", got, want)
	}
	if want := "git branch --force release/v1.2 origin/release/v1.2"; !strings.Contains(got, want) {
		t.Errorf("local base branch missing:\n%s\nwant %s", got, want)
	}
}

func TestPrepareSkipsEmptyBaseBranch(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "commands.log")
	writeCommand(t, bin, "gh", `printf 'gh %s\n' "$*" >> "$BAZEL_TEST_LOG"`)
	writeCommand(t, bin, "git", `printf 'git %s\n' "$*" >> "$BAZEL_TEST_LOG"`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BAZEL_TEST_LOG", log)

	ws, err := Prepare(context.Background(), gh.PR{Repo: "acme/api", Number: 42})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	defer ws.Remove()
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "git fetch") {
		t.Errorf("empty base should not fetch:\n%s", data)
	}
}

func TestPrepareKeepsExistingBaseWhenFetchFails(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "commands.log")
	writeCommand(t, bin, "gh", `printf 'gh %s\n' "$*" >> "$BAZEL_TEST_LOG"`)
	writeCommand(t, bin, "git", `
printf 'git %s\n' "$*" >> "$BAZEL_TEST_LOG"
case " $* " in
  *" fetch "*) echo 'could not fetch' >&2; exit 1 ;;
  *" rev-parse --verify --quiet origin/release/v1.2 "*) exit 0 ;;
  *" rev-parse --abbrev-ref HEAD "*) printf 'feature\n'; exit 0 ;;
esac
`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BAZEL_TEST_LOG", log)

	ws, err := Prepare(context.Background(), gh.PR{
		Repo:        "acme/api",
		Number:      42,
		BaseRefName: "release/v1.2",
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	defer ws.Remove()
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "git branch --force release/v1.2 origin/release/v1.2") {
		t.Errorf("should still create local base after a failed fetch:\n%s", got)
	}
}

func TestPrepareSkipsLocalBaseWhenHeadIsTheBase(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "commands.log")
	writeCommand(t, bin, "gh", `printf 'gh %s\n' "$*" >> "$BAZEL_TEST_LOG"`)
	writeCommand(t, bin, "git", `
printf 'git %s\n' "$*" >> "$BAZEL_TEST_LOG"
case " $* " in
  *" rev-parse --abbrev-ref HEAD "*) printf 'release/v1.2\n'; exit 0 ;;
esac
`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BAZEL_TEST_LOG", log)

	ws, err := Prepare(context.Background(), gh.PR{
		Repo:        "acme/api",
		Number:      42,
		BaseRefName: "release/v1.2",
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	defer ws.Remove()
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "git branch --force") {
		t.Errorf("should not force-create base when it is HEAD:\n%s", data)
	}
}

func writeCommand(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
