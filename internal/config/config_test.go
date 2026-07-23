package config

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRootFromGitConfig(t *testing.T) {
	git := requireGit(t)
	home := t.TempDir()
	root := filepath.Join(home, "Developer")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(git, "config", "--file", filepath.Join(home, ".gitconfig"), "clonepath.root", "~/Developer")
	cmd.Env = append(os.Environ(), "HOME="+home)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("configure Git: %v: %s", err, output)
	}

	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	got, err := Root(context.Background(), git)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("Root() = %q, want %q", got, root)
	}
}

func TestRootRejectsMissingRelativeMissingPathAndFile(t *testing.T) {
	git := requireGit(t)

	tests := []struct {
		name       string
		configured bool
		value      string
		setup      func(t *testing.T, home string) string
		want       string
	}{
		{name: "missing config", want: "git config --global clonepath.root"},
		{name: "relative", configured: true, value: "Developer", want: "relative"},
		{name: "missing directory", configured: true, value: "/path/that/does/not/exist/clonepath", want: "does not exist"},
		{
			name:       "file",
			configured: true,
			setup: func(t *testing.T, home string) string {
				path := filepath.Join(home, "not-a-directory")
				if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			want: "not a directory",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			configFile := filepath.Join(home, ".gitconfig")
			value := ""
			if tt.setup != nil {
				value = tt.setup(t, home)
			} else {
				value = tt.value
			}
			if tt.configured {
				cmd := exec.Command(git, "config", "--file", configFile, "clonepath.root", value)
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("configure Git: %v: %s", err, output)
				}
			}

			t.Setenv("HOME", home)
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			t.Setenv("GIT_CONFIG_GLOBAL", configFile)
			_, err := Root(context.Background(), git)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Root() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestRootHonorsGitCommandScope(t *testing.T) {
	git := requireGit(t)
	home := t.TempDir()
	root := filepath.Join(home, "command-scope-root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "missing.gitconfig"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "clonepath.root")
	t.Setenv("GIT_CONFIG_VALUE_0", root)

	got, err := Root(context.Background(), git)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("Root() = %q, want command-scoped value %q", got, root)
	}
}

func requireGit(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("v1 supports macOS and Linux")
	}
	path, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	return path
}
