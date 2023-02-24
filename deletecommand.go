package ghu

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	// delete
	deleteCmd.PersistentFlags().StringVarP(&username, "username", "u", "", "username to use")
	deleteCmd.PersistentFlags().StringVarP(&sshKey, "key", "k", "", "ssh key to use")
	rootCmd.AddCommand(deleteCmd)
}

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete value",
	Long:  "Delete value",
	Run: func(cmd *cobra.Command, args []string) {
		logrus.Infof("deleting username: %s, ssh key: %s", username, sshKey)
	},
}
