package docker_registry_manager

import (
	"os"
	"time"

	registryclient "github.com/flant/docker-registry-client/registry"
	"github.com/romana/rlog"

	"github.com/deckhouse/deckhouse/antiopa/kube"
)

var (
	// новый id образа с тем же именем
	// (смена самого имени образа будет обрабатываться самим Deployment'ом автоматом)
	ImageUpdated       chan string
	AntiopaImageDigest string
	AntiopaImageName   string
	AntiopaImageInfo   DockerImageInfo
	PodHostname        string

	DockerRegistry *registryclient.Registry

	// счётчик ошибок обращений к registry и последняя ошибка
	RegistryErrorCounter int
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
// TODO вытащить token и host в секрет
func InitRegistryManager(hostname string) error {
	if kube.IsRunningOutOfKubeCluster() {
		rlog.Infof("Antiopa is running out of cluster. No registry manager required.")
		return nil
	}

	rlog.Debug("Init registry manager")

	PodHostname = hostname
	RegistryErrorCounter = 0

	// TODO Пока для доступа к registry.flant.com передаётся временный токен через переменную среды
	GitlabToken := os.Getenv("GITLAB_TOKEN")
	DockerRegistryInfo["registry.flant.com"]["password"] = GitlabToken

	ImageUpdated = make(chan string)

	rlog.Infof("Registry manager initialized")

	return nil
}

// Запускает проверку каждые 10 секунд, не изменился ли id образа.
func RunRegistryManager() {
	if kube.IsRunningOutOfKubeCluster() {
		return
	}

	rlog.Infof("Registry manager: start watch for image '%s'", AntiopaImageName)

	ticker := time.NewTicker(time.Duration(10) * time.Second)

	CheckIsImageUpdated()
	for {
		select {
		case <-ticker.C:
			CheckIsImageUpdated()
		}
	}
}

// Основной метод проверки обновления образа.
// Метод запускается периодически. Вначале пытается достучаться до kube-api
// и по имени Pod-а получить имя и digest его образа. Когда digest получен, то
// обращается в registry и по имени образа смотрит, изменился ли digest. Если да,
// то отправляет новый digest в канал.
func CheckIsImageUpdated() {
	// Первый шаг - получить имя и id образа из куба.
	// kube-api может быть недоступно, поэтому нужно периодически подключаться к нему.
	if AntiopaImageName == "" {
		rlog.Debugf("Registry manager: retrieve image name and id from kube-api")
		podImageName, podImageId := kube.KubeGetPodImageInfo(PodHostname)
		if podImageName == "" {
			rlog.Debugf("Registry manager: error retrieving image name and id from kube-api. Will try again")
			return
		}

		var err error
		AntiopaImageInfo, err = DockerParseImageName(podImageName)
		if err != nil {
			// Очень маловероятная ситуация, потому что Pod запустился, а имя образа из его спеки не парсится.
			rlog.Errorf("Registry manager: pod image name '%s' is invalid. Will try again. Error was: %v", podImageName, err)
			return
		}

		AntiopaImageName = podImageName
		AntiopaImageDigest = FindImageDigest(podImageId)
	}

	// Второй шаг — после получения id начать мониторить его изменение в registry.
	// registry тоже может быть недоступен
	if DockerRegistry == nil {
		rlog.Debugf("Registry manager: create docker registry client")
		var url, user, password string
		if info, hasInfo := DockerRegistryInfo[AntiopaImageInfo.Registry]; hasInfo {
			url = info["url"]
			user = info["user"]
			password = info["password"]
		}
		// Создать клиента для подключения к docker-registry
		// в единственном экземляре
		DockerRegistry = NewDockerRegistry(url, user, password)
	}

	rlog.Debugf("Registry manager: checking registry for updates")
	digest, err := DockerRegistryGetImageDigest(AntiopaImageInfo, DockerRegistry)
	SetOrCheckAntiopaImageDigest(digest, err)
}

// Сравнить запомненный digest образа с полученным из registry.
// Если отличаются — отправить полученный digest в канал.
// Если digest не был запомнен, то запомнить.
// Если была ошибка при опросе registry, то увеличить счётчик ошибок.
// Когда накопится 3 ошибки подряд, вывести ошибку и сбросить счётчик
func SetOrCheckAntiopaImageDigest(digest string, err error) {
	// Если пришёл не валидный id или была ошибка — увеличить счётчик ошибок.
	// Сообщить в лог, когда накопится 3 ошибки подряд
	if err != nil || !IsValidImageDigest(digest) {
		RegistryErrorCounter++
		if RegistryErrorCounter >= 3 {
			rlog.Errorf("Registry manager: registry request error: %s", err)
			RegistryErrorCounter = 0
		}
		return
	}
	if digest != AntiopaImageDigest {
		ImageUpdated <- digest
	}
	RegistryErrorCounter = 0
}
