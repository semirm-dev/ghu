package ghu

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	// set
	setCmd.PersistentFlags().StringVarP(&username, "username", "u", "", "username to use")
	setCmd.PersistentFlags().StringVarP(&sshKey, "key", "k", "", "ssh key to use")
	rootCmd.AddCommand(setCmd)
}

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Set value",
	Long:  "Set value",
	Run: func(cmd *cobra.Command, args []string) {
		if err := ProcessSet(username, sshKey); err != nil {
			logrus.Error(err)
		}
	},
}
