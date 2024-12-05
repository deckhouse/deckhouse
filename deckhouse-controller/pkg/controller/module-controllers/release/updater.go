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
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/updater"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func newModuleUpdater(
	ctx context.Context,
	dc dependency.Container,
	logger *log.Logger,
	settings *updater.Settings,
	kubeAPI updater.KubeAPI[*v1alpha1.ModuleRelease],
	enabledModules []string,
	metricStorage *metricstorage.MetricStorage,
) *updater.Updater[*v1alpha1.ModuleRelease] {
	return updater.NewUpdater[*v1alpha1.ModuleRelease](
		ctx,
		dc,
		logger, settings,
		updater.DeckhouseReleaseData{},
		true,
		false,
		kubeAPI,
		newMetricsUpdater(metricStorage),
		newWebhookDataSource(logger),
		enabledModules,
	)
}

func newWebhookDataSource(logger *log.Logger) *webhookDataSource {
	return &webhookDataSource{logger: logger}
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
	output.Message = fmt.Sprintf("New module %s release %s is available. Release will be applied at: %s",
		release.Spec.ModuleName, output.Version, applyTime.Format(time.RFC850))
	output.ModuleName = release.GetModuleName()
}

func newKubeAPI(ctx context.Context, logger *log.Logger, client client.Client, downloadedModulesDir string, symlinksDir string,
	moduleManager moduleManager, dc dependency.Container, clusterUUID string,
) *kubeAPI {
	return &kubeAPI{
		ctx:                  ctx,
		logger:               logger,
		client:               client,
		downloadedModulesDir: downloadedModulesDir,
		symlinksDir:          symlinksDir,
		moduleManager:        moduleManager,
		dc:                   dc,
		clusterUUID:          clusterUUID,
	}
}

type kubeAPI struct {
	// TODO: move context from struct field to arguments
	ctx                  context.Context
	logger               *log.Logger
	client               client.Client
	downloadedModulesDir string
	symlinksDir          string
	moduleManager        moduleManager
	dc                   dependency.Container
	clusterUUID          string
}

func (k *kubeAPI) UpdateReleaseStatus(ctx context.Context, release *v1alpha1.ModuleRelease, msg, phase string) error {
	// reset transition time only if phase changes
	if release.Status.Phase != phase {
		release.Status.TransitionTime = metav1.NewTime(k.dc.GetClock().Now().UTC())
	}
	release.Status.Phase = phase
	release.Status.Message = msg

	if err := k.client.Status().Update(ctx, release); err != nil {
		return fmt.Errorf("update release %s status: %w", release.Name, err)
	}

	return nil
}

func (k *kubeAPI) PatchReleaseAnnotations(ctx context.Context, release *v1alpha1.ModuleRelease, annotations map[string]any) error {
	patch, _ := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": annotations,
		},
	})
	p := client.RawPatch(types.MergePatchType, patch)

	return k.client.Patch(ctx, release, p)
}

func (k *kubeAPI) PatchReleaseApplyAfter(release *v1alpha1.ModuleRelease, applyTime time.Time) error {
	return k.PatchReleaseAnnotations(context.TODO(), release, map[string]any{
		"release.deckhouse.io/notification-time-shift": "true",
		"release.deckhouse.io/applyAfter":              applyTime.Format(time.RFC3339),
	})
}

func (k *kubeAPI) DeployRelease(ctx context.Context, release *v1alpha1.ModuleRelease) error {
	moduleName := release.Spec.ModuleName

	// download desired module version
	var ms v1alpha1.ModuleSource
	err := k.client.Get(ctx, types.NamespacedName{Name: release.GetModuleSource()}, &ms)
	if err != nil {
		return fmt.Errorf("get module source: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "module*")
	if err != nil {
		return fmt.Errorf("cannot create tmp directory: %w", err)
	}
	defer func() {
		if err = os.RemoveAll(tmpDir); err != nil {
			k.logger.Errorf("cannot remove old module dir %q: %s", tmpDir, err.Error())
		}
	}()

	options := utils.GenerateRegistryOptionsFromModuleSource(&ms, k.clusterUUID, k.logger)
	md := downloader.NewModuleDownloader(k.dc, tmpDir, &ms, options)
	ds, err := md.DownloadByModuleVersion(release.Spec.ModuleName, release.Spec.Version.String())
	if err != nil {
		return fmt.Errorf("download module: %w", err)
	}

	err = k.updateModuleReleaseDownloadStatistic(context.Background(), release, ds)
	if err != nil {
		return fmt.Errorf("update module release download statistic: %w", err)
	}

	tmpModuleVersionPath := path.Join(tmpDir, moduleName, "v"+release.Spec.Version.String())
	relativeModulePath := generateModulePath(moduleName, release.Spec.Version.String())

	def := moduleloader.Definition{
		Name:   moduleName,
		Weight: release.Spec.Weight,
		Path:   tmpModuleVersionPath,
	}
	values := make(addonutils.Values)
	if module := k.moduleManager.GetModule(moduleName); module != nil {
		values = module.GetConfigValues(false)
	}
	err = validateModule(def, values, k.logger)
	if err != nil {
		release.Status.Phase = v1alpha1.ModuleReleasePhaseSuspended
		if statusErr := k.UpdateReleaseStatus(ctx, release, "validation failed: "+err.Error(), release.Status.Phase); statusErr != nil {
			k.logger.Errorf("update the '%s' release status: %s", release.Name, statusErr.Error())
		}
		return fmt.Errorf("module '%s:v%s' validation failed: %s", moduleName, release.Spec.Version.String(), err)
	}

	moduleVersionPath := path.Join(k.downloadedModulesDir, moduleName, "v"+release.Spec.Version.String())
	if err = os.RemoveAll(moduleVersionPath); err != nil {
		return fmt.Errorf("cannot remove old module dir %q: %w", moduleVersionPath, err)
	}

	if err = cp.Copy(tmpModuleVersionPath, moduleVersionPath); err != nil {
		return fmt.Errorf("copy module dir: %w", err)
	}
	def.Path = moduleVersionPath

	// search symlink for module by regexp
	// module weight for a new version of the module may be different from the old one,
	// we need to find a symlink that contains the module name without looking at the weight prefix.
	currentModuleSymlink, err := findExistingModuleSymlink(k.symlinksDir, moduleName)
	newModuleSymlink := path.Join(k.symlinksDir, fmt.Sprintf("%d-%s", def.Weight, moduleName))
	if err != nil {
		currentModuleSymlink = "900-" + moduleName // fallback
	}
	err = enableModule(k.downloadedModulesDir, currentModuleSymlink, newModuleSymlink, relativeModulePath)
	if err != nil {
		return fmt.Errorf("module deploy failed: %w", err)
	}

	// disable target module hooks so as not to invoke them before restart
	if k.moduleManager.GetModule(moduleName) != nil {
		k.moduleManager.DisableModuleHooks(moduleName)
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

func (k *kubeAPI) updateModuleReleaseDownloadStatistic(ctx context.Context, release *v1alpha1.ModuleRelease,
	ds *downloader.DownloadStatistic,
) error {
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
