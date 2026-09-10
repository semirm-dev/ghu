package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is injected at build time from the VERSION file via
// -ldflags "-X github.com/semirm-dev/ghu/cli.Version=$(cat VERSION)".
var Version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the ghu version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), Version)
			return err
		},
	}
}
