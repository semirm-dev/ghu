package cli

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// version is where ghu's version is written, and the only place it is. A v*
// tag is checked against it before a release publishes, the way sigi's tag is
// checked against build.zig.zon. Bump it in the commit that prepares a release.
const version = "0.2.0"

// Version reports the version this binary was built from.
//
// A `go install` build records the module version it resolved -- an exact tag,
// or a pseudo-version for a commit past one -- which describes the commit
// rather than the last release, so it wins where it exists. That is the path
// the README recommends, and it used to report "dev". A build from a checkout
// has no module version (the Makefile passes -buildvcs=false to keep release
// binaries reproducible), and falls back to the constant.
func Version() string {
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
