// Package destination plans and safely prepares clone destinations.
package destination

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nkaewam/clone-path/internal/remote"
)

// Plan is a validated, read-only destination plan.
type Plan struct {
	Root         string
	ResolvedRoot string
	Path         string
	Parent       string
}

// NewPlan validates that identity derives a new destination contained by root.
// It does not mutate the filesystem.
func NewPlan(root string, identity remote.Identity) (Plan, error) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return Plan{}, fmt.Errorf("resolve clone root %q: %w", root, err)
	}
	resolvedRoot, err = filepath.Abs(resolvedRoot)
	if err != nil {
		return Plan{}, fmt.Errorf("make resolved clone root absolute: %w", err)
	}

	parts := make([]string, 0, len(identity.Segments)+2)
	parts = append(parts, root, identity.Host)
	parts = append(parts, identity.Segments...)
	path := filepath.Join(parts...)
	if !contained(root, path) {
		return Plan{}, fmt.Errorf("derived destination %q escapes clone root %q", path, root)
	}

	if _, err := os.Lstat(path); err == nil {
		return Plan{}, fmt.Errorf("clone destination %q already exists", path)
	} else if !os.IsNotExist(err) {
		return Plan{}, fmt.Errorf("inspect clone destination %q: %w", path, err)
	}

	if err := checkResolvedContainment(resolvedRoot, path); err != nil {
		return Plan{}, err
	}

	return Plan{
		Root:         root,
		ResolvedRoot: resolvedRoot,
		Path:         path,
		Parent:       filepath.Dir(path),
	}, nil
}

// CreateParents creates missing host and namespace directories. It returns only
// directories created by this call, in creation order.
func (p Plan) CreateParents() ([]string, error) {
	relative, err := filepath.Rel(p.Root, p.Parent)
	if err != nil {
		return nil, fmt.Errorf("derive destination parent path: %w", err)
	}
	if relative == "." {
		return nil, nil
	}

	var created []string
	current := p.Root
	for _, segment := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, segment)
		info, err := os.Stat(current)
		switch {
		case err == nil:
			if !info.IsDir() {
				Cleanup(created)
				return nil, fmt.Errorf("destination parent %q is not a directory", current)
			}
		case os.IsNotExist(err):
			if err := os.Mkdir(current, 0o755); err != nil {
				if errors.Is(err, os.ErrExist) {
					info, err = os.Stat(current)
					if err != nil || !info.IsDir() {
						Cleanup(created)
						return nil, fmt.Errorf("destination parent %q appeared but is not a directory", current)
					}
				} else {
					Cleanup(created)
					return nil, fmt.Errorf("create destination parent %q: %w", current, err)
				}
			} else {
				created = append(created, current)
			}
		default:
			Cleanup(created)
			return nil, fmt.Errorf("inspect destination parent %q: %w", current, err)
		}

		resolved, err := filepath.EvalSymlinks(current)
		if err != nil {
			Cleanup(created)
			return nil, fmt.Errorf("resolve destination parent %q: %w", current, err)
		}
		if !contained(p.ResolvedRoot, resolved) {
			Cleanup(created)
			return nil, fmt.Errorf("destination parent %q resolves outside clone root %q", current, p.Root)
		}
	}

	if _, err := os.Lstat(p.Path); err == nil {
		Cleanup(created)
		return nil, fmt.Errorf("clone destination %q already exists", p.Path)
	} else if !os.IsNotExist(err) {
		Cleanup(created)
		return nil, fmt.Errorf("inspect clone destination %q: %w", p.Path, err)
	}

	return created, nil
}

// Cleanup removes created directories in reverse order, stopping safely at any
// directory that is no longer empty. It never receives or removes the root.
func Cleanup(created []string) []error {
	var errs []error
	for i := len(created) - 1; i >= 0; i-- {
		if err := os.Remove(created[i]); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			errs = append(errs, fmt.Errorf("remove empty directory %q: %w", created[i], err))
			break
		}
	}
	return errs
}

func checkResolvedContainment(resolvedRoot, destination string) error {
	current := destination
	var missing []string
	for {
		info, err := os.Stat(current)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("destination ancestor %q is not a directory", current)
			}
			break
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("inspect destination ancestor %q: %w", current, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return fmt.Errorf("could not find an existing ancestor for destination %q", destination)
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}

	resolved, err := filepath.EvalSymlinks(current)
	if err != nil {
		return fmt.Errorf("resolve destination ancestor %q: %w", current, err)
	}
	for i := len(missing) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, missing[i])
	}
	if !contained(resolvedRoot, resolved) {
		return fmt.Errorf("derived destination %q resolves outside clone root", destination)
	}
	return nil
}

func contained(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
