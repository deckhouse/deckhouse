// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package release

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	metricstorage "github.com/flant/shell-operator/pkg/metric_storage"
	cp "github.com/otiai10/copy"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/updater"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const moduleReleaseBlockedMetricName = "d8_module_release_info"

type metricsUpdater struct {
	metricStorage *metricstorage.MetricStorage
}

func (m *metricsUpdater) UpdateReleaseMetric(name string, metricLabels updater.MetricLabels) {
	m.PurgeReleaseMetric(name)
	m.metricStorage.Grouped().GaugeSet(name, moduleReleaseBlockedMetricName, 1, metricLabels)
}

func (m *metricsUpdater) PurgeReleaseMetric(name string) {
	m.metricStorage.Grouped().ExpireGroupMetricByName(name, moduleReleaseBlockedMetricName)
}

type webhookDataSource struct {
	logger *log.Logger
}

func (s *webhookDataSource) Fill(output *updater.WebhookData, release *v1alpha1.ModuleRelease, applyTime time.Time) {
	if output == nil {
		s.logger.Error("webhook data must be defined")
		return
	}

	if release == nil {
		s.logger.Error("release must be defined")
		return
	}

	output.Subject = updater.SubjectModule
	output.Message = fmt.Sprintf("New module %s release %s is available. Release will be applied at: %s", release.Spec.ModuleName, output.Version, applyTime.Format(time.RFC850))
	output.ModuleName = release.GetModuleName()
}

func newKubeAPI(logger *log.Logger, client client.Client, downloadedModulesDir, symlinksDir, clusterUUID string, mm moduleManager, dc dependency.Container) *kubeAPI {
	return &kubeAPI{
		client:               client,
		log:                  logger,
		moduleManager:        mm,
		downloadedModulesDir: downloadedModulesDir,
		symlinksDir:          symlinksDir,
		clusterUUID:          clusterUUID,
		dc:                   dc,
	}
}

type kubeAPI struct {
	client               client.Client
	log                  *log.Logger
	moduleManager        moduleManager
	downloadedModulesDir string
	symlinksDir          string
	clusterUUID          string
	dc                   dependency.Container
}

func (k *kubeAPI) UpdateReleaseStatus(ctx context.Context, release *v1alpha1.ModuleRelease, msg, phase string) error {
	// reset transition time only if phase changes
	if release.Status.Phase != phase {
		release.Status.TransitionTime = metav1.NewTime(k.dc.GetClock().Now().UTC())
	}
	release.Status.Phase = phase
	release.Status.Message = msg

	if err := k.client.Status().Update(ctx, release); err != nil {
		return fmt.Errorf("update the '%s' release status: %w", release.Name, err)
	}

	return nil
}

func (k *kubeAPI) PatchReleaseAnnotations(ctx context.Context, release *v1alpha1.ModuleRelease, annotations map[string]any) error {
	marshalledPatch, _ := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": annotations,
		},
	})
	patch := client.RawPatch(types.MergePatchType, marshalledPatch)

	return k.client.Patch(ctx, release, patch)
}

func (k *kubeAPI) PatchReleaseApplyAfter(release *v1alpha1.ModuleRelease, applyTime time.Time) error {
	return k.PatchReleaseAnnotations(context.TODO(), release, map[string]any{
		"release.deckhouse.io/notification-time-shift": "true",
		"release.deckhouse.io/applyAfter":              applyTime.Format(time.RFC3339),
	})
}

