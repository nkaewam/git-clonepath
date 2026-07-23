// Package gitexec invokes the installed Git client without intercepting its I/O.
package gitexec

import (
	"context"
	"errors"
	"io"
	"os/exec"
)

// Clone invokes git clone with the provided arguments and derived destination.
// It returns Git's exact exit code if the child process starts.
func Clone(ctx context.Context, gitBinary string, args []string, destination string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	gitArgs := make([]string, 0, len(args)+2)
	gitArgs = append(gitArgs, "clone")
	gitArgs = append(gitArgs, args...)
	gitArgs = append(gitArgs, destination)

	cmd := exec.CommandContext(ctx, gitBinary, gitArgs...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 1, err
}
