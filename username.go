package ghu

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"strings"
)

const (
	gitConfigPath = ".git/config"
)

func ReplaceUsernameConfig(value string, replacer func(conf io.Reader, value string) (string, error)) error {
	conf, err := os.ReadFile(gitConfigPath)
	if err != nil {
		return err
	}

	replacedConf, err := replacer(bytes.NewBuffer(conf), value)
	if err != nil {
		return err
	}

	return os.WriteFile(gitConfigPath, []byte(replacedConf), os.ModePerm)
}

// UsernameReplacer replaces an existing GitHub username with new one.
func UsernameReplacer(conf io.Reader, username string) (string, error) {
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
