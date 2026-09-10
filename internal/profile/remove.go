package profile

import (
	"context"
	"strings"

	"fmt"

	"github.com/spf13/cobra"

	"github.com/semirm-dev/ghu/internal/kernel"
	"github.com/semirm-dev/ghu/internal/kernel/command"
	"github.com/semirm-dev/ghu/internal/ui"
)

type RemoveResult struct {
	Profile kernel.Profile     `json:"profile"`
	Apply   kernel.ApplyResult `json:"apply"`

	// KeyKept is the key left on disk. ghu never deletes ssh.
	KeyKept string `json:"key_kept"`
}

// Remove drops a profile and reconciles.
//
// The ssh key is deliberately left in place: it may be trusted by hosts ghu
// knows nothing about, and removing a profile is a config change, not a
// revocation.
func (s *Set) Remove(ctx context.Context, name string) (RemoveResult, error) {
	var result RemoveResult

	p, ok := s.Config.Find(name)
	if !ok {
		return result, kernel.UnknownProfile(s.Config, name)
	}

	kept := make([]kernel.Profile, 0, len(s.Config.Profiles)-1)
	for _, existing := range s.Config.Profiles {
		if !strings.EqualFold(existing.Name, name) {
			kept = append(kept, existing)
		}
	}
	s.Config.Profiles = kept

	if !s.dryRun() {
		if err := s.Save(); err != nil {
			return result, err
		}
	}

	applied, err := s.Apply(ctx)
	if err != nil {
		return result, err
	}

	return RemoveResult{
		Profile: p,
		Apply:   applied,
		KeyKept: kernel.Expand(p.KeyPath(), s.Layout.Home),
	}, nil
}

// removeCmd builds `ghu profile rm`.
func removeCmd(e *command.Env) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rm <name>",
		Aliases: []string{"remove"},
		Short:   "Remove a profile (its SSH key is kept)",
		Long: "Removes a profile, so its folder stops using that account.\n\n" +
			"The SSH key on disk is never deleted: other hosts may still trust it, and\n" +
			"removing a profile is a change of configuration, not a revocation.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := Open(e.Layout, e.Git)
			if err != nil {
				return err
			}
			result, err := s.Remove(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			if e.JSON {
				return e.JSONOut(result)
			}

			out := e.Out
			fmt.Fprintln(out, ui.Good.Render("removed profile ")+ui.Key.Render(result.Profile.Name))
			for _, line := range summarize(result.Apply, e.Layout) {
				fmt.Fprintln(out, ui.Muted.Render(line))
			}
			fmt.Fprintln(out, ui.Muted.Render("  key left in place: "+
				kernel.Tildify(result.KeyKept, e.Layout.Home)))
			return nil
		},
	}
	return cmd
}
