package ghu

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	// set
	generateCmd.PersistentFlags().StringVarP(&path, "path", "p", "", "path where to generate new ssh key")
	rootCmd.AddCommand(generateCmd)
}

var generateCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate ssh key",
	Long:  "Generate ssh key",
	Run: func(cmd *cobra.Command, args []string) {
		logrus.Infof("generating new ssh key: %s", path)
	},
}
