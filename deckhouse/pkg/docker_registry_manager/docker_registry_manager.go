package docker_registry_manager

import (
	"fmt"
	"io/ioutil"
	"path"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/romana/rlog"

	utils_file "github.com/flant/shell-operator/pkg/utils/file"

	"flant/deckhouse/pkg/app"
)

type DockerRegistryManager interface {
	WithRegistrySecretPath(string)
	WithErrorCallback(errorCb func())
	WithFatalCallback(fatalCb func())
	WithSuccessCallback(successCb func())
	WithImageInfoCallback(imageInfoCb func() (string, string))
	WithImageUpdatedCallback(imageUpdatedCb func(string))
	Init() error
	Run()
}

type MainRegistryManager struct {
	AntiopaImageDigest string
	AntiopaImageName   string
	AntiopaImageInfo   name.Reference
	// path to a file with dockercfg
	RegistrySecretPath string
	// счётчик ошибок обращений к registry
	ErrorCounter int
	// callback вызывается в случае ошибки
	ErrorCallback func()
	// callback for fatal errors — program should exit
	FatalCallback func()
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

// Init loads authes from registry secret
func (rm *MainRegistryManager) Init() error {
	rlog.Infof("Load registry auths from %s dir", rm.RegistrySecretPath)

	if exists, err := utils_file.DirExists(rm.RegistrySecretPath); !exists {
		rlog.Errorf("Error accessing registry secret directory: %s, watcher is disabled now", err)
		return nil
	}

	// Secret type: kubernetes.io/dockerconfigjson
	configJsonExists, err1 := utils_file.FileExists(path.Join(rm.RegistrySecretPath, ".dockerconfigjson"))
	// Secret type: kubernetes.io/dockercfg
	cfgExists, err2 := utils_file.FileExists(path.Join(rm.RegistrySecretPath, ".dockercfg"))

	if !configJsonExists && !cfgExists {
		return fmt.Errorf("No .dockerconfigjson or .dockercfg in a secret directory: watcher is disabled now, %v, %v", err1, err2)
	}

	var readErr error
	var secretBytes []byte
	if configJsonExists {
		secretBytes, readErr = ioutil.ReadFile(path.Join(rm.RegistrySecretPath, ".dockerconfigjson"))
		if readErr != nil {
			return fmt.Errorf("Cannot read registry secret from .dockerconfigjson: %s", readErr)
		}
		if len(secretBytes) == 0 {
			return fmt.Errorf("Registry secret in .dockerconfigjson is empty")
		}
	} else {
		secretBytes, readErr = ioutil.ReadFile(path.Join(rm.RegistrySecretPath, ".dockercfg"))
		if readErr != nil {
			return fmt.Errorf("Cannot read registry secret from .dockercfg: %s", readErr)
		}
		if len(secretBytes) == 0 {
			return fmt.Errorf("Registry secret in .dockercfg is empty")
		}
	}

	err := LoadDockerRegistrySecret(secretBytes)
	if err != nil {
		return fmt.Errorf("Cannot load registry secret: %s", err)
	}

	registries := []string{}
	for k := range DockerCfgAuths {
		registries = append(registries, "'"+k+"'")
	}
	rlog.Infof("Load auths for this registries: %s", strings.Join(registries, ", "))

	return nil
}

// Запускает проверку каждые 10 секунд, не изменился ли id образа.
func (rm *MainRegistryManager) Run() {
	rlog.Infof("Registry manager: start")

	getImageTicker := time.NewTicker(time.Duration(1) * time.Second)
	rm.GetAntiopaImageInfo()
	for {
		if rm.AntiopaImageDigest != "" {
			break
		}
		select {
		case <-getImageTicker.C:
			rm.GetAntiopaImageInfo()
		}
	}
	getImageTicker.Stop()

	checkImageTicker := time.NewTicker(time.Duration(10) * time.Second)
	rm.CheckIsImageUpdated()
	for {
		select {
		case <-checkImageTicker.C:
			rm.CheckIsImageUpdated()
		}
	}
}

func (rm *MainRegistryManager) WithErrorCallback(errorCb func()) {
	rm.ErrorCallback = errorCb
}

func (rm *MainRegistryManager) WithFatalCallback(fatalCb func()) {
	rm.FatalCallback = fatalCb
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

// GetAntiopaImageInfo is a first phase: get image name and imageID from pod's status.
//
// Api-server may be unavailable sometimes, status.imageID is updated with delay, so
// this method is repeated until api-server returns object with non-empty imageID.
func (rm *MainRegistryManager) GetAntiopaImageInfo() {
	rlog.Debugf("Registry manager: retrieve image name and id from kube-api")
	podImageName, podImageId := rm.ImageInfoCallback()
	if podImageName == "" {
		rlog.Infof("Registry manager: cannot get image name for pod. Will request kubernetes api-server again.")
		return
	}
	if podImageId == "" {
		rlog.Infof("Registry manager: image ID for pod is empty. Will request kubernetes api-server again.")
		return
	}

	var err error
	rm.AntiopaImageInfo, err = name.ParseReference(podImageName, ParseReferenceOptions()...)
	if err != nil {
		// This should not really happen: Pod is started, podImageName is non empty but cannot be parsed.
		rlog.Errorf("Possibly a bug: REGISTRY MANAGER got pod image name '%s' that is invalid. Will try again. Error was: %v", podImageName, err)
		return
	}

	rm.AntiopaImageName = podImageName

	rm.AntiopaImageDigest, err = FindImageDigest(podImageId)
	if err != nil {
		rlog.Errorf("RegistryManager: %s", err)
		rm.ImageUpdatedCallback("NO_DIGEST_FOUND")
		return
	}
	// docker 1.11 case
	if rm.AntiopaImageDigest == "" {
		return
	}

	// It is a fatal error if registry in image name has no authConfig
	_, err = NewKeychain().Resolve(rm.AntiopaImageInfo.Context())
	if err != nil && app.InsecureRegistry == "no" {
		rlog.Errorf("No auth found for registry %s. Exiting.", rm.AntiopaImageInfo.Context().RegistryStr())
		rm.FatalCallback()
	}
}

// CheckIsImageUpdated is a second phase: image name and image digest are available, so try to get digest from registry.
// Основной метод проверки обновления образа.
// Метод запускается периодически, когда получены имя и digest образа Pod'd.
// Метод обращается в registry и по имени образа смотрит, изменился ли digest.
// Если digest изменился, то вызывает SuccessCallback
// Если при запросе к registry была ошибки, то вызывается ErrorCallback, а раз в три ошибки записывается в лог.
func (rm *MainRegistryManager) CheckIsImageUpdated() {
	// Catch panic in case of registry request error, print stack to log, increase metric.
	defer func() {
		if r := recover(); r != nil {
			rlog.Debugf("REGISTRY: manifest digest request panic: %s", r)
			rlog.Debugf("%s", debug.Stack())
			rm.ErrorCallback()
		}
	}()

	rlog.Debugf("REGISTRY: checking registry for updates...")
	digest, err := ImageDigest(rm.AntiopaImageInfo)
	rlog.Debugf("REGISTRY: digest=%s saved=%s err=%v", digest, rm.AntiopaImageDigest, err)

	if err != nil || !IsValidImageDigest(digest) {
		rm.ErrorCallback()
		rm.ErrorCounter++
		if rm.ErrorCounter >= 3 {
			msg := ""
			if err != nil {
				msg = err.Error()
			} else {
				msg = fmt.Sprintf("digest '%s' is invalid", digest)
			}
			rlog.Errorf("Registry manager: registry request error: %s", msg)
			rm.ErrorCounter = 0
		}
		return
	}
	// Request to the registry was successful, call SuccessCallback
	rm.ErrorCounter = 0
	rm.SuccessCallback()
	if digest != rm.AntiopaImageDigest {
		rlog.Infof("New image detected in registry: %s@sha256:%s", rm.AntiopaImageName, digest)
		rm.ImageUpdatedCallback(digest)
	}
}
