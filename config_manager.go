package main

import (
	"time"

	"github.com/romana/rlog"
)

var (
    // (старый-репо, новый-репо)
	RepoUpdated chan (map[string]interface{}, map[string]interface{})

	// (старый-список, новый-список)
	ModulesUpdated chan ([]map[string]interface{}, []map[string]interface{})
)

/*
repo => json{url: "...", ref: "..." || "master"}
modules => json[{name: "", entrypoint: "path-to-sh" || "ctl.sh"}, ...]
*/

func InitConfigManager() {
	rlog.Info("Init config manager")

    RepoUpdated<- map[string]interface{}{"url": "https://github.com/deckhouse/deckhouse-scripts"}
}

func RunConfigManager() {
	rlog.Info("Run config manager")

	for {
		time.Sleep(time.Duration(1) * time.Second)
	}
}
