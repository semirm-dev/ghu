package command

// Making `help` work the same way everywhere.

import "github.com/spf13/cobra"

// WithHelp adds subcommands to a parent, plus the `help` command that lists
// them. Every parent command in ghu is built this way, so `help` is available
// at whatever depth you happen to be thinking at.
func WithHelp(parent *cobra.Command, subs ...*cobra.Command) *cobra.Command {
	parent.AddCommand(subs...)
	parent.AddCommand(helpFor(parent))
	return parent
}

// helpFor returns a `help` subcommand for a parent command.
//
// Cobra gives the root one for free, so `ghu help` and `ghu help profile add`
// both work, but a parent command deeper in the tree gets nothing -- leaving
// `ghu help` working and `ghu profile help` failing, which is the kind of
// inconsistency you only find by typing the wrong one.
func helpFor(parent *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command here",
		RunE: func(_ *cobra.Command, args []string) error {
			target, _, err := parent.Find(args)
			if err != nil {
				return err
			}
			return target.Help()
		},
	}
}
