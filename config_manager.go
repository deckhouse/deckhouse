package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/romana/rlog"

	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	RepoUpdated    chan map[string]string
	ModulesUpdated chan []map[string]string

	lastKnownRepoChecksum    string
	lastKnownModulesChecksum string
)

func getConfigMap() (*v1.ConfigMap, error) {
	configMap, err := KubernetesClient.CoreV1().ConfigMaps(KubernetesNamespace).Get("antiopa", meta_v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("ConfigMap '%s' is not found in namespace '%s'", "antiopa", KubernetesNamespace)
	}

	return configMap, nil
}

func calculateRepoChecksum(cm *v1.ConfigMap) string {
	hasher := md5.New()
	hasher.Write([]byte(cm.Data["repo"]))
	return hex.EncodeToString(hasher.Sum(nil))
}

func getRepo(cm *v1.ConfigMap) (map[string]string, error) {
	var res map[string]string

	if err := json.Unmarshal([]byte(cm.Data["repo"]), &res); err != nil {
		return nil, err
	}

	return res, nil
}

func calculateModulesChecksum(cm *v1.ConfigMap) string {
	hasher := md5.New()
	hasher.Write([]byte(cm.Data["modules"]))
	return hex.EncodeToString(hasher.Sum(nil))
}

func getModules(cm *v1.ConfigMap) ([]map[string]string, error) {
	var res []map[string]string

	if err := json.Unmarshal([]byte(cm.Data["modules"]), &res); err != nil {
		return nil, err
	}

	return res, nil
}

func InitConfigManager() {
	rlog.Info("Init config manager")

	RepoUpdated = make(chan map[string]string, 1)
	ModulesUpdated = make(chan []map[string]string, 1)

	if cm, err := getConfigMap(); err == nil {
		if repo, err := getRepo(cm); err == nil {
			lastKnownRepoChecksum = calculateRepoChecksum(cm)

			rlog.Debugf("UPDATEREPO:[%s] %v", lastKnownRepoChecksum, repo)

			RepoUpdated <- repo
		} else {
			rlog.Errorf("Bad repo configuration: %s", err)
		}

		if modules, err := getModules(cm); err == nil {
			lastKnownModulesChecksum = calculateModulesChecksum(cm)

			rlog.Debugf("UPDATEMODULES: [%s] %v", lastKnownModulesChecksum, modules)

			ModulesUpdated <- modules
		} else {
			rlog.Errorf("Bad modules configuration: %s", err)
		}
	} else {
		rlog.Warnf("Unable to get kubernetes ConfigMap: %s", err)
	}
}

func RunConfigManager() {
	rlog.Info("Run config manager")
	return

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
