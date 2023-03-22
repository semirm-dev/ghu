package commands

import (
	"github.com/semirm-dev/ghu"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	version = "0.0.1"
)

var (
	usernameToSet string
	sshKeyToSet   string
	sshHost       string
)

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(refreshAgentCmd)
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
		displayVersion()
	},
}

var refreshAgentCmd = &cobra.Command{
	Use:   "ragent",
	Short: "Refresh ssh agent",
	Long:  "Refresh ssh agent",
	Run: func(cmd *cobra.Command, args []string) {
		if err := ghu.RefreshSSHAgent(); err != nil {
			logrus.Error(err)
		}
	},
}

// Execute will trigger root command.
func Execute() error {
	return rootCmd.Execute()
}

func displayVersion() {
	logrus.Infof("version: %s", version)
}
