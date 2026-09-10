package profile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/semirm-dev/ghu/internal/command"
	"github.com/semirm-dev/ghu/internal/kernel"
	"github.com/semirm-dev/ghu/internal/sys"
	"github.com/semirm-dev/ghu/internal/ui"
)

// ShowKeys are resolved and reported in this order.
var ShowKeys = []string{
	"user.name",
	"user.email",
	"core.sshCommand",
	"user.signingkey",
	"commit.gpgsign",
}

// Value.Set distinguishes an unset key from one set to the empty string.
type Value struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	File  string `json:"file"`
	Set   bool   `json:"set"`
}

type Status struct {
	Dir     string  `json:"dir"`
	InRepo  bool    `json:"in_repo"`
	Profile string  `json:"profile"`
	Values  []Value `json:"values"`

	// ProfileEmail is what the matched profile declares, so the drift warning
	// can name both sides of the disagreement. It is the one field here that
	// comes from ghu's config rather than from git.
	ProfileEmail string `json:"profile_email,omitempty"`

	Override string `json:"override"`
	// OverrideManaged reports whether Override points into ghu's profiles
	// directory, as opposed to a file the user wired up themselves.
	OverrideManaged bool `json:"override_managed"`

	// Drift is set when a ghu profile governs this directory but git resolves
	// a different user.email than the profile declares. This is the failure
	// that mis-attributes commits silently.
	Drift bool `json:"drift"`
}

func (s Status) Lookup(key string) (Value, bool) {
	for _, v := range s.Values {
		if v.Key == key {
			return v, true
		}
	}
	return Value{}, false
}

// Show takes every value from `git config --show-origin`, never from ghu's own
// config: a report that read ghu's config would agree with itself by
// construction and could not detect drift.
func (s *Set) Show(ctx context.Context) (Status, error) {
	home := s.Layout.Home

	dir, err := sys.CurrentDir()
	if err != nil {
		return Status{}, fmt.Errorf("resolving current directory: %w", err)
	}

	status := Status{
		Dir:    kernel.Tildify(dir, home),
		InRepo: s.git.InRepo(ctx),
		Values: make([]Value, 0, len(ShowKeys)),
	}

	for _, key := range ShowKeys {
		origin, err := s.git.ShowOrigin(ctx, key)
		switch {
		case errors.Is(err, sys.ErrNotFound):
			// Unset is a fact to report, not a failure.
			status.Values = append(status.Values, Value{Key: key})
		case err != nil:
			return Status{}, fmt.Errorf("resolving %s: %w", key, err)
		default:
			status.Values = append(status.Values, Value{
				Key:   key,
				Value: origin.Value,
				File:  tildifyOrigin(origin.File, home),
				Set:   true,
			})
		}
	}

	matched, ok := kernel.MatchCurrent(s.Config, s.Layout.Home)
	if ok {
		status.Profile = matched.Name
		status.ProfileEmail = matched.Email
	}

	// --local only works inside a repository; asking outside one is a fatal
	// git error rather than an empty answer.
	if status.InRepo {
		include, err := s.git.Get(ctx, sys.Local(), "include.path")
		switch {
		case errors.Is(err, sys.ErrNotFound):
			// No per-repo override, which is the common case.
		case err != nil:
			return Status{}, fmt.Errorf("reading local include.path: %w", err)
		default:
			status.Override = kernel.Tildify(include, home)
			status.OverrideManaged = s.Layout.OwnsProfilePath(kernel.Expand(include, home))
		}
	}

	if ok {
		email, _ := status.Lookup(EmailKey)
		status.Drift = !strings.EqualFold(email.Value, matched.Email)
	}

	return status, nil
}

func showCmd(e *command.Env) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show which account git will use in this folder",
		Long: "Reports the name, email and SSH key git will actually use for commits made\n" +
			"here, and which file each value came from.\n\n" +
			"It reads git, not ghu's own config, so it is the command that can tell you\n" +
			"the two disagree -- the failure worth catching, because commits otherwise\n" +
			"land under the wrong account silently.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			set, err := Open(e.Layout, e.Git)
			if err != nil {
				return err
			}
			st, err := set.Show(cmd.Context())
			if err != nil {
				return err
			}
			return renderShow(e, st)
		},
	}
}

func renderShow(e *command.Env, st Status) error {
	w, errw := e.Out, e.Err
	if e.JSON {
		return e.JSONOut(st)
	}

	head := [][]string{
		{ui.Muted.Render("directory"), st.Dir},
	}

	if st.InRepo {
		head = append(head, []string{ui.Muted.Render("repository"), ui.Good.Render("yes")})
	} else {
		head = append(head, []string{
			ui.Muted.Render("repository"),
			ui.Warn.Render("no") + ui.Muted.Render(" — not inside a git repository, the values below are the global ones"),
		})
	}

	if st.Profile != "" {
		head = append(head, []string{ui.Muted.Render("profile"), ui.Key.Render(st.Profile)})
	} else {
		head = append(head, []string{
			ui.Muted.Render("profile"),
			ui.Muted.Render("no ghu profile covers this directory"),
		})
	}

	if st.InRepo {
		switch {
		case st.Override == "":
			head = append(head, []string{ui.Muted.Render("override"), ui.Muted.Render("none")})
		case st.OverrideManaged:
			head = append(head, []string{
				ui.Muted.Render("override"),
				st.Override + ui.Muted.Render(" (managed by ghu)"),
			})
		default:
			head = append(head, []string{
				ui.Muted.Render("override"),
				st.Override + ui.Muted.Render(" (not a ghu profile)"),
			})
		}
	}

	ui.Table(w, head)
	fmt.Fprintln(w)

	values := make([][]string, 0, len(st.Values)+1)
	values = append(values, []string{
		ui.Title.Render("KEY"),
		ui.Title.Render("VALUE"),
		ui.Title.Render("FROM"),
	})
	for _, v := range st.Values {
		if !v.Set {
			values = append(values, []string{ui.Key.Render(v.Key), ui.Muted.Render("not set"), ""})
			continue
		}
		values = append(values, []string{ui.Key.Render(v.Key), v.Value, ui.Muted.Render(v.File)})
	}
	ui.Table(w, values)

	if st.Drift {
		fmt.Fprintln(errw, ui.Warn.Render(driftMessage(st)))
	}
	return nil
}
