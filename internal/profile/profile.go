package profile

// The mutable profile set: loading it, changing it, and keeping ~/.ghu and
// ~/.gitconfig in step with it.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/semirm-dev/ghu/internal/cli/command"
	"github.com/semirm-dev/ghu/internal/kernel"
	"github.com/semirm-dev/ghu/internal/kernel/sys"
	"github.com/semirm-dev/ghu/internal/tui"
)

// EmailKey is the key GitHub attributes commits by, and therefore the one the
// drift check compares.
const EmailKey = "user.email"

// Set is the profile set on disk together with the operations that change it.
//
// Add, Remove and Init are the only operations that both edit the config and
// reconcile afterwards, so this is the one type holding a config, the layout it
// lives in, and the reconciler that follows a change. List reads the same
// config without one.
type Set struct {
	Layout kernel.Layout
	Config kernel.Config

	git *sys.Git
}

// Open loads the profile set from a home directory. A config that fails to
// load is fatal rather than skipped: reconciling from one ghu could not parse
// would rewrite ~/.gitconfig from an empty profile list.
func Open(l kernel.Layout, git *sys.Git) (*Set, error) {
	cfg, err := kernel.LoadConfig(l.ConfigFile())
	if err != nil {
		return nil, err
	}

	return &Set{
		Layout: l,
		Config: cfg,
		git:    git,
	}, nil
}

// `ghu init`, `add`, `rm` and `ls` -- the commands, their flags, and everything
// they print.

// `ghu profile ls` reports what ghu's own config claims, marking the profile that
// covers the current directory.

// InitCommand is `ghu init`. It sits at the top level rather than under
// `ghu profile` because it is the first thing you run, before there is a
// profile to manage.
func InitCommand(e *command.Env, generate command.Action) *cobra.Command {
	return initCmd(e, generate)
}

// Command is `ghu profile`: everything you do to the set once it exists.
//
// generate is ssh's command, handed in so that `ghu profile add --generate`
// prints exactly what `ghu ssh generate` prints, without profile importing ssh.
func Command(e *command.Env, generate command.Action) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "profile",
		Aliases: []string{"profiles"},
		Short:   "Manage profiles: add, ls, show, use, rm",
		Long: "A profile is one GitHub account -- its name, email and SSH key -- tied to a\n" +
			"folder. Every repository under that folder uses that account.\n\n" +
			"`show` and `use` work on the repository you are standing in: which profile\n" +
			"it ends up with, and how to override that.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}

	return command.WithHelp(cmd,
		addCmd(e, generate), listCmd(e), showCmd(e), useCmd(e), removeCmd(e))
}

// Reload re-reads the config from disk, after an operation changed it.
func (s *Set) Reload() error {
	cfg, err := kernel.LoadConfig(s.Layout.ConfigFile())
	if err != nil {
		return err
	}
	s.Config = cfg
	return nil
}

func (s *Set) Save() error {
	return kernel.SaveConfig(s.Layout.ConfigFile(), s.Config)
}

// Apply reconciles ~/.ghu and ~/.gitconfig to match the loaded config.
func (s *Set) Apply(ctx context.Context) (kernel.ApplyResult, error) {
	return kernel.Reconciler{Layout: s.Layout, Git: s.git}.Apply(ctx, s.Config)
}

// dryRun reports whether this set's writes are suppressed. The runner behind
// git is the only place that is recorded.
func (s *Set) dryRun() bool { return s.git.DryRun() }

// Rendering shared by every command that reconciles -- init, add and rm all
// report the same ApplyResult, and reporting it differently would make the
// same underlying change look like three different ones.

// summarize renders an ApplyResult for a human.
func summarize(result kernel.ApplyResult, l kernel.Layout) []string {
	home := l.Home
	var lines []string

	if n := len(result.ProfilesWritten); n > 0 {
		lines = append(lines, fmt.Sprintf("generated %d profile %s: %s",
			n, plural(n, "file", "files"), strings.Join(result.ProfilesWritten, ", ")))
	}
	if n := len(result.ProfilesRemoved); n > 0 {
		lines = append(lines, fmt.Sprintf("removed %d stale profile %s: %s",
			n, plural(n, "file", "files"), strings.Join(result.ProfilesRemoved, ", ")))
	}
	if n := len(result.IncludesWritten); n > 0 {
		lines = append(lines, fmt.Sprintf("wrote %d includeIf %s to %s",
			n, plural(n, "entry", "entries"), kernel.Tildify(l.GitConfig(), home)))
	}
	// The first run is worth calling out: that copy is the only pristine
	// record of what ~/.gitconfig looked like before ghu existed.
	switch {
	case result.Backup.Established:
		lines = append(lines, "preserved your original ~/.gitconfig at "+
			kernel.Tildify(result.Backup.Pivot, home)+" (kept permanently)")
	case result.Backup.Snapshot != "":
		lines = append(lines, "backed up ~/.gitconfig to "+
			kernel.Tildify(result.Backup.Snapshot, home))
	}
	return lines
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// addInteractively collects a profile from a form and adds it, for `ghu init`
// when it finds no profiles and there is a terminal to ask on.
func addInteractively(ctx context.Context, e *command.Env, generate command.Action) error {
	p, wantsKey, err := tui.PromptProfile(e.Layout.Home, e.Config.Names(), kernel.Profile{})
	if err != nil {
		return err
	}
	if p.Name == "" {
		return errors.New("cancelled")
	}

	s, err := Open(e.Layout, e.Git)
	if err != nil {
		return err
	}
	result, err := s.Add(ctx, AddOpts{Profile: p, Generate: wantsKey})
	if err != nil {
		return err
	}
	e.Config = s.Config
	if err := renderAdd(e, result); err != nil {
		return err
	}
	if result.Generate {
		return generate(ctx, result.Profile.Name)
	}
	return nil
}

func driftMessage(st Status) string {
	email, _ := st.Lookup(EmailKey)
	effective := email.Value
	if !email.Set {
		effective = "not set"
	}
	return fmt.Sprintf(
		"warning: profile %q declares %s but git resolves user.email to %s — commits made here would be attributed to the wrong account; run `ghu init` to reconcile",
		st.Profile, st.ProfileEmail, effective)
}

// git reports origins other than files (command line, standard input) without
// a file: prefix, and those are not paths, so they are left alone.
func tildifyOrigin(origin, home string) string {
	if origin == "" || !strings.HasPrefix(origin, "/") {
		return origin
	}
	return kernel.Tildify(origin, home)
}
