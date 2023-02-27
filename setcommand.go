package ghu

import (
	"bytes"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

const (
	gitConfigPath = ".git/config"
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
		conf, err := os.ReadFile(gitConfigPath)
		if err != nil {
			logrus.Fatal(err)
		}

		confBuf := bytes.NewBuffer(conf)

		replacedConf, _, err := Set(username, "", confBuf, nil)
		if err != nil {
			logrus.Fatal(err)
		}

		if err := os.WriteFile(gitConfigPath, []byte(replacedConf), os.ModePerm); err != nil {
			logrus.Fatal(err)
		}
	},
}
