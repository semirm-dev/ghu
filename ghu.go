package ghu

import (
	"github.com/sirupsen/logrus"
	"strings"
)

func ProcessSet(username, sshKey string) error {
	if strings.TrimSpace(username) == "" {
		if err := setUsername(username); err != nil {
			return err
		}
	}

	if strings.TrimSpace(sshKey) == "" {
		if err := setSSHKey(sshKey); err != nil {
			return err
		}
	}

	return nil
}

func ProcessDelete(username, sshKey string) error {
	if strings.TrimSpace(username) == "" {
		if err := deleteUsername(username); err != nil {
			return err
		}
	}

	if strings.TrimSpace(sshKey) == "" {
		if err := deleteSSHKey(sshKey); err != nil {
			return err
		}
	}

	return nil
}

func setUsername(username string) error {
	logrus.Infof("setting username: %s", username)
	return nil
}

func setSSHKey(sshKey string) error {
	logrus.Infof("setting ssh key: %s", sshKey)
	return nil
}

func deleteUsername(username string) error {
	logrus.Infof("deleting username: %s", username)
	return nil
}

func deleteSSHKey(sshKey string) error {
	logrus.Infof("deleting ssh key: %s", sshKey)
	return nil
}
