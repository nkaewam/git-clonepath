// Package config reads and validates git-clonepath's Git configuration.
package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const setupCommand = "git config --global clonepath.root ~/Developer"

// Root reads the effective clonepath.root using Git's normal configuration
// precedence and --path expansion.
func Root(ctx context.Context, gitBinary string) (string, error) {
	cmd := exec.CommandContext(ctx, gitBinary, "config", "--path", "--get", "clonepath.root")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && strings.TrimSpace(stderr.String()) == "" {
			return "", fmt.Errorf("clonepath.root is not configured; set it with:\n  %s", setupCommand)
		}
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", fmt.Errorf("read clonepath.root from Git configuration: %s", detail)
		}
		return "", fmt.Errorf("read clonepath.root from Git configuration: %w", err)
	}

	root := strings.TrimSuffix(string(output), "\n")
	root = strings.TrimSuffix(root, "\r")
	if root == "" {
		return "", fmt.Errorf("clonepath.root is empty; set it with:\n  %s", setupCommand)
	}
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("clonepath.root %q is relative; configure an absolute path (or ~/...) with:\n  %s", root, setupCommand)
	}

	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("clonepath.root %q does not exist; create the directory or update the configuration", root)
		}
		return "", fmt.Errorf("inspect clonepath.root %q: %w", root, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("clonepath.root %q is not a directory", root)
	}

	return filepath.Clean(root), nil
}
