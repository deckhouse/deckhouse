package docker_registry_watcher

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	log "github.com/sirupsen/logrus"

	utils_file "github.com/flant/shell-operator/pkg/utils/file"

	"flant/deckhouse-controller/pkg/app"
)

var logEntry = log.WithField("operator.component", "RegistryWatcher")

type DockerRegistryWatcher interface {
	WithContext(ctx context.Context)
	WithRegistrySecretPath(string)
	WithErrorCallback(errorCb func())
	WithFatalCallback(fatalCb func())
	WithSuccessCallback(successCb func())
	WithImageInfoCallback(imageInfoCb func() (string, string))
	WithImageUpdatedCallback(imageUpdatedCb func(string))
	Init() error
	Start()
	Stop()
}

type dockerRegistryWatcher struct {
	ctx    context.Context
	cancel context.CancelFunc

	ImageDigest string
	ImageName   string
	ImageInfo   name.Reference
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
func NewDockerRegistryWatcher() DockerRegistryWatcher {
	return &dockerRegistryWatcher{
		ErrorCounter: 0,
	}
}

func (w *dockerRegistryWatcher) WithContext(ctx context.Context) {
	w.ctx, w.cancel = context.WithCancel(ctx)
}

func (w *dockerRegistryWatcher) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
}

// Init loads authes from registry secret
func (w *dockerRegistryWatcher) Init() error {
	logEntry.Infof("Load registry auths from %s dir", w.RegistrySecretPath)

	if exists, err := utils_file.DirExists(w.RegistrySecretPath); !exists {
		logEntry.Errorf("Error accessing registry secret directory: %s, watcher is disabled now", err)
		return nil
	}

	// Secret type: kubernetes.io/dockerconfigjson
	configJsonExists, err1 := utils_file.FileExists(path.Join(w.RegistrySecretPath, ".dockerconfigjson"))
	// Secret type: kubernetes.io/dockercfg
	cfgExists, err2 := utils_file.FileExists(path.Join(w.RegistrySecretPath, ".dockercfg"))

	if !configJsonExists && !cfgExists {
		return fmt.Errorf("No .dockerconfigjson or .dockercfg in a secret directory: watcher is disabled now, %v, %v", err1, err2)
	}

	var readErr error
	var secretBytes []byte
	if configJsonExists {
		secretBytes, readErr = ioutil.ReadFile(path.Join(w.RegistrySecretPath, ".dockerconfigjson"))
		if readErr != nil {
			return fmt.Errorf("Cannot read registry secret from .dockerconfigjson: %s", readErr)
		}
		if len(secretBytes) == 0 {
			return fmt.Errorf("Registry secret in .dockerconfigjson is empty")
		}
	} else {
		secretBytes, readErr = ioutil.ReadFile(path.Join(w.RegistrySecretPath, ".dockercfg"))
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
	logEntry.Infof("Load auths for this registries: %s", strings.Join(registries, ", "))

	return nil
}

// Запускает проверку каждые 10 секунд, не изменился ли id образа.
func (w *dockerRegistryWatcher) Start() {
	go func() {
		logEntry.Infof("Registry manager: start")

		getImageTicker := time.NewTicker(time.Duration(1) * time.Second)
		w.GetImageInfo()
		for {
			if w.ImageDigest != "" {
				break
			}
			select {
			case <-getImageTicker.C:
				w.GetImageInfo()
			case <-w.ctx.Done():
				return
			}
		}
		getImageTicker.Stop()

		checkImageTicker := time.NewTicker(time.Duration(10) * time.Second)
		w.CheckIsImageUpdated()
		for {
			select {
			case <-checkImageTicker.C:
				w.CheckIsImageUpdated()
			case <-w.ctx.Done():
				return
			}
		}
	}()
}

func (w *dockerRegistryWatcher) WithErrorCallback(errorCb func()) {
	w.ErrorCallback = errorCb
}

func (w *dockerRegistryWatcher) WithFatalCallback(fatalCb func()) {
	w.FatalCallback = fatalCb
}

func (w *dockerRegistryWatcher) WithSuccessCallback(successCb func()) {
	w.SuccessCallback = successCb
}

func (w *dockerRegistryWatcher) WithImageInfoCallback(imageInfoCb func() (string, string)) {
	w.ImageInfoCallback = imageInfoCb
}

func (w *dockerRegistryWatcher) WithImageUpdatedCallback(imageUpdatedCb func(string)) {
	w.ImageUpdatedCallback = imageUpdatedCb
}

func (w *dockerRegistryWatcher) WithRegistrySecretPath(secretPath string) {
	w.RegistrySecretPath = secretPath
}

// GetImageInfo is a first phase: get image name and imageID from pod's status.
//
// Api-server may be unavailable sometimes and status.imageID is updated with delay, so
// this method should be called until api-server returns object with non-empty imageID.
func (w *dockerRegistryWatcher) GetImageInfo() {
	logEntry.Debugf("Retrieve image name and id from kube-api")
	podImageName, podImageId := w.ImageInfoCallback()
	if podImageName == "" {
		logEntry.Warnf("Cannot get image name from pod status. Will request kubernetes api-server again.")
		return
	}
	if podImageId == "" {
		logEntry.Infof("Image ID for pod is empty. Will request kubernetes api-server again.")
		return
	}

	var err error
	w.ImageInfo, err = name.ParseReference(podImageName, ParseReferenceOptions()...)
	if err != nil {
		// This should not really happen: Pod is started, podImageName is non empty but cannot be parsed.
		logEntry.Errorf("Possibly a bug: REGISTRY MANAGER got pod image name '%s' that is invalid. Will try again. Error was: %v", podImageName, err)
		return
	}

	w.ImageName = podImageName

	w.ImageDigest, err = FindImageDigest(podImageId)
	if err != nil {
		logEntry.Errorf("Find image digest: %s", err)
		w.ImageUpdatedCallback("NO_DIGEST_FOUND")
		return
	}
	// docker 1.11 case
	if w.ImageDigest == "" {
		return
	}

	// It is a fatal error if registry in image name has no authConfig
	_, err = NewKeychain().Resolve(w.ImageInfo.Context())
	if err != nil && app.InsecureRegistry == "no" {
		logEntry.Errorf("No auth found for registry %s. Exiting.", w.ImageInfo.Context().RegistryStr())
		w.FatalCallback()
	}
}

// CheckIsImageUpdated is a second phase: image name and image digest
// are available after GetImageInfo, so try to get digest from registry.
// If registry check is successful, run SuccessCallback.
// If digest is changed, run ImageUpdatedCallback.
// If registry request is failed, run ErrorCallback and after 3 errors
// write message to log.
func (w *dockerRegistryWatcher) CheckIsImageUpdated() {
	// Catch panic in case of registry request error, print stack to log, increase metric.
	defer func() {
		if r := recover(); r != nil {
			logEntry.Debugf("Manifest digest request panic: %s", r)
			logEntry.Debugf("%s", debug.Stack())
			w.ErrorCallback()
		}
	}()

	logEntry.Debugf("Checking registry for updates...")
	digest, err := ImageDigest(w.ImageInfo)
	logEntry.Debugf("Registry response: remote_digest=%s saved_digest=%s err=%v", digest, w.ImageDigest, err)

	if err != nil || !IsValidImageDigest(digest) {
		w.ErrorCallback()
		w.ErrorCounter++
		if w.ErrorCounter >= 3 {
			msg := ""
			if err != nil {
				msg = err.Error()
			} else {
				msg = fmt.Sprintf("digest '%s' is invalid", digest)
			}
			logEntry.Errorf("Registry request error: %s", msg)
			w.ErrorCounter = 0
		}
		return
	}
	// Request to the registry was successful, call SuccessCallback
	w.ErrorCounter = 0
	w.SuccessCallback()
	if digest != w.ImageDigest {
		logEntry.Infof("New image detected in registry: %s@sha256:%s", w.ImageName, digest)
		w.ImageUpdatedCallback(digest)
	}
}
