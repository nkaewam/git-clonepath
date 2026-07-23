package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunForwardsArgumentsStreamsAndDestination(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("v1 supports macOS and Linux")
	}

	root := t.TempDir()
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "args")
	writeFakeGit(t, bin)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CLONEPATH_TEST_ROOT", root)
	t.Setenv("CLONEPATH_TEST_LOG", log)
	t.Setenv("CLONEPATH_TEST_EXIT", "0")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"--depth", "1", "--branch=main", "git@example.com:group/repo.git",
	}, IO{In: strings.NewReader("credential-input\n"), Out: &stdout, Err: &stderr})

	if code != 0 {
		t.Fatalf("Run() = %d, stderr=%s", code, stderr.String())
	}
	gotArgs, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	wantDestination := filepath.Join(root, "example.com", "group", "repo")
	wantArgs := "clone\n--depth\n1\n--branch=main\ngit@example.com:group/repo.git\n" + wantDestination + "\n"
	if string(gotArgs) != wantArgs {
		t.Fatalf("Git arguments:\n%s\nwant:\n%s", gotArgs, wantArgs)
	}
	if stdout.String() != "fake git stdout\ncredential-input\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if stderr.String() != "fake git stderr\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPropagatesGitExitAndCleansEmptyParents(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("v1 supports macOS and Linux")
	}

	root := t.TempDir()
	bin := t.TempDir()
	writeFakeGit(t, bin)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CLONEPATH_TEST_ROOT", root)
	t.Setenv("CLONEPATH_TEST_LOG", filepath.Join(t.TempDir(), "args"))
	t.Setenv("CLONEPATH_TEST_EXIT", "42")

	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"https://example.com/a/b/repo.git"}, IO{
		In:  strings.NewReader(""),
		Out: &bytes.Buffer{},
		Err: &stderr,
	})

	if code != 42 {
		t.Fatalf("Run() = %d, want 42; stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "example.com")); !os.IsNotExist(err) {
		t.Fatalf("created hierarchy was not cleaned: %v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("clone root was removed: %v", err)
	}
}

func TestRunRejectsExistingDestinationBeforeGitClone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("v1 supports macOS and Linux")
	}

	root := t.TempDir()
	destination := filepath.Join(root, "example.com", "group", "repo")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "args")
	writeFakeGit(t, bin)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CLONEPATH_TEST_ROOT", root)
	t.Setenv("CLONEPATH_TEST_LOG", log)
	t.Setenv("CLONEPATH_TEST_EXIT", "0")

	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"https://example.com/group/repo.git"}, IO{Err: &stderr})
	if code != 1 || !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("Run() = %d, stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatalf("git clone was invoked: %v", err)
	}
}

func TestRunUsageAndInvalidRemote(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{name: "usage", want: "usage:"},
		{name: "local remote", args: []string{"./repo"}, want: "not a hosted remote"},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer
			if code := Run(context.Background(), tt.args, IO{Err: &stderr}); code != 2 {
				t.Fatalf("Run() = %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr = %q, want containing %q", stderr.String(), tt.want)
			}
		})
	}
}

func writeFakeGit(t *testing.T, directory string) {
	t.Helper()
	script := `#!/bin/sh
if [ "$1" = "config" ]; then
  printf '%s\n' "$CLONEPATH_TEST_ROOT"
  exit 0
fi
: > "$CLONEPATH_TEST_LOG"
for arg in "$@"; do
  printf '%s\n' "$arg" >> "$CLONEPATH_TEST_LOG"
done
printf 'fake git stdout\n'
printf 'fake git stderr\n' >&2
cat
exit "${CLONEPATH_TEST_EXIT:-0}"
`
	path := filepath.Join(directory, "git")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
