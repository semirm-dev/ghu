package ghu

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/gobackpack/colr"
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

	logrus.Infof("%s \n%s\n", colr.Cyan("current Github config:"), conf)

	replacedConf, err := replacerFunc(bytes.NewBuffer(conf), value)
	if err != nil {
		return err
	}

	if strings.TrimSpace(replacedConf) == "" {
		logrus.Info("nothing to replace")
		return nil
	}

	logrus.Infof("%s \n%s\n", colr.Green("new GitHub config to write:"), replacedConf)

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
			logrus.Infof(colr.Magenta("replacing old Github username [%s] with new username: [%s]"), oldUsername, username)
			lineToWrite = fmt.Sprintf("%v%v%v\n", lineIndent, pattern, username)
		}

		replaced += lineToWrite
	}

	return replaced, nil
}

func ReplaceUsernameConfigV2(value string, replacer io.ReadWriter, replacerFunc UsernameReplacerFunc) error {
	conf, err := io.ReadAll(replacer)
	if err != nil {
		return err
	}

	logrus.Infof("%s \n%s\n", colr.Cyan("current Github config:"), conf)

	replacedConf, err := replacerFunc(bytes.NewBuffer(conf), value)
	if err != nil {
		return err
	}

	if strings.TrimSpace(replacedConf) == "" {
		logrus.Info("nothing to replace")
		return nil
	}

	logrus.Infof("%s \n%s\n", colr.Green("new GitHub config to write:"), replacedConf)

	_, err = io.WriteString(replacer, replacedConf)
	return err
}

func FileUsernameReplacer(path string) io.ReadWriter {
	f, _ := os.OpenFile(path, os.O_RDWR, os.ModePerm)
	return f
}
