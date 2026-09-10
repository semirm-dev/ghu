package profile

// `ghu profile use` is the explicit, per-repository half of ghu's two switching
// mechanisms.
//
// It sets one local key, include.path, which pulls in the whole generated
// profile file. Setting user.name, user.email and core.sshCommand individually
// would leave three keys to keep in sync with a file that already holds them.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/semirm-dev/ghu/internal/command"
	"github.com/semirm-dev/ghu/internal/kernel"
	"github.com/semirm-dev/ghu/internal/kernel/sys"
	"github.com/semirm-dev/ghu/internal/ui"
)

const includeKey = "include.path"

// UseResult is returned rather than printed so the TUI can call Use and Clear
// directly and render the outcome its own way.
type UseResult struct {
	Profile     string `json:"profile,omitempty"`
	IncludePath string `json:"include_path,omitempty"`
	Previous    string `json:"previous,omitempty"`
	// PreviousProfile names the profile Previous pointed at, when ghu wrote it.
	PreviousProfile string `json:"previous_profile,omitempty"`
	Cleared         bool   `json:"cleared"`

	// The identity now effective in this repository, so a successful use can
	// show it without a second `ghu profile show`.
	User  string `json:"user,omitempty"`
	Email string `json:"email,omitempty"`
	Key   string `json:"key,omitempty"`

	// Redundant reports that the working directory already resolves to this
	// profile by directory match, making the local override unnecessary.
	Redundant bool `json:"redundant"`
	// ReplacedForeign reports that Previous was a value ghu did not write.
	ReplacedForeign bool `json:"replaced_foreign"`
	DryRun          bool `json:"dry_run"`
}

func (s *Set) Use(ctx context.Context, name string) (UseResult, error) {
	if !s.git.InRepo(ctx) {
		return UseResult{}, errors.New("`ghu profile use` needs a git repository: the current directory is not inside one")
	}

	p, ok := s.Config.Find(name)
	if !ok {
		return UseResult{}, kernel.UnknownProfile(s.Config, name)
	}

	file := s.Layout.ProfileFile(p.Name)
	if _, err := os.Stat(file); err != nil {
		if os.IsNotExist(err) {
			return UseResult{}, fmt.Errorf("profile %q has no generated config at %s: run `ghu init` to generate it", p.Name, file)
		}
		return UseResult{}, fmt.Errorf("reading %s: %w", file, err)
	}

	previous, err := s.current(ctx)
	if err != nil {
		return UseResult{}, err
	}

	res := UseResult{
		Profile:         p.Name,
		IncludePath:     file,
		Previous:        previous,
		PreviousProfile: s.profileNameFor(previous),
		User:            p.User,
		Email:           p.Email,
		Key:             kernel.Tildify(kernel.Expand(p.KeyPath(), s.Layout.Home), s.Layout.Home),
		DryRun:          s.git.DryRun(),
	}
	res.ReplacedForeign = previous != "" && !s.Layout.OwnsProfilePath(previous)

	// An explicit `use` beats an implicit directory match, so a redundant
	// override is still applied -- it is only worth saying so.
	if match, ok := kernel.MatchCurrent(s.Config, s.Layout.Home); ok && strings.EqualFold(match.Name, p.Name) {
		res.Redundant = true
	}

	if err := s.git.Set(ctx, sys.Local(), includeKey, file); err != nil {
		return UseResult{}, fmt.Errorf("applying profile %q: %w", p.Name, err)
	}
	return res, nil
}

// Clear refuses when include.path holds a value ghu did not write: removing
// configuration ghu does not own would be indistinguishable from losing it.
func (s *Set) Clear(ctx context.Context) (UseResult, error) {
	if !s.git.InRepo(ctx) {
		return UseResult{}, errors.New("`ghu profile use --clear` needs a git repository: the current directory is not inside one")
	}

	previous, err := s.current(ctx)
	if err != nil {
		return UseResult{}, err
	}

	res := UseResult{
		Previous:        previous,
		PreviousProfile: s.profileNameFor(previous),
		DryRun:          s.git.DryRun(),
	}
	if previous == "" {
		// Nothing to undo. Not an error: clearing twice should be safe.
		return res, nil
	}

	if !s.Layout.OwnsProfilePath(previous) {
		return UseResult{}, fmt.Errorf(
			"include.path in this repository is %s, which ghu did not write: remove it yourself with `git config --local --unset include.path`",
			previous)
	}

	if err := s.git.Unset(ctx, sys.Local(), includeKey); err != nil {
		return UseResult{}, fmt.Errorf("clearing include.path: %w", err)
	}
	res.Cleared = true
	return res, nil
}

