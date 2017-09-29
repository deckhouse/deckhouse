package main

import (
	"time"

	"github.com/romana/rlog"
)

var (
	RepoUpdated    chan map[string]string
	ModulesUpdated chan []map[string]string
)

/*
repo => json{url: "...", ref: "..." || "master"}
modules => json[{name: "", entrypoint: "path-to-sh" || "ctl.sh"}, ...]
*/

func InitConfigManager() {
	rlog.Info("Init config manager")

	RepoUpdated <- map[string]string{
		"url": "https://github.com/deckhouse/deckhouse-scripts",
	}
}

func RunConfigManager() {
	rlog.Info("Run config manager")

	repoS := []map[string]string{
		map[string]string{
			"url": "https://github.com/deckhouse/deckhouse-scripts",
		},
		map[string]string{
			"url": "https://github.com/deckhouse/deckhouse-scripts",
			"ref": "no-such-ref",
		},
		map[string]string{
			"url": "no-such-url",
		},
	}

	lastRepo := map[string]string{
		"url": "https://github.com/deckhouse/deckhouse-scripts",
	}
	i := 0

	for {
		time.Sleep(time.Duration(60) * time.Second)

		rlog.Debugf("REPOUPDATE old=%v new=%v", lastRepo, repoS[i])

		RepoUpdated <- repoS[i]

		lastRepo = repoS[i]

		i = (i + 1) % len(repoS)
	}
}
