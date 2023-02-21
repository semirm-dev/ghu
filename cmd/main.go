package main

import (
	"github.com/semirm-dev/ghu"
	"github.com/sirupsen/logrus"
)

func main() {
	if err := ghu.Execute(); err != nil {
		logrus.Fatal(err)
	}
}
