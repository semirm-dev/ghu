package ghu

import (
	"bytes"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io"
	"os"
	"strings"
)

const (
	gitConfigPath = ".git/config"
	sshConfPath   = ""
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
		uName := strings.TrimSpace(username)
		if uName != "" {
			if err := replaceFunc(gitConfigPath, uName, ReplaceUsername); err != nil {
				logrus.Error(err)
			}
		}

		key := strings.TrimSpace(sshKey)
		if key != "" {
			if err := replaceFunc(sshConfPath, key, ReplaceSSHKey); err != nil {
				logrus.Error(err)
			}
		}
	},
}

// replaceFunc will replace an existing value from the confPath with the new value using given replacer
func replaceFunc(confPath, value string, replacer func(conf io.Reader, value string) (string, error)) error {
	conf, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}

	confBuf := bytes.NewBuffer(conf)

	replacedConf, err := replacer(confBuf, value)
	if err != nil {
		return err
	}

	if err := os.WriteFile(confPath, []byte(replacedConf), os.ModePerm); err != nil {
		return err
	}

	return nil
}
