package profile

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/semirm-dev/ghu/internal/command"
	"github.com/semirm-dev/ghu/internal/kernel"
	"github.com/semirm-dev/ghu/internal/tui"
	"github.com/semirm-dev/ghu/internal/ui"
)

type AddOpts struct {
	Profile kernel.Profile

	// Generate records the intent only. Generating the key is a second
	// operation, so an adapter can report the profile before the key exists.
	Generate bool
}

type AddResult struct {
	Profile  kernel.Profile     `json:"profile"`
	Generate bool               `json:"generate"`
	Apply    kernel.ApplyResult `json:"apply"`
}

// Add appends a profile and reconciles.
//
// The profile arrives finished. Collecting one from flags, from a form or from
// a request body is the adapter's job -- putting a prompt in here would mean
// this operation could not run without a terminal.
func (s *Set) Add(ctx context.Context, opts AddOpts) (AddResult, error) {
	var result AddResult

	p := opts.Profile
	generate := opts.Generate

	if p.Key == "" {
		p.Key = kernel.KeyDir + "/id_" + p.Name
	}

	if err := p.Validate(); err != nil {
		return result, err
	}
	if _, exists := s.Config.Find(p.Name); exists {
		return result, fmt.Errorf("profile %q already exists", p.Name)
	}

	s.Config.Profiles = append(s.Config.Profiles, p)

	if err := s.Config.Validate(); err != nil {
		return result, err
	}
	if !s.dryRun() {
		if err := s.Save(); err != nil {
			return result, err
		}
	}

	applied, err := s.Apply(ctx)
	if err != nil {
		return result, err
	}

	return AddResult{Profile: p, Generate: generate, Apply: applied}, nil
}

// addCmd builds `ghu profile add`.
func addCmd(e *command.Env, generate command.Action) *cobra.Command {
	var opts AddOpts

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Tie a GitHub account to a folder",
		Long: "Adds a profile: a GitHub account -- name, email and SSH key -- tied to a\n" +
			"folder. Every repository under that folder then uses that account.\n\n" +
			"--user is the name on your commits, git's user.name -- a display name.\n" +
			"--login is your GitHub account name; git never sees it, and `ghu doctor`\n" +
			"uses it to check the key really authenticates as that account.\n\n" +
			"With no flags it asks you for each value. With flags it runs unattended,\n" +
			"which is what you want in a script.",
		Example: "  ghu profile add   # asks you for each value\n" +
			"  ghu profile add --name work --dir ~/work \\\n" +
			"      --user \"Your Name\" --login your-gh-login --email you@work.com",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// No flags means no profile to add, so ask -- which is what the
			// help promises. Refusing without a terminal is what keeps a
			// scripted `ghu profile add` from hanging on a prompt.
			if opts.Profile.Name == "" {
				if !tui.Interactive() {
					return errors.New(
						"nothing to add: pass --name, --dir, --user and --email, " +
							"or run this where ghu can ask you")
				}
				return addInteractively(cmd.Context(), e, generate)
			}

			s, err := Open(e.Layout, e.Git)
			if err != nil {
				return err
			}
			result, err := s.Add(cmd.Context(), opts)
			if err != nil {
				return err
			}
			e.Config = s.Config
			if err := renderAdd(e, result); err != nil {
				return err
			}
			// generate owns the output for a key, so `ghu profile add --generate` and
			// `ghu ssh generate` print the same thing.
			if result.Generate {
				fmt.Fprintln(e.Out)
				return generate(cmd.Context(), result.Profile.Name)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&opts.Profile.Name, "name", "", "Short name for this profile, e.g. work or personal")
	cmd.Flags().StringVar(&opts.Profile.Dir, "dir", "", "Folder whose repositories use this account")
	cmd.Flags().StringVar(&opts.Profile.User, "user", "",
		"Name shown on your commits, e.g. \"Semir Mahovkic\" (git's user.name)")
	cmd.Flags().StringVar(&opts.Profile.Login, "login", "",
		"Your GitHub account name, so `ghu doctor` can check the key matches")
	cmd.Flags().StringVar(&opts.Profile.Email, "email", "", "Email GitHub attributes your commits to")
	cmd.Flags().StringVar(&opts.Profile.Key, "key", "", "Private key in ~/.ssh, by name or path (default id_<name>)")
	cmd.Flags().BoolVar(&opts.Profile.Sign, "sign", false, "Sign commits with this key, so GitHub marks them Verified")
	cmd.Flags().BoolVar(&opts.Generate, "generate", false,
		"Also create the SSH key now, and print it to add to GitHub")

	return cmd
}

func renderAdd(e *command.Env, result AddResult) error {
	if e.JSON {
		return e.JSONOut(result)
	}

	home := e.Layout.Home
	fmt.Fprintln(e.Out, ui.Good.Render("added profile ")+ui.Key.Render(result.Profile.Name))
	fmt.Fprintln(e.Out, ui.Muted.Render("  "+kernel.Tildify(result.Profile.Dir, home)+
		" → "+result.Profile.User+" <"+result.Profile.Email+">"))
	for _, line := range summarize(result.Apply, e.Layout) {
		fmt.Fprintln(e.Out, ui.Muted.Render(line))
	}
	return nil
}
