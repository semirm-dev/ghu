package cli

// The cobra command tree: which features contribute commands, which flags they
// all share, and which of them need a loaded config before they run.

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/colorprofile"
	"github.com/spf13/cobra"

	"github.com/semirm-dev/ghu/internal/backup"
	"github.com/semirm-dev/ghu/internal/core"
	"github.com/semirm-dev/ghu/internal/core/command"
	"github.com/semirm-dev/ghu/internal/core/sys"
	"github.com/semirm-dev/ghu/internal/doctor"
	"github.com/semirm-dev/ghu/internal/profile"
	"github.com/semirm-dev/ghu/internal/ssh"
)

// NewRootCmd builds the ghu command tree.
func NewRootCmd() *cobra.Command {
	// Commands appear in the order each feature adds them, not alphabetically:
	// `add, ls, rm` is the order you would use them in, and sorting that into
	// `add, ls, rm` by first letter says nothing.
	cobra.EnableCommandSorting = false

	out, errOut := colorWriters()

	// One env per command tree, filled in before each command runs. Commands
	// needing nothing -- `ghu version`, `ghu --help` -- never touch the
	// filesystem, and their env keeps only the writers.
	e := &command.Env{Out: out, Err: errOut}

	// generate is ssh's command, handed to profile so `ghu profile add
	// --generate` prints exactly what `ghu ssh generate` prints. Passing it
	// rather than importing it is what keeps the features independent.
	generate := func(ctx context.Context, name string) error {
		return ssh.Run(ctx, e, name, false)
	}

	root := &cobra.Command{
		Use:   "ghu",
		Short: "Use a different GitHub account in each folder",
		Long: "Use a different GitHub account in each folder, without switching.\n\n" +
			"A profile is one GitHub account -- its name, email and SSH key -- tied to a\n" +
			"folder. Clone anything under that folder and git commits and pushes as that\n" +
			"account, automatically. Nothing to run, nothing to remember.\n\n" +
			"A repository outside every profile's folder can opt in with `ghu profile use`.\n" +
			"ghu never edits ~/.ssh/config, and never deletes an SSH key.",
		Example: "  ghu init                # set up, once\n" +
			"  ghu profile add         # tie a GitHub account to a folder\n" +
			"  ghu ssh generate work   # create its key, then paste into GitHub\n" +
			"  ghu profile show        # which account does git use here?\n" +
			"  ghu profile use work    # force this repository to use it\n" +
			"  ghu doctor              # is it all actually working?\n" +
			"  ghu restore             # put ~/.gitconfig back the way it was\n" +
			"\n" +
			"  ghu profile --help      # everything you can do to profiles\n" +
			"  ghu ssh --help          # everything you can do to keys",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,

		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// Keep the writers in step with whatever the caller set on the
			// command, so a test -- or any embedder -- can capture a whole run.
			e.Out, e.Err = cmd.OutOrStdout(), cmd.ErrOrStderr()

			if !needsDeps(cmd) {
				return nil
			}
			return resolve(e)
		},

		// Bare `ghu` says what ghu can do. Anything that acts is a command you
		// named, which is what makes the tool predictable from a script.
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	// Subcommands inherit these, so cmd.OutOrStdout() is already colour-aware.
	root.SetOut(out)
	root.SetErr(errOut)

	root.PersistentFlags().BoolVar(&e.DryRun, "dry-run", false,
		"Show what would happen, changing nothing")
	root.PersistentFlags().BoolVar(&e.JSON, "json", false,
		"Print JSON instead of a table, for scripts")
	root.PersistentFlags().BoolVar(&e.Verbose, "verbose", false,
		"Print each git and ssh command as it runs")

	// Each feature contributes the commands that drive it.

	root.AddCommand(profile.InitCommand(e, generate))
	root.AddCommand()
	root.AddCommand(profile.Command(e, generate))
	root.AddCommand(ssh.Command(e))
	root.AddCommand(doctor.Commands(e)...)
	root.AddCommand(backup.Command(e))
	root.AddCommand(newVersionCmd())

	return root
}

// needsDeps reports whether a command reads ghu's configuration.
func needsDeps(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "version", "help", "completion", "bash", "zsh", "fish", "powershell", "man":
		return false
	}
	return true
}

// resolve fills in the collaborators for one invocation.
func resolve(e *command.Env) error {
	layout, err := core.NewLayout("")
	if err != nil {
		return fmt.Errorf("resolving configuration: %w", err)
	}
	e.Layout = layout

	// --dry-run and --verbose are the runner's configuration, not a choice
	// between runners, so there is nothing to branch on here. The runner is the
	// only place the dry run is recorded; Git and SSH read it back from there.
	runner := sys.NewRunner(runnerOut(e), e.Verbose, e.DryRun)
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
