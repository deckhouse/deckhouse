package docker_registry_manager

import (
	"os"
	"time"

	registryclient "github.com/flant/docker-registry-client/registry"
	"github.com/romana/rlog"

	"github.com/deckhouse/deckhouse/antiopa/kube"
)

type DockerRegistryManager interface {
	SetErrorCallback(errorCb func())
	Run()
}

var (
	// новый id образа с тем же именем
	// (смена самого имени образа будет обрабатываться самим Deployment'ом автоматом)
	ImageUpdated chan string
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
// TODO вытащить token и host в секрет
func Init(hostname string) (DockerRegistryManager, error) {
	if kube.IsRunningOutOfKubeCluster() {
		rlog.Infof("Antiopa is running out of cluster. No registry manager required.")
		return nil, nil
	}

	rlog.Debug("Init registry manager")

	// TODO Пока для доступа к registry.flant.com передаётся временный токен через переменную среды
	GitlabToken := os.Getenv("GITLAB_TOKEN")
	DockerRegistryInfo["registry.flant.com"]["password"] = GitlabToken

	ImageUpdated = make(chan string)

	rlog.Infof("Registry manager initialized")

	return &MainRegistryManager{
		ErrorCounter: 0,
		PodHostname:  hostname,
	}, nil
}

// Запускает проверку каждые 10 секунд, не изменился ли id образа.
func (rm *MainRegistryManager) Run() {
	if kube.IsRunningOutOfKubeCluster() {
		return
	}

	rlog.Infof("Registry manager: start watch for image '%s'", rm.AntiopaImageName)

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
		podImageName, podImageId := kube.KubeGetPodImageInfo(rm.PodHostname)
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
		rm.AntiopaImageDigest = FindImageDigest(podImageId)
	}

	// Второй шаг — после получения id начать мониторить его изменение в registry.
	// registry тоже может быть недоступен
	if rm.DockerRegistry == nil {
		rlog.Debugf("Registry manager: create docker registry client")
		var url, user, password string
		if info, hasInfo := DockerRegistryInfo[rm.AntiopaImageInfo.Registry]; hasInfo {
			url = info["url"]
			user = info["user"]
			password = info["password"]
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
