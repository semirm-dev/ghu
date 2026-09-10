// Package backup restores ~/.gitconfig from the copies ghu keeps.
//
// ghu backs the file up before every write it makes: once permanently, as the
// pivot -- your ~/.gitconfig exactly as it stood before ghu ever ran -- and
// then as rolling snapshots. This is the way back.
package backup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/semirm-dev/ghu/internal/cli/command"
	"github.com/semirm-dev/ghu/internal/kernel"
	"github.com/semirm-dev/ghu/internal/kernel/sys"
	"github.com/semirm-dev/ghu/internal/ui"
)

// Result is what a restore did, or would have done under --dry-run.
type Result struct {
	// From names the backup restored, as it appears in ~/.ghu/backups.
	From string `json:"from"`
	// To is the file it was written over.
	To string `json:"to"`
	// Replaced is the snapshot taken of the file being overwritten, empty when
	// there was nothing there to keep.
	Replaced string `json:"replaced,omitempty"`
	// Pivot reports that From was the original, rather than a later snapshot.
	Pivot  bool `json:"pivot"`
	DryRun bool `json:"dry_run"`
}

// Entry is one file in ~/.ghu/backups.
type Entry struct {
	Name  string    `json:"name"`
	Size  int64     `json:"size"`
	Taken time.Time `json:"taken"`
	Pivot bool      `json:"pivot"`
}

// Command is `ghu restore`.
func Command(e *command.Env) *cobra.Command {
	var (
		from string
		list bool
	)

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Put ~/.gitconfig back the way it was before ghu",
		Long: "Restores ~/.gitconfig from the copy ghu took before it first changed the\n" +
			"file. Your profiles stay configured -- this only undoes what ghu wrote to\n" +
			"~/.gitconfig, so run `ghu init` to apply them again.\n\n" +
			"The file being replaced is itself backed up first, so restoring is not a\n" +
			"one-way door.",
		Example: "  ghu restore                            # back to before ghu\n" +
			"  ghu restore --list                     # what is available\n" +
			"  ghu restore --from gitconfig.2026...Z  # a specific snapshot",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if list {
				entries, err := List(e.Layout)
				if err != nil {
					return err
				}
				return renderList(e, entries)
			}

			res, err := Run(e, from)
			if err != nil {
				return err
			}
			return render(e, res)
		},
	}

	cmd.Flags().StringVar(&from, "from", "",
		"Restore this backup instead of the original (see --list)")
	cmd.Flags().BoolVar(&list, "list", false,
		"List the backups ghu holds, without restoring anything")

	return cmd
}

// List reports the backups ghu holds, newest first, with the pivot last: it is
// the oldest thing there by definition.
func List(l kernel.Layout) ([]Entry, error) {
	dir := l.BackupsDir()

	found, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}

	entries := make([]Entry, 0, len(found))
	for _, f := range found {
		if f.IsDir() {
			continue
		}
		info, err := f.Info()
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", filepath.Join(dir, f.Name()), err)
		}
		entries = append(entries, Entry{
			Name:  f.Name(),
			Size:  info.Size(),
			Taken: info.ModTime(),
			Pivot: f.Name() == kernel.PivotName,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Pivot != entries[j].Pivot {
			return !entries[i].Pivot
		}
		return entries[i].Taken.After(entries[j].Taken)
	})
	return entries, nil
}

// Run restores ~/.gitconfig. An empty from means the pivot, which is the
// answer to "put it back the way it was".
func Run(e *command.Env, from string) (Result, error) {
	if from == "" {
		from = kernel.PivotName
	}
	if strings.ContainsRune(from, filepath.Separator) || from == ".." {
		return Result{}, fmt.Errorf("a backup is named by its file in ~/.ghu/backups, not by a path: %q", from)
	}

	src := filepath.Join(e.Layout.BackupsDir(), from)
	dst := e.Layout.GitConfig()

	data, err := os.ReadFile(src)
	if os.IsNotExist(err) {
		if from == kernel.PivotName {
			return Result{}, errors.New(
				"there is no backup to restore: ghu has not changed ~/.gitconfig on this machine")
		}
		return Result{}, fmt.Errorf("no backup named %q in %s; run `ghu restore --list` to see them",
			from, e.Layout.BackupsDir())
	}
	if err != nil {
		return Result{}, fmt.Errorf("reading %s: %w", src, err)
	}

	res := Result{
		From:   from,
		To:     dst,
		Pivot:  from == kernel.PivotName,
		DryRun: e.DryRun,
	}

	// Restoring overwrites a file the user may have changed since, so it is a
	// write like any other and gets the same backup first.
	if !e.DryRun {
		taken, err := e.Layout.BackupGitConfig(time.Now())
		if err != nil {
			return Result{}, err
		}
		res.Replaced = taken.Snapshot

		if err := sys.WriteAtomic(dst, data, 0o600); err != nil {
			return Result{}, err
		}
	}
	return res, nil
}

func render(e *command.Env, res Result) error {
	if e.JSON {
		return e.JSONOut(res)
	}

	home := e.Layout.Home
	verb := "restored"
	if res.DryRun {
		verb = "would restore"
	}

	what := "snapshot " + res.From
	if res.Pivot {
		what = "your original ~/.gitconfig"
	}

	fmt.Fprintln(e.Out, ui.Good.Render(ui.Marker+" "+verb+" ")+what)
	fmt.Fprintln(e.Out, ui.Muted.Render("  to "+kernel.Tildify(res.To, home)))

	if res.Replaced != "" {
		fmt.Fprintln(e.Out, ui.Muted.Render("  the file it replaced is kept at "+
			kernel.Tildify(res.Replaced, home)))
	}
	if res.DryRun {
		fmt.Fprintln(e.Out, ui.Muted.Render("  dry run: nothing was written"))
		return nil
	}

	// The profiles are still configured; only what ghu wrote to ~/.gitconfig
	// is undone. Saying so stops this looking like it uninstalled ghu.
	fmt.Fprintln(e.Out, ui.Muted.Render(
		"  your profiles are untouched — run `ghu init` to apply them again"))
	return nil
}

func renderList(e *command.Env, entries []Entry) error {
	if e.JSON {
		if entries == nil {
			entries = []Entry{}
		}
		return e.JSONOut(entries)
	}

	if len(entries) == 0 {
		_, err := fmt.Fprintln(e.Out, ui.Muted.Render(
			"no backups yet — ghu has not changed ~/.gitconfig on this machine"))
		return err
	}

	rows := make([][]string, 0, len(entries)+1)
	rows = append(rows, []string{
		ui.Title.Render("BACKUP"),
		ui.Title.Render("TAKEN"),
		ui.Title.Render("SIZE"),
		ui.Title.Render(""),
	})
	for _, b := range entries {
		note := ""
		if b.Pivot {
			note = ui.Good.Render("your original, kept permanently")
		}
		rows = append(rows, []string{
			ui.Key.Render(b.Name),
			b.Taken.Local().Format("2006-01-02 15:04"),
			fmt.Sprintf("%d B", b.Size),
			note,
		})
	}
	ui.Table(e.Out, rows)
	return nil
}
