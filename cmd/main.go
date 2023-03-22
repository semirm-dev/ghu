package main

import (
	"github.com/semirm-dev/ghu/commands"
	"github.com/sirupsen/logrus"
)

func main() {
	if err := commands.Execute(); err != nil {
		logrus.Fatal(err)
	}
}
