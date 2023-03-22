package ghu

import (
	"github.com/sirupsen/logrus"
	"io"
)

type Config struct{}

func SyncConfig(conf io.Reader) error {
	confContent, err := io.ReadAll(conf)
	if err != nil {
		return err
	}

	logrus.Infof("current ghu config: %s", confContent)

	return nil
}
