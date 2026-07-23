// Package app orchestrates the git-clonepath command.
package app

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/nkaewam/clone-path/internal/config"
	"github.com/nkaewam/clone-path/internal/destination"
	"github.com/nkaewam/clone-path/internal/gitexec"
	"github.com/nkaewam/clone-path/internal/remote"
)

// IO contains the streams inherited by the Git child process.
type IO struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// Run executes git-clonepath and returns the process exit code.
func Run(ctx context.Context, args []string, streams IO) int {
	if len(args) == 0 {
		fmt.Fprintln(streams.Err, "usage: git clonepath [git clone options] <hosted-remote>")
		return 2
	}

	rawRemote := args[len(args)-1]
	identity, err := remote.Parse(rawRemote)
	if err != nil {
		printError(streams.Err, err)
		return 2
	}

	gitBinary, err := exec.LookPath("git")
	if err != nil {
		printError(streams.Err, fmt.Errorf("find Git executable on PATH: %w", err))
		return 1
	}

	root, err := config.Root(ctx, gitBinary)
	if err != nil {
		printError(streams.Err, err)
		return 1
	}

	plan, err := destination.NewPlan(root, identity)
	if err != nil {
		printError(streams.Err, err)
		return 1
	}
	created, err := plan.CreateParents()
	if err != nil {
		printError(streams.Err, err)
		return 1
	}

	code, runErr := gitexec.Clone(ctx, gitBinary, args, plan.Path, streams.In, streams.Out, streams.Err)
	if code != 0 || runErr != nil {
		for _, cleanupErr := range destination.Cleanup(created) {
			fmt.Fprintf(streams.Err, "git-clonepath: warning: %v\n", cleanupErr)
		}
	}
	if runErr != nil {
		printError(streams.Err, fmt.Errorf("start Git clone: %w", runErr))
		return 1
	}
	return code
}

func printError(w io.Writer, err error) {
	fmt.Fprintf(w, "git-clonepath: %v\n", err)
}
