package docker_registry_manager

import (
	"fmt"
	"os"
	"time"

	registryclient "github.com/flant/docker-registry-client/registry"
	"github.com/romana/rlog"

	"github.com/deckhouse/deckhouse/antiopa/kube"
)

var (
	// новый id образа с тем же именем
	// (смена самого имени образа будет обрабатываться самим Deployment'ом автоматом)
	ImageUpdated     chan string
	AntiopaImageId   string
	AntiopaImageName string
	AntiopaImageInfo DockerImageInfo
	PodName          string

	DockerRegistry *registryclient.Registry
)

// TODO данные для доступа к registry серверам нужно хранить в secret-ах.
// TODO по imageInfo.Registry брать данные и подключаться к нужному registry.
// Пока известно, что будет только registry.flant.com
var DockerRegistryInfo = map[string]map[string]string{
	"registry.flant.com": map[string]string{
		"url":      "https://registry.flant.com",
		"user":     "oauth2",
		"password": "qweqwe",
	},
	// minikube specific
	"localhost:5000": map[string]string{
		"url": "http://kube-registry.kube-system.svc.cluster.local:5000",
	},
}

// InitRegistryManager получает имя образа по имени пода и запрашивает id этого образа.
func InitRegistryManager(hostname string) error {
	if kube.IsRunningOutOfKubeCluster() {
		return nil
	}

	rlog.Debug("Init registry manager")

	// TODO Пока для доступа к registry.flant.com передаётся временный токен через переменную среды
	GitlabToken := os.Getenv("GITLAB_TOKEN")
	DockerRegistryInfo["registry.flant.com"]["password"] = GitlabToken

	ImageUpdated = make(chan string)
	AntiopaImageName = kube.KubeGetPodImageName(hostname)

	var err error
	AntiopaImageInfo, err = DockerParseImageName(AntiopaImageName)
	if err != nil {
		return fmt.Errorf("problem parsing image %s: %v", AntiopaImageName, err)
	}

	url := ""
	user := ""
	password := ""
	if info, hasInfo := DockerRegistryInfo[AntiopaImageInfo.Registry]; hasInfo {
		url = info["url"]
		user = info["user"]
		password = info["password"]
	}
	// Создать клиента для подключения к docker-registry
	// в единственном экземляре
	DockerRegistry = NewDockerRegistry(url, user, password)

	AntiopaImageId, err = DockerRegistryGetImageId(AntiopaImageInfo, DockerRegistry)
	if err != nil {
		return err
	}

	return nil
}

// RunRegistryManager каждые 10 секунд проверяет
// не изменился ли id образа.
func RunRegistryManager() {
	if kube.IsRunningOutOfKubeCluster() {
		return
	}

	rlog.Debug("Run registry manager")

	ticker := time.NewTicker(time.Duration(10) * time.Second)

	for {
		select {
		case <-ticker.C:
			rlog.Debugf("Checking registry for updates")

			imageID, err := DockerRegistryGetImageId(AntiopaImageInfo, DockerRegistry)
			if err != nil {
				rlog.Errorf("REGISTRY Cannot check image id: %v", err)
			} else {
				if imageID != AntiopaImageId {
					ImageUpdated <- imageID
				}
			}
		}
	}
}
