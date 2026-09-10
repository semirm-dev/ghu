package doctor

// doctor reads the world back -- the filesystem, ~/.gitconfig as git itself
// parses it, and GitHub's answer to the profile's key -- and reports where
// reality and the config disagree. It behaves like a linter: no check aborts
// the run, and every finding is collected.

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/semirm-dev/ghu/internal/kernel"
	"github.com/semirm-dev/ghu/internal/kernel/command"
)

type Opts struct {
	// Profile limits the run to one profile. Empty checks them all.
	Profile string
	Offline bool
}

// `ghu doctor` -- the command, its flags, and everything it prints.

func Commands(e *command.Env) []*cobra.Command {
	var opts Opts

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check that every profile actually works",
		Long: "Checks each profile end to end: its folder exists, its SSH key is there\n" +
			"with sane permissions, git is wired up to use it, and -- the one thing no\n" +
			"local check can answer -- github.com agrees the key belongs to the account\n" +
			"the profile claims.\n\n" +
			"Run it when something is off, or in CI: it exits non-zero if any check\n" +
			"fails.",
		Example: "  ghu doctor\n" +
			"  ghu doctor --offline  # skip the check against github.com",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), e, opts)
		},
		// The failure is already printed as findings; cobra repeating it as
		// "Error: ..." with usage would bury them.
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&opts.Profile, "profile", "", "Check only this profile")
	cmd.Flags().BoolVar(&opts.Offline, "offline", false, "Skip the check against github.com, and run offline")

	return []*cobra.Command{cmd}
}

// run checks every profile and prints the report.
func run(ctx context.Context, e *command.Env, opts Opts) error {
	c := checker{layout: e.Layout, git: e.Git, ssh: e.SSH, cfg: e.Config}

	report, err := c.check(ctx, opts)
	if err != nil {
		return err
	}
	if err := printReport(e, report); err != nil {
		return err
	}
	return report.Err()
}

// (checks lives here rather than in checker.go or probe.go because both halves
// of a run -- the local checks and the github probe -- hang their methods off
// it, and neither owns it.)

func targets(cfg kernel.Config, name string) ([]kernel.Profile, error) {
	if name == "" {
		return cfg.Profiles, nil
	}
	p, ok := cfg.Find(name)
	if !ok {
		return nil, fmt.Errorf("profile %q not found in the config", name)
	}
	return []kernel.Profile{p}, nil
}
