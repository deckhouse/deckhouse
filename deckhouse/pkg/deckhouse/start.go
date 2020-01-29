package deckhouse

import (
	"context"
	"os"
	"regexp"
	"time"

	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/metrics_storage"
	log "github.com/sirupsen/logrus"

	"flant/deckhouse/pkg/app"
	"flant/deckhouse/pkg/docker_registry_watcher"
	addon_operator "github.com/flant/addon-operator/pkg/addon-operator"
	sh_app "github.com/flant/shell-operator/pkg/app"
)

type DeckhouseController struct {
	*addon_operator.AddonOperator
	RegistryWatcher docker_registry_watcher.DockerRegistryWatcher

	ctx    context.Context
	cancel context.CancelFunc
}

func NewDeckhouseController() *DeckhouseController {
	return &DeckhouseController{
		AddonOperator: addon_operator.NewAddonOperator(),
	}
}

func (d *DeckhouseController) WithContext(ctx context.Context) *DeckhouseController {
	d.ctx, d.cancel = context.WithCancel(ctx)
	d.AddonOperator.WithContext(d.ctx)
	return d
}

func (d *DeckhouseController) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
}

// StartWatchRegistry initializes and starts a RegistryManager.
func (d *DeckhouseController) InitAndStartRegistryWatcher() error {
	// Initialize RegistryWatcher dependencies

	// Metric storage.
	metricStorage := metrics_storage.NewMetricStorage()
	metricStorage.WithContext(d.ctx)
	metricStorage.WithPrefix(sh_app.PrometheusMetricsPrefix)
	metricStorage.Start()

	// Initialize kube client.
	kubeClient := kube.NewKubernetesClient()
	kubeClient.WithContextName(sh_app.KubeContext)
	kubeClient.WithConfigPath(sh_app.KubeConfig)
	err := kubeClient.Init()
	if err != nil {
		log.Errorf("MAIN Fatal: initialize kube client: %s\n", err)
		return err
	}

	// Initialize RegistryWatcher
	LastSuccessTime := time.Now()
	registryWatcher := docker_registry_watcher.NewDockerRegistryWatcher()
	registryWatcher.WithContext(d.ctx)
	registryWatcher.WithRegistrySecretPath(app.RegistrySecretPath)
	registryWatcher.WithFatalCallback(func() {
		os.Exit(1)
		return
	})
	registryWatcher.WithErrorCallback(func() {
		d.MetricStorage.SendCounterNoPrefix("deckhouse_registry_errors", 1.0, map[string]string{})
		nowTime := time.Now()
		if LastSuccessTime.Add(app.RegistryErrorsMaxTimeBeforeRestart).Before(nowTime) {
			log.Errorf("No success response from registry during %s. Forced restart.", app.RegistryErrorsMaxTimeBeforeRestart.String())
			os.Exit(1)
		}
		return
	})
	registryWatcher.WithSuccessCallback(func() {
		LastSuccessTime = time.Now()
	})
	registryWatcher.WithImageInfoCallback(func() (s string, s2 string) {
		return GetCurrentPodImageInfo(kubeClient)
	})
	registryWatcher.WithImageUpdatedCallback(func(s string) {
		UpdateDeploymentImageAndExit(kubeClient, s)
	})

	err = registryWatcher.Init()
	if err != nil {
		log.Errorf("Initialize registry manager: %s", err)
		return err
	}
	registryWatcher.Start()

	d.RegistryWatcher = registryWatcher

	// RegistryWatcher started, set initialized dependencies for AddonOperator.
	d.AddonOperator.WithKubernetesClient(kubeClient)
	d.AddonOperator.WithMetricStorage(metricStorage)

	return nil
}

// UpdateDeploymentImageAndExit updates "deckhouseImageId" label of deployment/deckhouse
func UpdateDeploymentImageAndExit(kubeClient kube.KubernetesClient, newImageId string) {
	deployment, err := GetDeploymentOfCurrentPod(kubeClient)
	if err != nil {
		log.Errorf("Get deployment of current pod: %s", err)
		return
	}

	deployment.Spec.Template.Labels["deckhouseImageId"] = NormalizeLabelValue(newImageId)

	err = UpdateDeployment(kubeClient, deployment)
	if err != nil {
		log.Errorf("Update Deployment/%s error: %s", deployment.Name, err)
		return
	}

	log.Infof("Update Deployment/%s is successful, exiting ...", deployment.Name)
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
// - "imageID": "docker-pullable://registry.gitlab.com/projectgroup/projectname/dev@sha256:05f5cc14dff4fcc3ff3eb554de0e550050e65c968dc8bbc2d7f4506edfcdc5b6"
// - "imageID": "docker://sha256:e537460dd124f6db6656c1728a42cf8e268923ff52575504a471fa485c2a884a"
//
// Image name should be taken from container spec. ContainerStatus contains bad image name
// if multiple tags has one digest!
// https://github.com/kubernetes/kubernetes/issues/51017
func GetCurrentPodImageInfo(kubeClient kube.KubernetesClient) (imageName string, imageId string) {
	res, err := GetCurrentPod(kubeClient)
	if err != nil {
		log.Debugf("Get current pod info: %v", err)
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

func DefaultDeckhouse() *DeckhouseController {
	ctrl := NewDeckhouseController()
	ctrl.WithContext(context.Background())
	return ctrl
}

func InitAndStart(ctrl *DeckhouseController) error {
	// This callback is executed when KubernetesClient and MetricStorage
	// are ready to use, and RegistryWatcher can started before
	// ModuleManager initialization to be able to update image
	// with hook configuration errors.
	if app.FeatureWatchRegistry == "yes" {
		err := ctrl.InitAndStartRegistryWatcher()
		if err != nil {
			log.Errorf("Fail to start RegistryWatcher: %s", err)
			return err
		}
	} else {
		log.Debugf("Deckhouse: registry manager disabled with DECKHOUSE_WATCH_REGISTRY=%s.", app.FeatureWatchRegistry)
	}

	err := addon_operator.InitAndStart(ctrl.AddonOperator)
	if err != nil {
		return err
	}

	return nil
}
