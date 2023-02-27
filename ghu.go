package ghu

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ReplaceUsername replaces an existing GitHub username with new one.
func ReplaceUsername(conf io.Reader, username string) (string, error) {
	return replace(conf, "\t", "name = ", username)
}

// ReplaceSSHKey replaces an existing GitHub ssh key with new one.
func ReplaceSSHKey(conf io.Reader, sshKey string) (string, error) {
	return replace(conf, "  ", "IdentityFile ~/.ssh/", sshKey)
}

// replace value in given conf.
func replace(conf io.Reader, lineIndent, pattern, value string) (string, error) {
	var replaced string

	scanner := bufio.NewScanner(conf)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		lineToWrite := line + "\n"

		if strings.Contains(line, pattern) {
			lineToWrite = fmt.Sprintf("%v%v%v\n", lineIndent, pattern, value)
		}

		replaced += lineToWrite
	}

	return replaced, nil
}
