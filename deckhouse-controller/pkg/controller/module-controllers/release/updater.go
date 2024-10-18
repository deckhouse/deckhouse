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
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strconv"
	"time"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/utils/logger"
	"github.com/flant/shell-operator/pkg/metric_storage"
	cp "github.com/otiai10/copy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/models"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/updater"
)

func newModuleUpdater(dc dependency.Container, logger logger.Logger, settings *updater.Settings,
	kubeAPI updater.KubeAPI[*v1alpha1.ModuleRelease], enabledModules []string, metricStorage *metric_storage.MetricStorage,
) *updater.Updater[*v1alpha1.ModuleRelease] {
	return updater.NewUpdater[*v1alpha1.ModuleRelease](dc, logger, settings,
		updater.DeckhouseReleaseData{}, true, false, kubeAPI, newMetricsUpdater(metricStorage, enabledModules),
		newWebhookDataSource(logger), enabledModules)
}

func newWebhookDataSource(logger logger.Logger) *webhookDataSource {
	return &webhookDataSource{logger: logger}
}

type webhookDataSource struct {
	logger logger.Logger
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

func newKubeAPI(ctx context.Context, logger logger.Logger, client client.Client, downloadedModulesDir string, symlinksDir string,
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
	logger               logger.Logger
	client               client.Client
	downloadedModulesDir string
	symlinksDir          string
	moduleManager        moduleManager
	dc                   dependency.Container
	clusterUUID          string
}

func (k *kubeAPI) UpdateReleaseStatus(release *v1alpha1.ModuleRelease, msg, phase string) error {
	release.Status.Phase = phase
	release.Status.Message = msg
	release.Status.TransitionTime = metav1.NewTime(k.dc.GetClock().Now().UTC())

	err := k.client.Status().Update(k.ctx, release)
	if err != nil {
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

	options := utils.GenerateRegistryOptionsFromModuleSource(&ms, k.clusterUUID)
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

	def := models.DeckhouseModuleDefinition{
		Name:   moduleName,
		Weight: release.Spec.Weight,
		Path:   tmpModuleVersionPath,
	}
	values := make(addonutils.Values)
	if module := k.moduleManager.GetModule(moduleName); module != nil {
		values = module.GetConfigValues(false)
	}
	err = validateModule(def, values)
	if err != nil {
		release.Status.Phase = v1alpha1.PhaseSuspended
		_ = k.UpdateReleaseStatus(release, "validation failed: "+err.Error(), release.Status.Phase)
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
		k.logger.Errorf("Module deploy failed: %v", err)
		if e := k.suspendModuleVersionForRelease(release, err); e != nil {
			return e
		}
	}

	// disable target module hooks so as not to invoke them before restart
	if k.moduleManager.GetModule(moduleName) != nil {
		k.moduleManager.DisableModuleHooks(moduleName)
	}

	return nil
}

func (k *kubeAPI) suspendModuleVersionForRelease(release *v1alpha1.ModuleRelease, err error) error {
	if os.IsNotExist(err) {
		err = errors.New("not found")
	}

	message := fmt.Sprintf("Desired version of the module met problems: %s", err)
	return k.UpdateReleaseStatus(release, updater.PhaseSuspended, message)
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

type metricsUpdater struct {
	metricStorage  *metric_storage.MetricStorage
	enabledModules []string
}

func newMetricsUpdater(metricStorage *metric_storage.MetricStorage, enabledModules []string) *metricsUpdater {
	return &metricsUpdater{
		enabledModules: enabledModules,
		metricStorage:  metricStorage,
	}
}

func (m *metricsUpdater) ReleaseBlocked(_, _ string) {
}

func (m *metricsUpdater) WaitingManual(release *v1alpha1.ModuleRelease, totalPendingManualReleases float64) {
	if !slices.Contains(m.enabledModules, release.GetModuleName()) {
		return
	}

	m.metricStorage.GaugeSet(
		"d8_module_release_waiting_manual",
		totalPendingManualReleases,
		map[string]string{
			"name":       release.GetName(),
			"kind":       "module",
			"moduleName": release.GetModuleName(),
			"version":    "v" + release.Spec.Version.String(),
		})
}
