package docker_registry_manager

import (
	"fmt"
	"io/ioutil"
	"path"
	"time"

	registryclient "github.com/flant/docker-registry-client/registry"
	"github.com/romana/rlog"

	utils_file "github.com/flant/shell-operator/pkg/utils/file"
)

type DockerRegistryManager interface {
	WithRegistrySecretPath(string)
	WithErrorCallback(errorCb func())
	WithSuccessCallback(errorCb func())
	WithImageInfoCallback(imageInfoCb func() (string, string))
	WithImageUpdatedCallback(imageUpdatedCb func(string))
	Init() error
	Run()
}

var (
	RegistryToUrlMapping map[string]string
)

type MainRegistryManager struct {
	AntiopaImageDigest string
	AntiopaImageName   string
	AntiopaImageInfo   DockerImageInfo
	// клиент для обращений к
	DockerRegistry *registryclient.Registry
	// path to a file with dockercfg
	RegistrySecretPath string
	// счётчик ошибок обращений к registry
	ErrorCounter int
	// callback вызывается в случае ошибки
	ErrorCallback func()
	// calls when get info from registry
	SuccessCallback      func()
	ImageInfoCallback    func() (string, string)
	ImageUpdatedCallback func(string)
}

// InitRegistryManager получает имя образа по имени пода и запрашивает id этого образа.
func NewDockerRegistryManager() DockerRegistryManager {
	return &MainRegistryManager{
		ErrorCounter: 0,
	}
}

func (rm *MainRegistryManager) Init() error {
	rlog.Debug("Init registry manager")

	//RegistrySecretPath = os.Getenv("ANTIOPA_REGISTRY_SECRET_PATH")
	//if RegistrySecretPath == "" {
	//	RegistrySecretPath = DefaultRegistrySecretPath
	//}
	rlog.Infof("Load registry auths from %s dir", rm.RegistrySecretPath)

	// Load json from file /etc/registrysecret/.dockercfg
	if exists, err := utils_file.DirExists(rm.RegistrySecretPath); !exists {
		rlog.Errorf("Error accessing registry secret directory: %s, watcher is disabled now", err)
		return nil
	}

	var readErr error
	var secretBytes []byte
	secretBytes, readErr = ioutil.ReadFile(path.Join(rm.RegistrySecretPath, ".dockercfg"))
	if readErr != nil {
		secretBytes, readErr = ioutil.ReadFile(path.Join(rm.RegistrySecretPath, ".dockerconfigjson"))
		if readErr != nil {
			return fmt.Errorf("Cannot read registry secret from .docker[cfg,configjson]: %s", readErr)
		}
	}

	err := LoadDockerRegistrySecret(secretBytes)
	if err != nil {
		return fmt.Errorf("Cannot load registry secret: %s", err)
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

	rlog.Debugf("Registry manager initialized")
	return nil
}

// Запускает проверку каждые 10 секунд, не изменился ли id образа.
func (rm *MainRegistryManager) Run() {
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

func (rm *MainRegistryManager) WithErrorCallback(errorCb func()) {
	rm.ErrorCallback = errorCb
}

func (rm *MainRegistryManager) WithSuccessCallback(successCb func()) {
	rm.SuccessCallback = successCb
}

func (rm *MainRegistryManager) WithImageInfoCallback(imageInfoCb func() (string, string)) {
	rm.ImageInfoCallback = imageInfoCb
}

func (rm *MainRegistryManager) WithImageUpdatedCallback(imageUpdatedCb func(string)) {
	rm.ImageUpdatedCallback = imageUpdatedCb
}

func (rm *MainRegistryManager) WithRegistrySecretPath(secretPath string) {
	rm.RegistrySecretPath = secretPath
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
		podImageName, podImageId := rm.ImageInfoCallback()
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
			rm.ImageUpdatedCallback("NO_DIGEST_FOUND")
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
	// Request to the registry was successful, call SuccessCallback
	rm.SuccessCallback()
	if digest != rm.AntiopaImageDigest {
		rm.ImageUpdatedCallback(digest)
	}
	rm.ErrorCounter = 0
}
