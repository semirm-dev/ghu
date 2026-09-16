package cli

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// version is set by the release workflow from the tag it is building, with
// -X github.com/semirm-dev/ghu/internal/cli.version=X.Y.Z. It is the only
// thing that sets it: a Go module's version is its tag, so writing it down
// anywhere else would be a second copy to keep in agreement with the first.
var version = "dev"

// Version reports the version this binary was built from.
//
// A `go install` build records the module version it resolved -- an exact tag,
// or a pseudo-version for a commit past one -- which the ldflag cannot reach,
// so it is used where it exists. A build from a checkout has neither and says
// so.
func Version() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return version
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the ghu version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), Version())
			return err
		},
	}
}
