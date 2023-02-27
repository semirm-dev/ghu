package ghu

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ReplaceUsername replaces an existing GitHub username with new one.
func ReplaceUsername(username string, ghConf io.Reader) (string, error) {
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

	return conf, nil
}

// ReplaceSSHKey replaces an existing GitHub ssh key with new one.
func ReplaceSSHKey(sshKey string, sshConf io.Reader) (string, error) {
	var conf string

	scanner := bufio.NewScanner(sshConf)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		lineToWrite := line + "\n"

		if strings.Contains(line, "IdentityFile ~/.ssh/") {
			lineToWrite = fmt.Sprintf("  IdentityFile ~/.ssh/%v\n", sshKey)
		}

		conf += lineToWrite
	}

	return conf, nil
}