func (s *Set) current(ctx context.Context) (string, error) {
	value, err := s.git.Get(ctx, sys.Local(), includeKey)
	if errors.Is(err, sys.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading include.path: %w", err)
	}
	return strings.TrimSpace(value), nil
}

func (s *Set) profileNameFor(path string) string {
	if path == "" || !s.Layout.OwnsProfilePath(path) {
		return ""
	}
	return strings.TrimSuffix(filepath.Base(path), kernel.ProfileFileSuffix)
}

func useCmd(e *command.Env) *cobra.Command {
	var clear bool

	cmd := &cobra.Command{
		Use:   "use [profile]",
		Short: "Use a profile in this repository, wherever it lives",
		Long: "Makes this one repository use a profile's account, no matter which folder\n" +
			"it sits in. For the repository outside every profile's folder, or the one\n" +
			"that needs a different account than its neighbours.\n\n" +
			"Affects this repository only. --clear undoes it, and the folder's own\n" +
			"profile takes over again.",
		Example: "  ghu profile use work     # this repository uses the work account\n" +
			"  ghu profile use --clear  # back to whatever the folder says",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			set, err := Open(e.Layout, e.Git)
			if err != nil {
				return err
			}

			var res UseResult
			switch {
			case clear && len(args) > 0:
				return errors.New("--clear takes no profile name")
			case clear:
				res, err = set.Clear(ctx)
			case len(args) == 0:
				return errors.New("accepts 1 arg, received 0: name a profile, or pass --clear")
			default:
				res, err = set.Use(ctx, args[0])
			}
			if err != nil {
				return err
			}
			return renderUse(e, res)
		},
	}

	cmd.Flags().BoolVar(&clear, "clear", false, "Undo it: go back to whatever this folder's profile says")
	return cmd
}

func renderUse(e *command.Env, res UseResult) error {
	if e.JSON {
		return e.JSONOut(res)
	}
	writeUseText(e.Out, res)
	return nil
}

func writeUseText(w io.Writer, res UseResult) {
	fmt.Fprintln(w, ui.Title.Render(useHeadline(res)))

	if !res.Cleared && res.Profile != "" {
		ui.Table(w, [][]string{
			{"  " + ui.Muted.Render("include.path"), res.IncludePath},
			{"  " + ui.Muted.Render("user.name"), res.User},
			{"  " + ui.Muted.Render("user.email"), res.Email},
			{"  " + ui.Muted.Render("key"), res.Key},
		})
	}

	if res.ReplacedForeign {
		fmt.Fprintln(w, ui.Warn.Render(fmt.Sprintf(
			"replaced an include.path ghu did not write: %s", res.Previous)))
	}
	if res.Redundant {
		fmt.Fprintln(w, ui.Muted.Render(fmt.Sprintf(
			"note: this directory already resolves to %s, so the local override is redundant", res.Profile)))
	}
	if res.DryRun {
		fmt.Fprintln(w, ui.Muted.Render("dry run: nothing was written"))
	}
}

func useHeadline(res UseResult) string {
	switch {
	case res.Cleared:
		if res.PreviousProfile != "" {
			return fmt.Sprintf("%s cleared %s from this repository", ui.Marker, res.PreviousProfile)
		}
		return fmt.Sprintf("%s cleared include.path from this repository", ui.Marker)
	case res.Profile == "":
		return "no ghu include.path is set in this repository"
	case res.PreviousProfile != "" && !strings.EqualFold(res.PreviousProfile, res.Profile):
		return fmt.Sprintf("%s switched this repository from %s to %s",
			ui.Marker, res.PreviousProfile, ui.Active.Render(res.Profile))
	case res.Previous == res.IncludePath:
		return fmt.Sprintf("%s %s was already applied to this repository",
			ui.Marker, ui.Active.Render(res.Profile))
	default:
		return fmt.Sprintf("%s %s applied to this repository", ui.Marker, ui.Active.Render(res.Profile))
	}
}
