// Package tui is the form ghu puts on screen to collect a profile, and the
// check for whether it can put anything on screen at all.
//
// It depends on the model and nothing else, so any feature can prompt with it.
package tui

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"charm.land/huh/v2"
	"github.com/charmbracelet/x/term"

	"github.com/semirm-dev/ghu/internal/kernel"
)

const (
	keyExisting keyChoice = iota
	keyGenerate
)

type keyChoice int

// Interactive reports whether ghu can put a form on the screen. A form written
// to a pipe waits forever for a keystroke that never comes, which is how a CLI
// hangs a CI job.
func Interactive() bool {
	return term.IsTerminal(os.Stdout.Fd()) && term.IsTerminal(os.Stdin.Fd())
}

// Confirm asks a yes/no question, defaulting to no. Callers check Interactive
// first: a confirmation written to a pipe waits forever for a keystroke that
// never comes.
func Confirm(title string) bool {
	answer := false
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().Title(title).Affirmative("Yes").Negative("No").Value(&answer),
		),
	).Run()
	return err == nil && answer
}

// PromptProfile collects one profile. It gathers values and applies nothing,
// keeping the interactive and flag-driven paths on the same code below here.
func PromptProfile(home string, taken []string, seed kernel.Profile) (kernel.Profile, bool, error) {
	p := seed
	choice := keyExisting
	if p.Key == "" {
		choice = keyGenerate
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Profile name").
				Description("Short identifier, e.g. personal or work.").
				Value(&p.Name).
				Validate(validateName(taken, seed.Name)),

			huh.NewInput().
				Title("Directory").
				Description("Repositories under this tree get this kernel.").
				Value(&p.Dir).
				Validate(validateDir(home)),

			huh.NewInput().
				Title("Name on your commits").
				Description("A display name, e.g. Semir Mahovkic. This is git's user.name.").
				Value(&p.User).
				Validate(required("name")),

			huh.NewInput().
				Title("GitHub account name").
				Description("The one in github.com/<name>. Optional; lets `ghu doctor` verify the key.").
				Value(&p.Login),

			huh.NewInput().
				Title("Commit email").
				Description("This is what GitHub attributes commits by.").
				Value(&p.Email).
				Validate(validateEmail),
		),

		huh.NewGroup(
			huh.NewSelect[keyChoice]().
				Title("SSH key").
				Options(
					huh.NewOption("Use an existing key", keyExisting),
					huh.NewOption("Generate a new one", keyGenerate),
				).
				Value(&choice),

			huh.NewInput().
				Title("Key path").
				Description("Private key. The public half is <path>.pub.").
				Value(&p.Key).
				Validate(required("key path")),

			huh.NewConfirm().
				Title("Sign commits with this key?").
				Description("Adds SSH commit signing, so commits show Verified.").
				Value(&p.Sign),
		),
	)

	if err := form.Run(); err != nil {
		if err == huh.ErrUserAborted {
			return kernel.Profile{}, false, nil
		}
		return kernel.Profile{}, false, err
	}

	p.Name = strings.TrimSpace(p.Name)
	p.Dir = strings.TrimSpace(p.Dir)
	p.User = strings.TrimSpace(p.User)
	p.Email = strings.TrimSpace(p.Email)
	p.Key = strings.TrimSpace(p.Key)

	if p.Key == "" {
		p.Key = kernel.KeyDir + "/id_" + p.Name
	}
	return p, choice == keyGenerate, nil
}

func validateName(taken []string, allow string) func(string) error {
	return func(s string) error {
		s = strings.TrimSpace(s)
		if err := (kernel.Profile{Name: s, Dir: "x", User: "x", Email: "x@y", Key: "x"}).Validate(); err != nil {
			return err
		}
		for _, t := range taken {
			if strings.EqualFold(t, s) && !strings.EqualFold(t, allow) {
				return fmt.Errorf("profile %q already exists", s)
			}
		}
		return nil
	}
}

// validateDir insists the directory exists: a profile pointing at a missing
// path never matches, and fails silently -- commits just get the wrong
// kernel.
func validateDir(home string) func(string) error {
	return func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" {
			return errors.New("directory is required")
		}

		abs := kernel.Expand(s, home)
		info, err := os.Stat(abs)
		if os.IsNotExist(err) {
			return fmt.Errorf("%s does not exist", abs)
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", abs)
		}
		return nil
	}
}

func validateEmail(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errors.New("email is required")
	}
	if !strings.Contains(s, "@") {
		return fmt.Errorf("%q is not an address", s)
	}
	return nil
}

func required(label string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%s is required", label)
		}
		return nil
	}
}
