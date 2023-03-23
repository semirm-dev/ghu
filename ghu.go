package ghu

import (
	"bytes"
	"errors"
	"github.com/gobackpack/colr"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"os/exec"
	"strings"
)

func RefreshSSHAgent() error {
	cmd := exec.Command("bash", "-c", "eval $(ssh-agent -s)")
	out, err := cmd.Output()
	if err != nil {
		return err
	}

	logrus.Infof(colr.Green("%s"), out)

	return nil
}

func createGhuConfigIfNotExists(confPath string) error {
	if fileExists(confPath) {
		return nil
	}

	logrus.Infof("ghu config file [%s] does not exist, creating new from template", confPath)

	if err := createGhuDirIfNotExists(strings.TrimSuffix(confPath, "/config.yaml")); err != nil {
		return err
	}

	if err := writeGhuConfig(confPath); err != nil {
		return err
	}

	logrus.Infof("new ghu config file [%s] created", confPath)

	return nil
}

func createGhuDirIfNotExists(ghuPath string) error {
	if _, err := os.Stat(ghuPath); errors.Is(err, os.ErrNotExist) {
		logrus.Infof("creating .ghu dir: %s", ghuPath)

		if err = os.MkdirAll(ghuPath, os.ModePerm); err != nil {
			return err
		}
	}

	return nil
}

func writeGhuConfig(confPath string) error {
	writtenConf, err := os.ReadFile("fixtures/ghuconfig.yaml")
	if err != nil {
		return err
	}

	f, err := os.Create(confPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, bytes.NewBuffer(writtenConf))
	return err
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
