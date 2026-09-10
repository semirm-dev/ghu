package ui

// Asking the person at the keyboard something, when there is one.

import (
	"os"

	"charm.land/huh/v2"
	"github.com/charmbracelet/x/term"
)

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
