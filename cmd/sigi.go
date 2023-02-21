package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var username string

func init() {
	ghuRoot.Flags().StringVarP(&username, "username", "u", "", "username to use")
}

var ghuRoot = &cobra.Command{
	Use:   "",
	Short: "GitHub User",
	Long:  `GitHub User`,
	Run: func(cmd *cobra.Command, args []string) {
		logrus.Info("ghu")
	},
}

// Execute will trigger root command.
func Execute() error {
	return ghuRoot.Execute()
}
