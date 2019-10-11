package deckhouse

import (
	"os"
	"regexp"
	"time"

	"github.com/romana/rlog"

	addon_operator "github.com/flant/addon-operator/pkg/addon-operator"

	"flant/deckhouse/pkg/app"
	"flant/deckhouse/pkg/docker_registry_manager"
)

// Start runs registry watcher and start addon_operator
func Start() {
	rlog.Debug("DECKHOUSE: Start")

	// BeforeHelmInitCb is called when kube client is initialized and metrics storage is started
	addon_operator.BeforeHelmInitCb = func() {
		if app.FeatureWatchRegistry == "yes" {
			err := StartWatchRegistry()
			if err != nil {
				rlog.Errorf("Cannot start watch registry: %s", err)
				os.Exit(1)
			}
		} else {
			rlog.Debugf("Deckhouse: registry manager disabled with DECKHOUSE_WATCH_REGISTRY=%s.", app.FeatureWatchRegistry)
		}
	}

	addon_operator.Start()
}

// StartWatchRegistry initializes and starts a RegistryManager.
func StartWatchRegistry() error {
	LastSuccessTime := time.Now()
	RegistryManager := docker_registry_manager.NewDockerRegistryManager()
	RegistryManager.WithRegistrySecretPath(app.RegistrySecretPath)
	RegistryManager.WithFatalCallback(func() {
		os.Exit(1)
		return
	})
	RegistryManager.WithErrorCallback(func() {
		addon_operator.MetricsStorage.SendCounterMetric("deckhouse_registry_errors", 1.0, map[string]string{})
		nowTime := time.Now()
		if LastSuccessTime.Add(app.RegistryErrorsMaxTimeBeforeRestart).Before(nowTime) {
			rlog.Errorf("No success response from registry during %s. Forced restart.", app.RegistryErrorsMaxTimeBeforeRestart.String())
			os.Exit(1)
		}
		return
	})
	RegistryManager.WithSuccessCallback(func() {
		LastSuccessTime = time.Now()
	})
	RegistryManager.WithImageInfoCallback(GetCurrentPodImageInfo)
	RegistryManager.WithImageUpdatedCallback(UpdateDeploymentImage)

	err := RegistryManager.Init()
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot initialize registry manager: %s", err)
		return err
	}
	go RegistryManager.Run()

	return nil
}

// UpdateDeploymentImage updates "deckhouseImageId" label of deployment/deckhouse
func UpdateDeploymentImage(newImageId string) {
	deployment, err := GetDeploymentOfCurrentPod()
	if err != nil {
		rlog.Errorf("KUBE get current deployment: %s", err)
		return
	}

	deployment.Spec.Template.Labels["deckhouseImageId"] = NormalizeLabelValue(newImageId)

	err = UpdateDeployment(deployment)
	if err != nil {
		rlog.Errorf("KUBE deployment update error: %s", err)
		return
	}

	rlog.Infof("KUBE deployment update successful, exiting ...")
	os.Exit(1)
}

var NonSafeCharsRegexp = regexp.MustCompile(`[^a-zA-Z0-9]`)

func NormalizeLabelValue(value string) string {
	newVal := NonSafeCharsRegexp.ReplaceAllLiteralString(value, "_")
	labelLen := len(newVal)
	if labelLen > 63 {
		labelLen = 63
	}
	return newVal[:labelLen]
}

// GetCurrentPodImageInfo returns image name (registry:port/image_repo:image_tag) and imageID.
//
// imageID can be in two forms on docker backend:
// - "imageID": "docker-pullable://registry.flant.com/sys/antiopa/dev@sha256:05f5cc14dff4fcc3ff3eb554de0e550050e65c968dc8bbc2d7f4506edfcdc5b6"
// - "imageID": "docker://sha256:e537460dd124f6db6656c1728a42cf8e268923ff52575504a471fa485c2a884a"
//
// Image name should be taken from container spec. ContainerStatus contains bad image name
// if multiple tags has one digest!
// https://github.com/kubernetes/kubernetes/issues/51017
func GetCurrentPodImageInfo() (imageName string, imageId string) {
	res, err := GetCurrentPod()
	if err != nil {
		rlog.Debugf("KUBE Get current pod info: %v", err)
		return "", ""
	}

	// TODO DELETE THIS AFTER MIGRATION
	// Temporary fix: container can be named "antiopa"
	for _, spec := range res.Spec.Containers {
		if spec.Name == "antiopa" {
			app.ContainerName = "antiopa"
			break
		}
	}
	// END DELETE THIS

	for _, spec := range res.Spec.Containers {
		if spec.Name == app.ContainerName {
			imageName = spec.Image
			break
		}
	}

	for _, status := range res.Status.ContainerStatuses {
		if status.Name == app.ContainerName {
			imageId = status.ImageID
			break
		}
	}

	return
}
