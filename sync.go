package ghu

import (
	"bytes"
	"errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	ghuConfPath = ".ghu/config.yaml"
)

type Config struct {
	Username string `yaml:"username"`
	SSH      string `yaml:"ssh"`
	Host     string `yaml:"host"`
}

func SyncConfig(replacer func(conf io.Reader) error) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	confPath := filepath.Join(home, ghuConfPath)

	if err = createGhuConfigIfNotExists(confPath); err != nil {
		return err
	}

	conf, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}

	return replacer(bytes.NewBuffer(conf))
}

func SyncConfigReplacer(ghuConf io.Reader) error {
	confContent, err := io.ReadAll(ghuConf)
	if err != nil {
		return err
	}

	var conf Config
	if err = yaml.Unmarshal(confContent, &conf); err != nil {
		return err
	}

	logrus.Infof("syncing ghu configuration: %v", conf)

	if err = ReplaceUsernameConfig(conf.Username, UsernameReplacer); err != nil {
		return err
	}

	if err = ReplaceSshConfig(conf.SSH, conf.Host, SSHKeyReplacer); err != nil {
		return err
	}

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
