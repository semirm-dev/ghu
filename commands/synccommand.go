package commands

import (
	"bytes"
	"errors"
	"github.com/semirm-dev/ghu"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	rootCmd.AddCommand(syncCmd)
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Map current ghu config",
	Long:  "Map current ghu config",
	Run: func(cmd *cobra.Command, args []string) {
		if err := syncConfig(ghu.SyncConfig); err != nil {
			logrus.Error(err)
		}
	},
}

func syncConfig(replacer func(conf io.Reader) error) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	confPath := filepath.Join(home, ghuConfPath)

	if err = createGhuConfIfNotExists(confPath); err != nil {
		return err
	}

	conf, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}

	return replacer(bytes.NewBuffer(conf))
}

func createGhuConfIfNotExists(confPath string) error {
	if fileExists(confPath) {
		return nil
	}

	logrus.Infof("ghu config file [%s] does not exist, creating new from template", confPath)

	ghuPath := strings.TrimSuffix(confPath, "/config.yaml")
	if _, err := os.Stat(ghuPath); errors.Is(err, os.ErrNotExist) {
		logrus.Infof("creating .ghu dir: %s", ghuPath)
		if err = os.MkdirAll(ghuPath, os.ModePerm); err != nil {
			return err
		}
	}

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
	if err != nil {
		return err
	}

	logrus.Infof("new ghu config file [%s] created", confPath)

	return nil
}
