package main

import (
	"time"

	"github.com/romana/rlog"
)

var (
	// (старый-репо, новый-репо)
	RepoUpdated chan RepoUpdate

	// (старый-список, новый-список)
	ModulesUpdated chan ModulesUpdate
)

type RepoUpdate struct {
	OldRepo map[string]string
	Repo    map[string]string
}

type ModulesUpdate struct {
	OldModules []map[string]string
	Modules    []map[string]string
}

/*
repo => json{url: "...", ref: "..." || "master"}
modules => json[{name: "", entrypoint: "path-to-sh" || "ctl.sh"}, ...]
*/

func InitConfigManager() {
	rlog.Info("Init config manager")

	RepoUpdated <- map[string]string{"url": "https://github.com/deckhouse/deckhouse-scripts"}
}

func RunConfigManager() {
	rlog.Info("Run config manager")

	for {
		time.Sleep(time.Duration(1) * time.Second)
	}
}
