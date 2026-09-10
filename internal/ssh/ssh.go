package ssh

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/semirm-dev/ghu/internal/cli/command"
	"github.com/semirm-dev/ghu/internal/kernel"
	"github.com/semirm-dev/ghu/internal/kernel/sys"
	"github.com/semirm-dev/ghu/internal/ui"
)

// `ghu ssh generate` -- the command, its flags, and everything it prints.

// addKeyURL is where an authentication key is pasted. GitHub uses the same
// form for signing keys, with the type selector switched.
const addKeyURL = "https://github.com/settings/ssh/new"

// rule frames the public key so it can be selected without catching the prose.
const rule = "────────────────────────────────────────────────────────────"

type GenerateResult struct {
	Profile       string `json:"profile"`
	KeyPath       string `json:"key_path"`
	PublicKeyPath string `json:"public_key_path"`
	// PublicKey is the line to paste into GitHub. Empty under --dry-run,
	// where the key was never written.
	PublicKey string `json:"public_key"`
	Sign      bool   `json:"sign"`
	// Replaced reports that an existing key at KeyPath was overwritten, or
	// under --dry-run that it would be.
	Replaced bool `json:"replaced"`
	DryRun   bool `json:"dry_run"`
}

// Generate creates a profile's keypair. It takes a resolved profile rather than
// a name and a config: the lookup is the caller's, and passing the whole config
// to reach one profile would be more than this needs.
func Generate(ctx context.Context, ssh *sys.SSH, home string, p kernel.Profile, force bool) (GenerateResult, error) {
	if strings.TrimSpace(p.Key) == "" {
		// Guessing a path here would write a key the profile does not
		// reference, which fails silently later at push time.
		return GenerateResult{}, fmt.Errorf(
			"profile %q has no key path: set one with `ghu profile add --name %s --key ~/.ssh/id_%s ...` or `ghu init`",
			p.Name, p.Name, p.Name)
	}

	// Generated artifacts always hold absolute paths: git expands ~ in some
	// fields but not in core.sshCommand, so ghu never relies on it.
	keyPath := kernel.Expand(p.KeyPath(), home)

	res := GenerateResult{
		Profile:       p.Name,
		KeyPath:       keyPath,
		PublicKeyPath: keyPath + sys.PublicKeySuffix,
		Sign:          p.Sign,
		DryRun:        ssh.DryRun(),
	}

	// The comment is what GitHub shows beside the key in its list, so the email
	// is the one value that makes a key identifiable months later.
	comment := strings.TrimSpace(p.Email)
	if comment == "" {
		comment = p.Name
	}

	// sys.Generate already refuses to overwrite without force; its error is
	// returned unwrapped so the user sees that instruction verbatim.
	gen, err := ssh.Generate(ctx, sys.GenerateOpts{
		Path:    keyPath,
		Comment: comment,
		Force:   force,
	})
	if err != nil {
		return GenerateResult{}, err
	}
	res.Replaced = gen.Replaced

	if ssh.DryRun() {
		// The runner suppressed ssh-keygen, so there is no public key to
		// read. Reporting that is more useful than failing on the absence.
		return res, nil
	}

	// Confirm the key really landed where the profile points, rather than
	// trusting ssh-keygen's exit status.
	if _, err := os.Stat(keyPath); err != nil {
		return GenerateResult{}, fmt.Errorf(
			"ssh-keygen reported success but no key exists at %s, which is where profile %q points: %w",
			keyPath, p.Name, err)
	}

	pub, err := sys.PublicKey(keyPath)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("profile %q: %w", p.Name, err)
	}
	res.PublicKey = pub

	return res, nil
}

// Command is `ghu ssh`: the keys a profile pushes and signs with.
func Command(e *command.Env) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Manage the keys profiles push with: generate",
		Long: "Each profile pushes with its own SSH key. These commands create and\n" +
			"inspect them; ghu never edits ~/.ssh/config, and never deletes a key.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}

	return command.WithHelp(cmd, generateCmd(e))
}

