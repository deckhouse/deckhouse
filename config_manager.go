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

	RepoUpdated = make(chan map[string]string, 1)
	ModulesUpdated = make(chan []map[string]string, 1)

	RepoUpdated <- map[string]string{
		"url": "https://github.com/deckhouse/deckhouse-scripts",
	}
	ModulesUpdated <- []map[string]string{
		map[string]string{
			"name": "mymodule",
		},
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

	ticker := time.NewTicker(time.Duration(60) * time.Second)

	for {
		select {
		case <-ticker.C:
			rlog.Debugf("REPOUPDATE old=%v new=%v", lastRepo, repoS[i])

			RepoUpdated <- repoS[i]

			lastRepo = repoS[i]

			i = (i + 1) % len(repoS)
		}
	}
}
