package commands

import (
	"github.com/semirm-dev/ghu"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"strings"
)

func init() {
	// set
	setCmd.PersistentFlags().StringVarP(&usernameToSet, "username", "u", "", "username to use")
	setCmd.PersistentFlags().StringVarP(&sshKeyToSet, "key", "k", "", "ssh key to use")
	setCmd.PersistentFlags().StringVarP(&sshHost, "host", "", "", "ssh host")
	rootCmd.AddCommand(setCmd)
}

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Set value",
	Long:  "Set value",
	Run: func(cmd *cobra.Command, args []string) {
		if uName := strings.TrimSpace(usernameToSet); uName != "" {
			if err := ghu.ReplaceUsernameConfig(uName, ghu.UsernameReplacer); err != nil {
				logrus.Error(err)
			}
		}

		if key := strings.TrimSpace(sshKeyToSet); key != "" {
			if err := ghu.ReplaceSshConfig(key, sshHost, ghu.SSHKeyReplacer); err != nil {
				logrus.Error(err)
				return
			}

			if err := ghu.RefreshSSHAgent(); err != nil {
				logrus.Error(err)
			}
		}
	},
}
