package docker_registry_manager

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	registryclient "github.com/flant/docker-registry-client/registry"
	"github.com/romana/rlog"

	"github.com/deckhouse/deckhouse/antiopa/kube_helper"

	utils_file "github.com/flant/shell-operator/pkg/utils/file"
)

const DefaultRegistrySecretPath = "/etc/registrysecret"

type DockerRegistryManager interface {
	SetErrorCallback(errorCb func())
	Run()
}

var (
	// новый id образа с тем же именем
	// (смена самого имени образа будет обрабатываться самим Deployment'ом автоматом)
	ImageUpdated chan string

	// Path to a mounted docker registry secret
	RegistrySecretPath string

	RegistryToUrlMapping map[string]string
)

type MainRegistryManager struct {
	AntiopaImageDigest string
	AntiopaImageName   string
	AntiopaImageInfo   DockerImageInfo
	PodHostname        string
	// клиент для обращений к
	DockerRegistry *registryclient.Registry
	// счётчик ошибок обращений к registry
	ErrorCounter int
	// callback вызывается в случае ошибки
	ErrorCallback func()
}

// InitRegistryManager получает имя образа по имени пода и запрашивает id этого образа.
// TODO вытащить token и host в секрет
func Init(hostname string) (DockerRegistryManager, error) {
	if os.Getenv("ANTIOPA_WATCH_REGISTRY") == "false" {
		rlog.Infof("Antiopa: registry manager disabled with ANTIOPA_WATCH_REGISTRY=false.")
		return nil, nil
	}

	rlog.Debug("Init registry manager")

	RegistrySecretPath = os.Getenv("ANTIOPA_REGISTRY_SECRET_PATH")
	if RegistrySecretPath == "" {
		RegistrySecretPath = DefaultRegistrySecretPath
	}
	rlog.Infof("Load registry auths from %s dir", RegistrySecretPath)

	// Load json from file /etc/registrysecret/.dockercfg
	if exists, err := utils_file.DirExists(RegistrySecretPath); !exists {
		rlog.Errorf("Error accessing registry secret directory: %s, watcher is disabled now", err)
		return nil, nil
	}

	var readErr error
	var secretBytes []byte
	secretBytes, readErr = ioutil.ReadFile(path.Join(RegistrySecretPath, ".dockercfg"))
	if readErr != nil {
		secretBytes, readErr = ioutil.ReadFile(path.Join(RegistrySecretPath, ".dockerconfigjson"))
		if readErr != nil {
			return nil, fmt.Errorf("Cannot read registry secret from .docker[cfg,configjson]: %s", readErr)
		}
	}

	err := LoadDockerRegistrySecret(secretBytes)
	if err != nil {
		return nil, fmt.Errorf("Cannot load registry secret: %s", err)
	}

	registries := ""
	for k := range DockerCfgAuths {
		registries = registries + ", " + k
	}
	rlog.Infof("Load auths for: %s", registries)

	// FIXME: hack for minikube testing
	RegistryToUrlMapping = map[string]string{
		"localhost:5000": "http://kube-registry.kube-system.svc.cluster.local:5000",
	}

	ImageUpdated = make(chan string)

	rlog.Infof("Registry manager initialized")

	return &MainRegistryManager{
		ErrorCounter: 0,
		PodHostname:  hostname,
	}, nil
}

// Запускает проверку каждые 10 секунд, не изменился ли id образа.
func (rm *MainRegistryManager) Run() {
	if os.Getenv("ANTIOPA_WATCH_REGISTRY") == "false" {
		return
	}

	rlog.Infof("Registry manager: start")

	ticker := time.NewTicker(time.Duration(10) * time.Second)

	rm.CheckIsImageUpdated()
	for {
		select {
		case <-ticker.C:
			rm.CheckIsImageUpdated()
		}
	}
}

func (rm *MainRegistryManager) SetErrorCallback(errorCb func()) {
	rm.ErrorCallback = errorCb
}

// Основной метод проверки обновления образа.
// Метод запускается периодически. Вначале пытается достучаться до kube-api
// и по имени Pod-а получить имя и digest его образа. Когда digest получен, то
// обращается в registry и по имени образа смотрит, изменился ли digest. Если да,
// то отправляет новый digest в канал.
func (rm *MainRegistryManager) CheckIsImageUpdated() {
	// Первый шаг - получить имя и id образа из куба.
	// kube-api может быть недоступно, поэтому нужно периодически подключаться к нему.
	if rm.AntiopaImageName == "" {
		rlog.Debugf("Registry manager: retrieve image name and id from kube-api")
		podImageName, podImageId := kube_helper.KubeGetPodImageInfo(rm.PodHostname)
		if podImageName == "" {
			rlog.Debugf("Registry manager: error retrieving image name and id from kube-api. Will try again")
			return
		}

		var err error
		rm.AntiopaImageInfo, err = DockerParseImageName(podImageName)
		if err != nil {
			// Очень маловероятная ситуация, потому что Pod запустился, а имя образа из его спеки не парсится.
			rlog.Errorf("Registry manager: pod image name '%s' is invalid. Will try again. Error was: %v", podImageName, err)
			return
		}

		rm.AntiopaImageName = podImageName
		rm.AntiopaImageDigest, err = FindImageDigest(podImageId)
		if err != nil {
			rlog.Errorf("RegistryManager: %s", err)
			ImageUpdated <- "NO_DIGEST_FOUND"
			return
		}
	}

	// Второй шаг — после получения id начать мониторить его изменение в registry.
	// registry тоже может быть недоступен
	if rm.DockerRegistry == nil {
		rlog.Debugf("Registry manager: create docker registry client")
		var url, user, password string
		if info, hasInfo := DockerCfgAuths[rm.AntiopaImageInfo.Registry]; hasInfo {
			// FIXME Should we always use https here?
			if mappedUrl, hasKey := RegistryToUrlMapping[rm.AntiopaImageInfo.Registry]; hasKey {
				url = mappedUrl
			} else {
				url = fmt.Sprintf("https://%s", rm.AntiopaImageInfo.Registry)
			}
			user = info.Username
			password = info.Password
		}
		// Создать клиента для подключения к docker-registry
		// в единственном экземляре
		rm.DockerRegistry = NewDockerRegistry(url, user, password)
	}

	rlog.Debugf("Registry manager: checking registry for updates")
	digest, err := DockerRegistryGetImageDigest(rm.AntiopaImageInfo, rm.DockerRegistry)
	rm.SetOrCheckAntiopaImageDigest(digest, err)
}

// Сравнить запомненный digest образа с полученным из registry.
// Если отличаются — отправить полученный digest в канал.
// Если digest не был запомнен, то запомнить.
// Если была ошибка при опросе registry, то увеличить счётчик ошибок.
// Когда накопится 3 ошибки подряд, вывести ошибку и сбросить счётчик
func (rm *MainRegistryManager) SetOrCheckAntiopaImageDigest(digest string, err error) {
	// Если пришёл не валидный id или была ошибка — увеличить счётчик ошибок.
	// Сообщить в лог, когда накопится 3 ошибки подряд
	if err != nil || !IsValidImageDigest(digest) {
		rm.ErrorCallback()
		rm.ErrorCounter++
		if rm.ErrorCounter >= 3 {
			rlog.Errorf("Registry manager: registry request error: %s", err)
			rm.ErrorCounter = 0
		}
		return
	}
	if digest != rm.AntiopaImageDigest {
		ImageUpdated <- digest
	}
	rm.ErrorCounter = 0
}
