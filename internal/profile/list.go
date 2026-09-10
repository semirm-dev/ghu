package profile

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/semirm-dev/ghu/internal/cli/command"
	"github.com/semirm-dev/ghu/internal/kernel"
	"github.com/semirm-dev/ghu/internal/ui"
)

// ListRow carries paths with ~ rather than an absolute home: the config file
// stores them that way, and the output stays stable across machines with
// different home directories.
type ListRow struct {
	Name   string `json:"name"`
	Dir    string `json:"dir"`
	User   string `json:"user"`
	Login  string `json:"login,omitempty"`
	Email  string `json:"email"`
	Key    string `json:"key"`
	Sign   bool   `json:"sign"`
	Active bool   `json:"active"`
}

// List reports the configured profiles, marking the one governing the working
// directory. It cannot fail: an unresolvable directory simply marks nothing.
func List(cfg kernel.Config, home string) []ListRow {
	active, matched := kernel.MatchCurrent(cfg, home)

	rows := make([]ListRow, 0, len(cfg.Profiles))
	for _, p := range cfg.Profiles {
		rows = append(rows, ListRow{
			Name:   p.Name,
			Dir:    kernel.Tildify(p.Dir, home),
			User:   p.User,
			Login:  p.Login,
			Email:  p.Email,
			Key:    kernel.Tildify(p.KeyPath(), home),
			Sign:   p.Sign,
			Active: matched && p.Name == active.Name,
		})
	}
	return rows
}

func listCmd(e *command.Env) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List your profiles and the folders they cover",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return renderList(e, List(e.Config, e.Layout.Home))
		},
	}
}

func renderList(e *command.Env, rows []ListRow) error {
	w := e.Out
	if e.JSON {
		// An empty config is an empty array, never null, so consumers can
		// range over the result without a nil check.
		if rows == nil {
			rows = []ListRow{}
		}
		return e.JSONOut(rows)
	}

	if len(rows) == 0 {
		// Nothing configured is a starting state, not a failure.
		_, err := fmt.Fprintln(w, ui.Muted.Render("no profiles yet — run `ghu init` to create one"))
		return err
	}

	table := make([][]string, 0, len(rows)+1)
	table = append(table, []string{
		" ",
		ui.Title.Render("NAME"),
		ui.Title.Render("DIR"),
		ui.Title.Render("USER"),
		ui.Title.Render("LOGIN"),
		ui.Title.Render("EMAIL"),
		ui.Title.Render("KEY"),
		ui.Title.Render("SIGN"),
	})

	for _, r := range rows {
		marker := " "
		name := ui.Key.Render(r.Name)
		if r.Active {
			marker = ui.Active.Render(ui.Marker)
			name = ui.Active.Render(r.Name)
		}

		sign := ui.Muted.Render("no")
		if r.Sign {
			sign = ui.Good.Render("yes")
		}

		table = append(table, []string{
			marker,
			name,
			r.Dir,
			r.User,
			r.Login,
			r.Email,
			ui.Muted.Render(r.Key),
			sign,
		})
	}

	ui.Table(w, table)
	return nil
}
