package ghu

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ReplaceUsername replaces an existing GitHub username with new one.
func ReplaceUsername(conf io.Reader, username, host string) (string, error) {
	return replace(conf, "\t", "name = ", username, host)
}

// ReplaceSSHKey replaces an existing GitHub ssh key with new one.
func ReplaceSSHKey(conf io.Reader, sshKey, host string) (string, error) {
	return replace(conf, "  ", "IdentityFile ~/.ssh/", sshKey, host)
}

// replace value in given conf.
func replace(conf io.Reader, lineIndent, pattern, value, host string) (string, error) {
	var replaced string

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

		if strings.Contains(line, pattern) && (host == "" || host != "" && host == previousHost) {
			lineToWrite = fmt.Sprintf("%v%v%v\n", lineIndent, pattern, value)
		}

		replaced += lineToWrite
	}

	return replaced, nil
}