func (k *kubeAPI) DeployRelease(ctx context.Context, release *v1alpha1.ModuleRelease) error {
	// download desired module version
	source := new(v1alpha1.ModuleSource)
	if err := k.client.Get(ctx, client.ObjectKey{Name: release.GetModuleSource()}, source); err != nil {
		return fmt.Errorf("get the '%s' module source: %w", release.GetModuleSource(), err)
	}

	tmpDir, err := os.MkdirTemp("", "module*")
	if err != nil {
		return fmt.Errorf("create tmp directory: %w", err)
	}

	// clear tmp dir
	defer func() {
		if err = os.RemoveAll(tmpDir); err != nil {
			k.log.Errorf("failed to remove the '%s' old module dir: %v", tmpDir, err)
		}
	}()

	options := utils.GenerateRegistryOptionsFromModuleSource(source, k.clusterUUID, k.log)
	md := downloader.NewModuleDownloader(k.dc, tmpDir, source, options)

	downloadStatistic, err := md.DownloadByModuleVersion(release.Spec.ModuleName, release.Spec.Version.String())
	if err != nil {
		return fmt.Errorf("download the '%s/%s' module: %w", release.Spec.ModuleName, release.Spec.Version.String(), err)
	}

	if err = k.updateModuleReleaseDownloadStatistic(context.Background(), release, downloadStatistic); err != nil {
		return fmt.Errorf("updatethe '%s' module release download statistic: %w", release.Name, err)
	}

	def := &moduletypes.Definition{
		Name:   release.Spec.ModuleName,
		Weight: release.Spec.Weight,
		Path:   path.Join(tmpDir, release.Spec.ModuleName, "v"+release.Spec.Version.String()),
	}

	values := make(addonutils.Values)
	if module := k.moduleManager.GetModule(release.Spec.ModuleName); module != nil {
		values = module.GetConfigValues(false)
	}

	if err = def.Validate(values, k.log); err != nil {
		release.Status.Phase = v1alpha1.ModuleReleasePhaseSuspended
		if statusErr := k.UpdateReleaseStatus(ctx, release, "validation failed: "+err.Error(), release.Status.Phase); statusErr != nil {
			k.log.Errorf("update the '%s' release status: %v", release.Name, statusErr)
		}
		return fmt.Errorf("the '%s:v%s' module validation: %w", release.Spec.ModuleName, release.Spec.Version.String(), err)
	}

	moduleVersionPath := path.Join(k.downloadedModulesDir, release.Spec.ModuleName, "v"+release.Spec.Version.String())
	if err = os.RemoveAll(moduleVersionPath); err != nil {
		return fmt.Errorf("remove the '%s' old module dir: %w", moduleVersionPath, err)
	}

	if err = cp.Copy(def.Path, moduleVersionPath); err != nil {
		return fmt.Errorf("copy module dir: %w", err)
	}

	// search symlink for module by regexp
	// module weight for a new version of the module may be different from the old one,
	// we need to find a symlink that contains the module name without looking at the weight prefix.
	currentModuleSymlink, err := utils.GetModuleSymlink(k.symlinksDir, release.Spec.ModuleName)
	if err != nil {
		k.log.Warnf("failed to find the current module symlink for the '%s' module: %v", release.Spec.ModuleName, err)
		currentModuleSymlink = "900-" + release.Spec.ModuleName // fallback
	}

	newModuleSymlink := path.Join(k.symlinksDir, fmt.Sprintf("%d-%s", def.Weight, release.Spec.ModuleName))

	relativeModulePath := path.Join("../", release.Spec.ModuleName, "v"+release.Spec.Version.String())

	if err = utils.EnableModule(k.downloadedModulesDir, currentModuleSymlink, newModuleSymlink, relativeModulePath); err != nil {
		return fmt.Errorf("enable the '%s' module: %w", release.Spec.ModuleName, err)
	}

	// disable target module hooks so as not to invoke them before restart
	if k.moduleManager.GetModule(release.Spec.ModuleName) != nil {
		k.moduleManager.DisableModuleHooks(release.Spec.ModuleName)
	}

	return nil
}

func (k *kubeAPI) SaveReleaseData(ctx context.Context, release *v1alpha1.ModuleRelease, data updater.DeckhouseReleaseData) error {
	if release == nil {
		return fmt.Errorf("empty release")
	}

	return k.PatchReleaseAnnotations(ctx, release, map[string]interface{}{
		// "release.deckhouse.io/isUpdating": strconv.FormatBool(data.IsUpdating), // I don't think we need this flag for ModuleReleases
		"release.deckhouse.io/notified": strconv.FormatBool(data.Notified),
	})
}

func (k *kubeAPI) updateModuleReleaseDownloadStatistic(ctx context.Context, release *v1alpha1.ModuleRelease, ds *downloader.DownloadStatistic) error {
	release.Status.Size = ds.Size
	release.Status.PullDuration = metav1.Duration{Duration: ds.PullDuration}

	return k.client.Status().Update(ctx, release)
}

func (k *kubeAPI) IsKubernetesVersionAutomatic(ctx context.Context) (bool, error) {
	key := client.ObjectKey{Namespace: "kube-system", Name: "d8-cluster-configuration"}
	secret := new(corev1.Secret)
	if err := k.client.Get(ctx, key, secret); err != nil {
		return false, fmt.Errorf("check kubernetes version: failed to get secret: %w", err)
	}

	var clusterConf struct {
		KubernetesVersion string `json:"kubernetesVersion"`
	}
	clusterConfigurationRaw, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return false, fmt.Errorf("check kubernetes version: expected field 'cluster-configuration.yaml' not found in secret %s", secret.Name)
	}
	if err := yaml.Unmarshal(clusterConfigurationRaw, &clusterConf); err != nil {
		return false, fmt.Errorf("check kubernetes version: failed to unmarshal cluster configuration: %w", err)
	}
	return clusterConf.KubernetesVersion == "Automatic", nil
}
