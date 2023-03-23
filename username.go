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
	GitConfigPath = ".git/config"
)

// UsernameReplacerFunc will replace GitHub username
type UsernameReplacerFunc func(conf io.Reader, value string) (string, error)

func ReplaceUsernameConfig(value string, replacerFunc UsernameReplacerFunc) error {
	conf, err := os.ReadFile(GitConfigPath)
	if err != nil {
		return err
	}

	logrus.Infof("current Github config: \n%s\n", conf)

	replacedConf, err := replacerFunc(bytes.NewBuffer(conf), value)
	if err != nil {
		return err
	}

	if strings.TrimSpace(replacedConf) == "" {
		logrus.Info("nothing to replace")
		return nil
	}

	logrus.Infof("new GitHub config to write: \n%s\n", replacedConf)

	return os.WriteFile(GitConfigPath, []byte(replacedConf), os.ModePerm)
}

// UsernameReplacer replaces an existing GitHub username with new one.
func UsernameReplacer(conf io.Reader, username string) (string, error) {
	var replaced string
	lineIndent := "\t"
	pattern := "name = "

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
