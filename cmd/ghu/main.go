// Command ghu switches GitHub identities per directory tree.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/charmbracelet/fang"

	"github.com/semirm-dev/ghu/internal/cli"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := fang.Execute(ctx, cli.NewRootCmd(), fang.WithVersion(cli.Version)); err != nil {
		os.Exit(1)
	}
}
