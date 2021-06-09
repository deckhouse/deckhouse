package deckhouse

import (
	"context"
	"os"
	"regexp"
	"time"

	addon_operator "github.com/flant/addon-operator/pkg/addon-operator"
	klient "github.com/flant/kube-client/client"
	sh_app "github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/metric_storage"
	log "github.com/sirupsen/logrus"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/docker_registry_watcher"
)

// Ignore error: type name will be used as deckhouse.DeckhouseController by other packages, and that stutters
//nolint:golint
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

func (d *DeckhouseController) Shutdown() {
	d.RegistryWatcher.Stop()
	d.AddonOperator.Shutdown()
}

// StartWatchRegistry initializes and starts a RegistryManager.
func (d *DeckhouseController) InitAndStartRegistryWatcher() error {
	// Initialize RegistryWatcher dependencies

	// Create and start a metric storage for the RegistryWatcher and the AddonOperator.
	metricStorage := metric_storage.NewMetricStorage()
	metricStorage.WithContext(d.ctx)
	metricStorage.WithPrefix(sh_app.PrometheusMetricsPrefix)
	metricStorage.Start()
	addon_operator.RegisterAddonOperatorMetrics(metricStorage)
	RegisterDeckhouseMetrics(metricStorage)
	// Set MetricStorage in addon-operator to use in a Kubernetes client initialization.
	d.AddonOperator.WithMetricStorage(metricStorage)

	// Create and initialize a Kubernetes client for the RegistryWatcher and the AddonOperator
	// Register metrics for client-go with custom labels.
	//nolint:staticcheck
	klient.RegisterKubernetesClientMetrics(metricStorage, d.AddonOperator.GetMainKubeClientMetricLabels())
	// Initialize a Kubernetes client with settings, metricStorage and custom metric labels.
	kubeClient, err := d.AddonOperator.InitMainKubeClient()
	if err != nil {
		log.Errorf("MAIN Fatal: initialize kube client: %s\n", err)
		return err
	}
	// Set KubeClient in AddonOperator.
	d.AddonOperator.WithKubernetesClient(kubeClient)

	// Initialize and start the RegistryWatcher.
	LastSuccessTime := time.Now()
	registryWatcher := docker_registry_watcher.NewDockerRegistryWatcher()
	registryWatcher.WithContext(d.ctx)
	registryWatcher.WithRegistrySecretPath(app.RegistrySecretPath)
	registryWatcher.WithFatalCallback(func() {
		os.Exit(1)
	})
	registryWatcher.WithErrorCallback(func() {
		d.MetricStorage.CounterAdd("deckhouse_registry_check_total", 1.0, map[string]string{})

		d.MetricStorage.CounterAdd("deckhouse_registry_check_errors_total", 1.0, map[string]string{})
		// throttle logging
		nowTime := time.Now()
		if LastSuccessTime.Add(app.RegistryErrorsMaxTimeBeforeRestart).Before(nowTime) {
			log.Errorf("No success response from registry during %s. Forced restart.", app.RegistryErrorsMaxTimeBeforeRestart.String())
			os.Exit(1)
		}
	})
	registryWatcher.WithSuccessCallback(func() {
		d.MetricStorage.CounterAdd("deckhouse_registry_check_total", 1.0, map[string]string{})
		LastSuccessTime = time.Now()
	})
	registryWatcher.WithImageInfoCallback(func() (imageName string, imageId string) {
		imageName, imageId = GetCurrentPodImageInfo(kubeClient)

		d.MetricStorage.CounterAdd("deckhouse_kube_image_digest_check_total", 1.0, map[string]string{})
		if imageName != "" && imageId != "" {
			d.MetricStorage.GaugeSet("deckhouse_kube_image_digest_check_success", 1.0, map[string]string{})
		}

		return
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

	return nil
}

// UpdateDeploymentImageAndExit updates "deckhouseImageId" label of deployment/deckhouse
func UpdateDeploymentImageAndExit(kubeClient klient.Client, newImageID string) {
	deployment, err := GetDeploymentOfCurrentPod(kubeClient)
	if err != nil {
		log.Errorf("Get deployment of current pod: %s", err)
		return
	}

	deployment.Spec.Template.Labels["deckhouseImageId"] = NormalizeLabelValue(newImageID)

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
func GetCurrentPodImageInfo(kubeClient klient.Client) (imageName string, imageID string) {
	res, err := GetCurrentPod(kubeClient)
	if err != nil {
		log.Debugf("Get current pod info: %v", err)
		return "", ""
	}

	for _, spec := range res.Spec.Containers {
		if spec.Name == app.ContainerName {
			imageName = spec.Image
			break
		}
	}

	for _, status := range res.Status.ContainerStatuses {
		if status.Name == app.ContainerName {
			imageID = status.ImageID
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
