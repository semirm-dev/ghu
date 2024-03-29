package ghu

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/gobackpack/colr"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	sshConfPath = ".ssh/config"
)

// SSHKeyReplacerFunc will replace GitHub ssh key
type SSHKeyReplacerFunc func(conf io.Reader, value, host string) (string, error)

func ReplaceSshConfig(value, host string, replacerFunc SSHKeyReplacerFunc) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	confPath := filepath.Join(home, sshConfPath)

	conf, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}

	logrus.Infof("%s \n%s\n", colr.Cyan("current ssh config:"), conf)

	replacedConf, err := replacerFunc(bytes.NewBuffer(conf), value, host)
	if err != nil {
		return err
	}

	if strings.TrimSpace(replacedConf) == "" {
		logrus.Info(colr.Yellow("nothing to replace"))
		return nil
	}

	logrus.Infof("%s \n%s\n", colr.Green("new ssh config to write:"), replacedConf)

	return os.WriteFile(confPath, []byte(replacedConf), os.ModePerm)
}

// SSHKeyReplacer replaces an existing GitHub ssh key with new one.
func SSHKeyReplacer(conf io.Reader, sshKey, host string) (string, error) {
	var replaced string
	lineIndent := "  "
	pattern := "IdentityFile ~/.ssh/"

	scanner := bufio.NewScanner(conf)
	var previousHost string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			replaced += "\n"
			continue
		}

		lineToWrite := line + "\n"

		if strings.Contains(line, "Host ") {
			previousHost = strings.TrimPrefix(line, "Host ")
		}

		if strings.Contains(line, pattern) && (host == "" || host == previousHost) {
			oldSshKey := strings.TrimPrefix(strings.TrimSpace(line), pattern)
			logrus.Infof(colr.Magenta("replacing old ssh key [%s] with new ssh key: [%s], for host: [%s]"), oldSshKey, sshKey, host)
			lineToWrite = fmt.Sprintf("%v%v%v\n", lineIndent, pattern, sshKey)
		}

		replaced += lineToWrite
	}

	return replaced, nil
}
