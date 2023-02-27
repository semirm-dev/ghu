package ghu

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func Set(username, sshKey string, ghConf io.Reader, sshConf io.Reader) (string, string, error) {
	var gh string
	if strings.TrimSpace(username) != "" {
		gh = replaceUsername(username, ghConf)
	}

	var ssh string
	if strings.TrimSpace(sshKey) != "" {
		ssh = replaceSSHKey(sshKey, sshConf)
	}

	return gh, ssh, nil
}

func replaceUsername(username string, ghConf io.Reader) string {
	var conf string

	scanner := bufio.NewScanner(ghConf)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		lineToWrite := line + "\n"

		if strings.Contains(line, "name = ") {
			lineToWrite = fmt.Sprintf("\tname = %v\n", username)
		}

		conf += lineToWrite
	}

	return conf
}

func replaceSSHKey(sshKey string, sshConf io.Reader) string {
	var conf string

	scanner := bufio.NewScanner(sshConf)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		lineToWrite := line + "\n"

		if strings.Contains(line, "IdentityFile ~/.ssh") {
			lineToWrite = fmt.Sprintf("  IdentityFile ~/.ssh/%v\n", sshKey)
		}

		conf += lineToWrite
	}

	return conf
}
