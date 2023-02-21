package ghu

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var username string

func init() {
	rootCmd.Flags().StringVarP(&username, "username", "u", "", "username to use")
}

var rootCmd = &cobra.Command{
	Use:   "",
	Short: "GitHub User",
	Long:  `GitHub User`,
	Run: func(cmd *cobra.Command, args []string) {
		logrus.Info("ghu")
	},
}

// Execute will trigger root command.
func Execute() error {
	return rootCmd.Execute()
}
