package destination

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nkaewam/clone-path/internal/remote"
)

func identity() remote.Identity {
	return remote.Identity{Host: "git.example.com", Segments: []string{"group", "nested", "repo"}}
}

func TestPlanAndCreateParents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	plan, err := NewPlan(root, identity())
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "git.example.com", "group", "nested", "repo")
	if plan.Path != want {
		t.Fatalf("Path = %q, want %q", plan.Path, want)
	}
	if _, err := os.Stat(filepath.Dir(want)); !os.IsNotExist(err) {
		t.Fatalf("planning mutated filesystem: stat error = %v", err)
	}

	created, err := plan.CreateParents()
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 3 {
		t.Fatalf("created %d directories, want 3: %v", len(created), created)
	}
	if _, err := os.Stat(filepath.Dir(want)); err != nil {
		t.Fatalf("parent was not created: %v", err)
	}
	if _, err := os.Stat(want); !os.IsNotExist(err) {
		t.Fatalf("clone destination itself should not be created: %v", err)
	}
}

func TestExistingDestinationIsAlwaysRejected(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"empty directory", "file", "symlink"} {
		kind := kind
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "git.example.com", "group", "nested", "repo")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "empty directory":
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatal(err)
				}
			case "file":
				if err := os.WriteFile(path, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(filepath.Join(root, "missing"), path); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := NewPlan(root, identity()); err == nil || !strings.Contains(err.Error(), "already exists") {
				t.Fatalf("NewPlan() error = %v, want already exists", err)
			}
		})
	}
}

func TestSymlinkedRootIsAllowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("v1 supports macOS and Linux")
	}

	base := t.TempDir()
	realRoot := filepath.Join(base, "real")
	if err := os.Mkdir(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "root")
	if err := os.Symlink(realRoot, root); err != nil {
		t.Fatal(err)
	}

	plan, err := NewPlan(root, identity())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.CreateParents(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(realRoot, "git.example.com", "group", "nested")); err != nil {
		t.Fatalf("directories not created through root symlink: %v", err)
	}
}

func TestSymlinkEscapeIsRejectedWithoutMutation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("v1 supports macOS and Linux")
	}

	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "git.example.com")); err != nil {
		t.Fatal(err)
	}

	if _, err := NewPlan(root, identity()); err == nil || !strings.Contains(err.Error(), "outside clone root") {
		t.Fatalf("NewPlan() error = %v, want outside clone root", err)
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("outside directory was mutated: entries=%v err=%v", entries, err)
	}
}

func TestCleanupRemovesOnlyEmptyCreatedDirectories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	plan, err := NewPlan(root, identity())
	if err != nil {
		t.Fatal(err)
	}
	created, err := plan.CreateParents()
	if err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(created[len(created)-1], "concurrent-work")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	errs := Cleanup(created)
	if len(errs) == 0 {
		t.Fatal("Cleanup() should report that a directory is non-empty")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("cleanup removed user data: %v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("cleanup removed root: %v", err)
	}
}

func TestCleanupPreservesPreexistingParents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	host := filepath.Join(root, "git.example.com")
	if err := os.Mkdir(host, 0o755); err != nil {
		t.Fatal(err)
	}
	plan, err := NewPlan(root, identity())
	if err != nil {
		t.Fatal(err)
	}
	created, err := plan.CreateParents()
	if err != nil {
		t.Fatal(err)
	}
	if errs := Cleanup(created); len(errs) != 0 {
		t.Fatalf("Cleanup() errors = %v", errs)
	}
	if _, err := os.Stat(host); err != nil {
		t.Fatalf("cleanup removed preexisting host directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(host, "group")); !os.IsNotExist(err) {
		t.Fatalf("cleanup left a created directory: %v", err)
	}
}