// Run generates one profile's key and reports it. It is exported so that
// `ghu profile add --generate` prints exactly what `ghu ssh generate` prints,
// without profile importing this package.
func Run(ctx context.Context, e *command.Env, name string, force bool) error {
	// The lookup is the adapter's: ssh.Generate takes a resolved profile, so
	// that it needs neither the whole config nor a name to search for.
	p, ok := e.Config.Find(name)
	if !ok {
		return kernel.UnknownProfile(e.Config, name)
	}

	res, err := Generate(ctx, e.SSH, e.Layout.Home, p, force)
	if err != nil {
		return err
	}
	return render(e, res)
}

// generateCmd builds `ghu ssh generate`.
func generateCmd(e *command.Env) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "generate <profile>",
		Short: "Create a profile's SSH key, to add to GitHub",
		Long: "Creates the SSH key a profile pushes with, and prints the public half for\n" +
			"you to paste into GitHub. Until you paste it there, pushes with that\n" +
			"account will be refused.\n\n" +
			"An existing key is never replaced without --force, and the private half is\n" +
			"never printed.",
		Example: "  ghu ssh generate work  # then paste the key it prints into GitHub",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(cmd.Context(), e, args[0], force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Replace an existing key (the old one cannot be recovered)")

	return cmd
}

func render(e *command.Env, res GenerateResult) error {
	if e.JSON {
		return e.JSONOut(res)
	}

	home := e.Layout.Home
	verb := "Generated"
	if res.DryRun {
		verb = "Would generate"
	}

	fmt.Fprintf(e.Out, "%s\n", ui.Title.Render(
		fmt.Sprintf("%s ed25519 key for profile %s", verb, res.Profile)))
	ui.Table(e.Out, [][]string{
		{"  private", kernel.Tildify(res.KeyPath, home)},
		{"  public", kernel.Tildify(res.PublicKeyPath, home)},
	})

	// Worth saying out loud: a key generated at some other path is a key git
	// will never use.
	fmt.Fprintf(e.Out, "\n%s profile %s points at this key\n",
		ui.Good.Render(ui.Marker), ui.Key.Render(res.Profile))

	// Under --force the existing key is destroyed, so say which run is about
	// to do it and which one already has.
	if res.Replaced {
		if res.DryRun {
			fmt.Fprintf(e.Out, "\n%s\n", ui.Warn.Render(
				"--force would replace the existing key at "+kernel.Tildify(res.KeyPath, home)+
					"; the old key cannot be recovered."))
		} else {
			fmt.Fprintf(e.Out, "\n%s\n", ui.Warn.Render(
				"replaced the existing key at "+kernel.Tildify(res.KeyPath, home)+
					"; hosts trusting the old key must be given this one."))
		}
	}

	if res.DryRun {
		fmt.Fprintf(e.Out, "\n%s\n",
			ui.Warn.Render("--dry-run: nothing was written, so there is no public key to show."))
		fmt.Fprintf(e.Out, "Re-run without --dry-run to create it.\n")
		return nil
	}

	fmt.Fprintf(e.Out, "\nAdd this public key at %s\n\n", addKeyURL)
	fmt.Fprintf(e.Out, "%s\n%s\n%s\n", ui.Muted.Render(rule), res.PublicKey, ui.Muted.Render(rule))

	if res.Sign {
		fmt.Fprint(e.Out, "\n"+signingNote(res.Profile))
	}
	return nil
}

// signingNote spells out GitHub's separate key lists: profiles with sign: true
// set commit.gpgsign, and a key registered only for authentication leaves every
// commit unverified with no error anywhere to explain it.
func signingNote(name string) string {
	var b strings.Builder
	b.WriteString(ui.Warn.Render("Profile " + name + " also signs commits with this key."))
	b.WriteString("\n")
	b.WriteString("Add the SAME key a SECOND time at " + addKeyURL + ", choosing key type \"Signing Key\".\n")
	b.WriteString("GitHub keeps authentication keys and signing keys in separate lists: a key added\n")
	b.WriteString("only as an authentication key will not make your commits show as Verified.\n")
	return b.String()
}
