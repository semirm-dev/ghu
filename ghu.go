package ghu

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	version = "0.0.1"
)

var (
	username string
	sshKey   string
	path     string
)

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(showCmd)
}

var rootCmd = &cobra.Command{
	Use:   "",
	Short: "GitHub User",
	Long:  "GitHub User",
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display ghu version",
	Long:  "Display ghu version",
	Run: func(cmd *cobra.Command, args []string) {
		logrus.Infof("version: %s", version)
	},
}

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Display ghu active config",
	Long:  "Display ghu active config",
	Run: func(cmd *cobra.Command, args []string) {
		logrus.Infof("active configuration...")
	},
}

// Execute will trigger root command.
func Execute() error {
	return rootCmd.Execute()
}
