package main

import (
	"context"
	"os"

	"github.com/nkaewam/clone-path/internal/app"
)

func main() {
	os.Exit(app.Run(context.Background(), os.Args[1:], app.IO{
		In:  os.Stdin,
		Out: os.Stdout,
		Err: os.Stderr,
	}))
}
