package commands

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
)

const (
	version       = "0.0.1"
	gitConfigPath = ".git/config"
	sshConfPath   = ".ssh/config"
	ghuConfPath   = ".ghu/config.yaml"
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
		if err := refreshSSHAgent(); err != nil {
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

func refreshSSHAgent() error {
	cmd := exec.Command("bash", "-c", "eval \"$(ssh-agent -s)\"")
	return cmd.Run()
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
