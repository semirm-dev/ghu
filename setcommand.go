package ghu

import (
	"bytes"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	gitConfigPath = ".git/config"
	sshConfPath   = ".ssh/config"
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
			if err := replaceUsername(uName, ReplaceUsername); err != nil {
				logrus.Error(err)
			}
		}

		if key := strings.TrimSpace(sshKeyToSet); key != "" {
			if err := replaceSsh(key, sshHost, ReplaceSSHKey); err != nil {
				logrus.Error(err)
			}

			if err := refreshSSHAgent(); err != nil {
				logrus.Error(err)
			}
		}
	},
}

func replaceUsername(value string, replacer func(conf io.Reader, value string) (string, error)) error {
	conf, err := os.ReadFile(gitConfigPath)
	if err != nil {
		return err
	}

	confBuf := bytes.NewBuffer(conf)

	replacedConf, err := replacer(confBuf, value)
	if err != nil {
		return err
	}

	if err := os.WriteFile(gitConfigPath, []byte(replacedConf), os.ModePerm); err != nil {
		return err
	}

	return nil
}

func replaceSsh(value, host string, replacer func(conf io.Reader, value, host string) (string, error)) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	confPath := filepath.Join(home, sshConfPath)

	conf, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}

	confBuf := bytes.NewBuffer(conf)

	replacedConf, err := replacer(confBuf, value, host)
	if err != nil {
		return err
	}

	if err := os.WriteFile(confPath, []byte(replacedConf), os.ModePerm); err != nil {
		return err
	}

	return nil
}
