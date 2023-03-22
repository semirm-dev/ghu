package commands

import (
	"github.com/semirm-dev/ghu"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(syncCmd)
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Map current ghu config",
	Long:  "Map current ghu config",
	Run: func(cmd *cobra.Command, args []string) {
		if err := ghu.SyncConfig(ghu.SyncConfigReplacer); err != nil {
			logrus.Error(err)
		}
	},
}
