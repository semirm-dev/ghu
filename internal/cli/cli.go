// Package cli assembles ghu's command tree.
//
// It owns no commands of its own beyond `version`: each feature package
// declares the commands that drive it -- `ghu profile add` in profiles, `ghu profile show` in
// ssh -- and this package resolves one invocation's wiring, hands it to each
// of them, and connects the few actions that cross a feature boundary.
package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/colorprofile"

	"github.com/semirm-dev/ghu/internal/command"
	"github.com/semirm-dev/ghu/internal/kernel"
	"github.com/semirm-dev/ghu/internal/kernel/sys"
	"github.com/semirm-dev/ghu/internal/profile"
)

// resolve fills in the collaborators for one invocation.
func resolve(e *command.Env) error {
	layout, err := kernel.NewLayout("")
	if err != nil {
		return fmt.Errorf("resolving configuration: %w", err)
	}
	e.Layout = layout

	// --dry-run and --verbose are the runner's configuration, not a choice
	// between runners, so there is nothing to branch on here. The runner is the
	// only place the dry run is recorded; Git and SSH read it back from there.
	runner := sys.NewRunner(sys.Opts{
		Out:     runnerOut(e),
		Verbose: e.Verbose,
		DryRun:  e.DryRun,
	})
	e.Git = sys.NewGit(runner)
	e.SSH = sys.NewSSH(runner)

	set, err := profile.Open(e.Layout, e.Git)
	if err != nil {
		return fmt.Errorf("resolving configuration: %w", err)
	}
	e.Config = set.Config
	return nil
}

// runnerOut sends the --dry-run listing to stdout, where it is the output the
// user asked for, and the --verbose echo to stderr, where it is commentary
// alongside whatever the command itself prints.
func runnerOut(e *command.Env) io.Writer {
	if e.DryRun {
		return e.Out
	}
	return e.Err
}

// colorWriters wrap the process streams. lipgloss always renders escape codes;
// the writer decides what survives, so writing straight to os.Stdout would leak
// ANSI into pipes and redirects. These honour NO_COLOR and TERM.
func colorWriters() (io.Writer, io.Writer) {
	return colorprofile.NewWriter(os.Stdout, os.Environ()),
		colorprofile.NewWriter(os.Stderr, os.Environ())
}
