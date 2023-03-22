package ghu

import (
	"bytes"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path/filepath"
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

// SyncConfigReplacer replaces both username and ssh key from ~/.ghu/config.yaml
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
