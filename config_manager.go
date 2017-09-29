package main

import (
	"time"

	"github.com/romana/rlog"
)

var (
	ConfigUpdated chan *Config
	CurrentConfig *Config
)

type Repo struct {
	Url    string
	Branch string
}

type Module struct {
	Name          string
	Path          string
	EntryPointBin string
	WorkDir       string
}

type Config struct {
	Repo Repo
}

func InitConfigManager() {
	rlog.Info("Init config manager")

	CurrentConfig = &Config{Repo{"github.com/deckhouse/deckhouse-scripts", "master"}}
}

func RunConfigManager() {
	rlog.Info("Run config manager")

	for {
		time.Sleep(time.Duration(1) * time.Second)
	}
}
