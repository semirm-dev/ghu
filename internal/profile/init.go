package profile

import (
	"context"
	"errors"
	"strings"
	"time"

	"fmt"

	"github.com/spf13/cobra"

	"github.com/semirm-dev/ghu/internal/cli/command"
	"github.com/semirm-dev/ghu/internal/kernel"
	"github.com/semirm-dev/ghu/internal/tui"
	"github.com/semirm-dev/ghu/internal/ui"
)

type InitResult struct {
	// Added names the profiles this run created. Init creates none itself, so
	// it is the adapter's interview that fills this in.
	Added []string `json:"added,omitempty"`

	// Reset names the profiles a --reset removed, and where the config that
	// held them was kept.
	Reset       []string `json:"reset,omitempty"`
	ResetBackup string   `json:"reset_backup,omitempty"`

	Apply kernel.ApplyResult `json:"apply"`
}

// Init prepares ~/.ghu and reconciles every profile to disk. Safe to re-run.
//
// It does not offer to create a first profile: an empty config is a valid
// outcome, and whether to interview the user about it is the adapter's call.
func (s *Set) Init(ctx context.Context) (InitResult, error) {
	return s.init(ctx, false)
}

// Reset forgets every profile and puts ghu back to a fresh install.
//
// It is Init over an emptied config rather than a delete of its own: Apply
// already removes the generated files and includeIf entries belonging to
// profiles that are no longer configured, so emptying the config and
// reconciling is the same code path every other change takes.
//
// Backups and ssh keys are left alone. A key may be trusted by hosts ghu knows
// nothing about, and the backups are the way back from this.
func (s *Set) Reset(ctx context.Context) (InitResult, error) {
	return s.init(ctx, true)
}

func (s *Set) init(ctx context.Context, reset bool) (InitResult, error) {
	var result InitResult

	if !s.dryRun() {
		if err := s.Layout.EnsureDirs(); err != nil {
			return result, err
		}
		if err := kernel.EnsureConfigFile(s.Layout.ConfigFile()); err != nil {
			return result, err
		}
	}

	if err := s.Reload(); err != nil {
		return result, err
	}

	if reset {
		result.Reset = s.Config.Names()

		if !s.dryRun() {
			backup, err := kernel.BackupConfig(s.Layout.ConfigFile(), time.Now())
			if err != nil {
				return result, err
			}
			result.ResetBackup = backup
		}

		s.Config.Profiles = nil
		if !s.dryRun() {
			if err := s.Save(); err != nil {
				return result, err
			}
		}
	}

	applied, err := s.Apply(ctx)
	if err != nil {
		return result, err
	}
	result.Apply = applied

	return result, nil
}

// confirmReset asks before forgetting profiles, because nothing else ghu does
// discards configuration you wrote.
func confirmReset(e *command.Env, names []string, nonInteractive bool) error {
	if nonInteractive || !tui.Interactive() {
		// A script that asked for --reset meant it; there is nobody to ask.
		return nil
	}

	fmt.Fprintln(e.Out, ui.Warn.Render(fmt.Sprintf(
		"This forgets %d %s: %s", len(names),
		plural(len(names), "profile", "profiles"), strings.Join(names, ", "))))
	fmt.Fprintln(e.Out, ui.Muted.Render(
		"Backups and ssh keys are kept, and the config is saved alongside itself first."))

	if !tui.Confirm("Forget them?") {
		return errors.New("cancelled")
	}
	return nil
}

// initCmd builds `ghu init`.
func initCmd(e *command.Env, generate command.Action) *cobra.Command {
	var (
		nonInteractive bool
		reset          bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Set up ghu, and re-apply every profile to git",
		Example: "  ghu init          # set up, or re-apply what is configured\n" +
			"  ghu init --reset  # forget every profile and start over",
		Long: "Creates ~/.ghu, then writes each profile into git's configuration so that\n" +
			"folders start using the accounts you tied to them.\n\n" +
			"Run it once to get started. Run it again after editing ~/.ghu/config.yaml by\n" +
			"hand, or if `ghu doctor` reports something missing: it rewrites from your\n" +
			"config rather than patching it, so re-running is always safe.\n\n" +
			"--reset starts over: it forgets every profile, removes the files and\n" +
			"includeIf entries they produced, and leaves you where you began. Your\n" +
			"backups and ssh keys are kept, and the config is saved alongside itself\n" +
			"first, so this is recoverable.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			s, err := Open(e.Layout, e.Git)
			if err != nil {
				return err
			}

			if reset && !e.DryRun && len(s.Config.Profiles) > 0 {
				if err := confirmReset(e, s.Config.Names(), nonInteractive); err != nil {
					return err
				}
			}

			run := s.Init
			if reset {
				run = s.Reset
			}

			result, err := run(ctx)
			if err != nil {
				return err
			}
			e.Config = s.Config

			// An empty config after init is a valid outcome; offering to fill
			// it is an adapter's choice, not the core's.
			if len(s.Config.Profiles) == 0 && !nonInteractive && tui.Interactive() {
				if err := addInteractively(ctx, e, generate); err != nil {
					return err
				}
				result.Added = append(result.Added, e.Config.Names()...)
			}

			if e.JSON {
				return e.JSONOut(result)
			}

			out := e.Out
			if len(result.Reset) > 0 {
				fmt.Fprintln(out, ui.Good.Render(ui.Marker+" reset ghu: forgot ")+
					ui.Key.Render(fmt.Sprintf("%d", len(result.Reset)))+
					ui.Good.Render(" "+plural(len(result.Reset), "profile", "profiles"))+
					ui.Muted.Render(" ("+strings.Join(result.Reset, ", ")+")"))
				if result.ResetBackup != "" {
					fmt.Fprintln(out, ui.Muted.Render("  the config that held them is kept at "+
						kernel.Tildify(result.ResetBackup, e.Layout.Home)))
				}
				fmt.Fprintln(out, ui.Muted.Render("  backups and ssh keys were not touched"))
			}
			for _, name := range result.Added {
				fmt.Fprintln(out, ui.Good.Render("added profile ")+ui.Key.Render(name))
			}
			for _, line := range summarize(result.Apply, e.Layout) {
				fmt.Fprintln(out, ui.Muted.Render(line))
			}
			if len(e.Config.Profiles) == 0 {
				fmt.Fprintln(out, ui.Muted.Render("no profiles yet — run `ghu profile add`"))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false,
		"Never ask questions; fail instead")
	cmd.Flags().BoolVar(&reset, "reset", false,
		"Forget every profile and start over (keeps backups and ssh keys)")

	return cmd
}
