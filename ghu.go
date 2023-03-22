package ghu

import (
	"bufio"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"strings"
)

// ReplaceUsername replaces an existing GitHub username with new one.
func ReplaceUsername(conf io.Reader, username string) (string, error) {
	var replaced string
	lineIndent := "\t"
	pattern := "name = "

	logrus.Infof("new Github username: [%s]", username)

	scanner := bufio.NewScanner(conf)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		lineToWrite := line + "\n"

		if strings.Contains(line, pattern) {
			oldUsername := strings.TrimPrefix(strings.TrimSpace(line), pattern)
			logrus.Infof("replacing old Github username [%s] with new username: [%s]", oldUsername, username)
			lineToWrite = fmt.Sprintf("%v%v%v\n", lineIndent, pattern, username)
		}

		replaced += lineToWrite
	}

	return replaced, nil
}

// ReplaceSSHKey replaces an existing GitHub ssh key with new one.
func ReplaceSSHKey(conf io.Reader, sshKey, host string) (string, error) {
	var replaced string
	lineIndent := "  "
	pattern := "IdentityFile ~/.ssh/"

	logrus.Infof("new ssh key: [%s], for host: [%s]", sshKey, host)

	scanner := bufio.NewScanner(conf)
	var previousHost string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		lineToWrite := line + "\n"

		if strings.Contains(line, "Host ") {
			previousHost = strings.TrimPrefix(line, "Host ")
		}

		if strings.Contains(line, pattern) && (host == "" || host == previousHost) {
			oldSshKey := strings.TrimPrefix(strings.TrimSpace(line), pattern)
			logrus.Infof("replacing old ssh key [%s] with new ssh key: [%s], for host: [%s]", oldSshKey, sshKey, host)
			lineToWrite = fmt.Sprintf("%v%v%v\n", lineIndent, pattern, sshKey)
		}

		replaced += lineToWrite
	}

	return replaced, nil
}
