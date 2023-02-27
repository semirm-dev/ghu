package ghu

import (
	"bytes"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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
			if err := replaceUsername(gitConfigPath, uName); err != nil {
				logrus.Error(err)
			}
		}

		key := strings.TrimSpace(sshKey)
		if key != "" {
			if err := replaceSSHKey(sshConfPath, key); err != nil {
				logrus.Error(err)
			}
		}
	},
}

func replaceUsername(confPath, newUsername string) error {
	conf, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}

	confBuf := bytes.NewBuffer(conf)

	replacedConf, err := ReplaceUsername(newUsername, confBuf)
	if err != nil {
		return err
	}

	if err := os.WriteFile(confPath, []byte(replacedConf), os.ModePerm); err != nil {
		return err
	}

	return nil
}

func replaceSSHKey(confPath, newKey string) error {
	conf, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}

	confBuf := bytes.NewBuffer(conf)

	replacedConf, err := ReplaceUsername(newKey, confBuf)
	if err != nil {
		return err
	}

	if err := os.WriteFile(confPath, []byte(replacedConf), os.ModePerm); err != nil {
		return err
	}

	return nil
}
