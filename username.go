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
		if strings.TrimSpace(line) == "" {
			replaced += "\n"
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
