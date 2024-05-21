// Copyright 2023 Flant JSC
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
	"fmt"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"github.com/flant/addon-operator/pkg/utils/logger"
	log "github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	deckhouseconfig "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// modulePullOverrideReconciler is the controller implementation for ModulePullOverride resources
type modulePullOverrideReconciler struct {
	client client.Client
	dc     dependency.Container

	logger logger.Logger

	moduleManager      moduleManager
	externalModulesDir string
	symlinksDir        string
}

// NewModulePullOverrideController returns a new sample controller
func NewModulePullOverrideController(
	mgr manager.Manager,
	dc dependency.Container,
	moduleManager moduleManager,
) error {
	lg := log.WithField("component", "ModulePullOverrideController")

	rc := &modulePullOverrideReconciler{
		client: mgr.GetClient(),
		dc:     dc,
		logger: lg,

		moduleManager:      moduleManager,
		externalModulesDir: os.Getenv("EXTERNAL_MODULES_DIR"),
		symlinksDir:        filepath.Join(os.Getenv("EXTERNAL_MODULES_DIR"), "modules"),
	}

	// Add Preflight Check
	err := mgr.Add(manager.RunnableFunc(rc.PreflightCheck))
	if err != nil {
		return err
	}

	ctr, err := controller.New("module-pull-override", mgr, controller.Options{
		MaxConcurrentReconciles: 1,
		CacheSyncTimeout:        30 * time.Minute,
		NeedLeaderElection:      pointer.Bool(false),
		Reconciler:              rc,
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ModulePullOverride{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		Complete(ctr)
}

func (c *modulePullOverrideReconciler) PreflightCheck(ctx context.Context) error {
	// Check if controller's dependencies have been initialized
	_ = wait.PollUntilContextCancel(ctx, utils.SyncedPollPeriod, false,
		func(context.Context) (bool, error) {
			// TODO: add modulemanager initialization check c.moduleManager.AreModulesInited() (required for reloading modules without restarting deckhouse)
			return deckhouseconfig.IsServiceInited(), nil
		})

	err := c.restoreAbsentModulesFromOverrides(ctx)
	if err != nil {
		return fmt.Errorf("modules restoration from overrides failed: %w", err)
	}

	return nil
}

func (c *modulePullOverrideReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	mpo := new(v1alpha1.ModulePullOverride)
	err := c.client.Get(ctx, types.NamespacedName{Name: request.Name}, mpo)
	if err != nil {
		// The ModulePullOverride resource may no longer exist, in which case we stop
		// processing.
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	return c.moduleOverrideReconcile(ctx, mpo)
}

func (c *modulePullOverrideReconciler) moduleOverrideReconcile(ctx context.Context, mo *v1alpha1.ModulePullOverride) (ctrl.Result, error) {
	var metaUpdateRequired bool

	// check if RegistrySpecChangedAnnotation annotation is set and process it
	if _, set := mo.GetAnnotations()[RegistrySpecChangedAnnotation]; set {
		// if module is enabled - push runModule task in the main queue
		c.logger.Infof("Applying new registry settings to the %s module", mo.Name)
		err := c.moduleManager.RunModuleWithNewStaticValues(mo.Name, mo.ObjectMeta.Labels["source"], filepath.Join(c.externalModulesDir, mo.Name, downloader.DefaultDevVersion))
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		// delete annotation and requeue
		delete(mo.ObjectMeta.Annotations, RegistrySpecChangedAnnotation)
		metaUpdateRequired = true
	}

	// add labels if empty
	// source and release controllers are looking for this labels
	if _, ok := mo.Labels["module"]; !ok {
		if len(mo.Labels) > 0 {
			mo.Labels["module"] = mo.Name
			mo.Labels["source"] = mo.Spec.Source
		} else {
			mo.SetLabels(map[string]string{"module": mo.Name, "source": mo.Spec.Source})
		}
		metaUpdateRequired = true
	}

	if metaUpdateRequired {
		return ctrl.Result{RequeueAfter: 500 * time.Millisecond}, c.client.Update(ctx, mo)
	}

	ms := new(v1alpha1.ModuleSource)
	err := c.client.Get(ctx, types.NamespacedName{Name: mo.Spec.Source}, ms)
	if err != nil {
		if apierrors.IsNotFound(err) {
			mo.Status.Message = fmt.Sprintf("ModuleSource %q not found", mo.Spec.Source)
			if e := c.updateModulePullOverrideStatus(ctx, mo); e != nil {
				return ctrl.Result{Requeue: true}, e
			}
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	md := downloader.NewModuleDownloader(c.dc, c.externalModulesDir, ms, utils.GenerateRegistryOptions(ms))
	newChecksum, moduleDef, err := md.DownloadDevImageTag(mo.Name, mo.Spec.ImageTag, mo.Status.ImageDigest)
	if err != nil {
		mo.Status.Message = err.Error()
		if e := c.updateModulePullOverrideStatus(ctx, mo); e != nil {
			return ctrl.Result{Requeue: true}, e
		}
		return ctrl.Result{RequeueAfter: mo.Spec.ScanInterval.Duration}, nil
	}

	if newChecksum == "" {
		// module is up-to-date
		if mo.Status.Message != "" {
			// drop error message, if exists
			mo.Status.Message = ""
			if e := c.updateModulePullOverrideStatus(ctx, mo); e != nil {
				return ctrl.Result{Requeue: true}, e
			}
		}
		return ctrl.Result{RequeueAfter: mo.Spec.ScanInterval.Duration}, nil
	}

	if moduleDef == nil {
		return ctrl.Result{RequeueAfter: mo.Spec.ScanInterval.Duration}, fmt.Errorf("got an empty module definition for %s module pull override", mo.Name)
	}

	err = validateModule(*moduleDef)
	if err != nil {
		mo.Status.Message = fmt.Sprintf("validation failed: %s", err)
		if e := c.updateModulePullOverrideStatus(ctx, mo); e != nil {
			return ctrl.Result{Requeue: true}, e
		}

		return ctrl.Result{RequeueAfter: mo.Spec.ScanInterval.Duration}, nil
	}

	symlinkPath := filepath.Join(c.symlinksDir, fmt.Sprintf("%d-%s", moduleDef.Weight, mo.Name))
	err = c.enableModule(mo.Name, symlinkPath)
	if err != nil {
		mo.Status.Message = err.Error()
		if e := c.updateModulePullOverrideStatus(ctx, mo); e != nil {
			return ctrl.Result{Requeue: true}, e
		}

		return ctrl.Result{Requeue: true}, err
	}

	// disable target module hooks so as not to invoke them before restart
	if c.moduleManager.GetModule(mo.Name) != nil {
		c.moduleManager.DisableModuleHooks(mo.Name)
	}

	defer func() {
		c.logger.Infof("Restarting Deckhouse because %q ModulePullOverride image was updated", mo.Name)
		err := syscall.Kill(1, syscall.SIGUSR2)
		if err != nil {
			c.logger.Fatalf("Send SIGUSR2 signal failed: %s", err)
		}
	}()

	mo.Status.Message = ""
	mo.Status.ImageDigest = newChecksum
	mo.Status.Weight = moduleDef.Weight

	if e := c.updateModulePullOverrideStatus(ctx, mo); e != nil {
		return ctrl.Result{Requeue: true}, e
	}

	if _, ok := mo.Annotations["renew"]; ok {
		delete(mo.Annotations, "renew")
		_ = c.client.Update(ctx, mo)
	}

	// update module's documentation
	modulePath := fmt.Sprintf("/%s/dev", mo.GetModuleName())
	moduleVersion := mo.Spec.ImageTag
	checksum := mo.Status.ImageDigest
	ownerRef := metav1.OwnerReference{
		APIVersion: v1alpha1.ModulePullOverrideGVK.GroupVersion().String(),
		Kind:       v1alpha1.ModulePullOverrideGVK.Kind,
		Name:       mo.GetName(),
		UID:        mo.GetUID(),
		Controller: pointer.Bool(true),
	}
	err = createOrUpdateModuleDocumentationCR(ctx, c.client, mo.GetModuleName(), moduleVersion, checksum, modulePath, mo.GetModuleSource(), ownerRef)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{RequeueAfter: mo.Spec.ScanInterval.Duration}, nil
}

func (c *modulePullOverrideReconciler) enableModule(moduleName, symlinkPath string) error {
	currentModuleSymlink, err := findExistingModuleSymlink(c.symlinksDir, moduleName)
	if err != nil {
		currentModuleSymlink = "900-" + moduleName // fallback
	}

	return enableModule(c.externalModulesDir, currentModuleSymlink, symlinkPath, path.Join("../", moduleName, downloader.DefaultDevVersion))
}

func (c *modulePullOverrideReconciler) updateModulePullOverrideStatus(ctx context.Context, mo *v1alpha1.ModulePullOverride) error {
	mo.Status.UpdatedAt = metav1.Now()
	return c.client.Status().Update(ctx, mo)
}

// restoreAbsentModulesFromOverrides checks ModulePullOverrides and restore them on the FS
func (c *modulePullOverrideReconciler) restoreAbsentModulesFromOverrides(ctx context.Context) error {
	currentNodeName := os.Getenv("DECKHOUSE_NODE_NAME")
	if len(currentNodeName) == 0 {
		return fmt.Errorf("couldn't determine the node name deckhouse pod is running on: missing or empty DECKHOUSE_NODE_NAME env")
	}

	// restoring modules from MPO
	var mpoList v1alpha1.ModulePullOverrideList
	err := c.client.List(ctx, &mpoList)
	if err != nil {
		return err
	}

	for _, item := range mpoList.Items {
		// ignore deleted Releases
		if !item.ObjectMeta.DeletionTimestamp.IsZero() {
			continue
		}

		moduleName := item.Name
		moduleSource := item.Spec.Source
		moduleImageTag := item.Spec.ImageTag
		moduleWeight := item.Status.Weight

		// get relevant module source
		ms := new(v1alpha1.ModuleSource)
		err = c.client.Get(ctx, types.NamespacedName{Name: moduleSource}, ms)
		if err != nil {
			return fmt.Errorf("ModuleSource %v for ModulePullOverride/%s/%s got an error: %w", moduleSource, moduleName, moduleImageTag, err)
		}

		// mpo's status.weight field isn't set - get it from the module's definition
		if moduleWeight == 0 {
			md := downloader.NewModuleDownloader(c.dc, c.externalModulesDir, ms, utils.GenerateRegistryOptions(ms))
			def, err := md.DownloadModuleDefinitionByVersion(moduleName, moduleImageTag)
			if err != nil {
				return fmt.Errorf("couldn't get the %s module definition from repository: %w", moduleName, err)
			}
			moduleWeight = def.Weight

			item.Status.UpdatedAt = metav1.Now()
			item.Status.Weight = def.Weight
			// we need not be bothered - even if the update fails, the weight will be set one way or another
			_ = c.client.Status().Update(ctx, &item)
		}

		// if deckhouseNodeNameAnnotation annotation isn't set or its value doesn't equal to current node name
		// we must overwrite the module from the repository
		if annotationNodeName, set := item.GetAnnotations()[deckhouseNodeNameAnnotation]; !set || annotationNodeName != currentNodeName {
			c.logger.Infof("Reinitializing module %s pull override due to stale/absent %s annotation", moduleName, deckhouseNodeNameAnnotation)
			moduleDir := path.Join(c.externalModulesDir, moduleName, downloader.DefaultDevVersion)
			if err := os.RemoveAll(moduleDir); err != nil {
				return fmt.Errorf("Couldn't delete the stale directory %s of the %s module: %s", moduleDir, moduleName, err)
			}

			if item.ObjectMeta.Annotations == nil {
				item.ObjectMeta.Annotations = make(map[string]string)
			}

			item.ObjectMeta.Annotations[deckhouseNodeNameAnnotation] = currentNodeName
			if err := c.client.Update(ctx, &item); err != nil {
				c.logger.Warnf("Couldn't annotate %s module pull override: %s", moduleName, err)
			}
		}

		// if annotations is ok - we have to check that the file system is in sync
		moduleSymLink := filepath.Join(c.symlinksDir, fmt.Sprintf("%d-%s", moduleWeight, moduleName))
		_, err = os.Stat(moduleSymLink)
		if err != nil {
			// module symlink not found
			c.logger.Infof("Module %q symlink is absent on file system. Restoring it", moduleName)
			if os.IsNotExist(err) {
				err := c.createModuleSymlink(moduleName, moduleImageTag, ms, moduleWeight)
				if err != nil {
					return fmt.Errorf("couldn't create module symlink: %s", err)
				}
				// some other error
			} else {
				return fmt.Errorf("module %s check error: %s", moduleName, err)
			}
			// check if module symlink leads to current version
		} else {
			dstDir, err := filepath.EvalSymlinks(moduleSymLink)
			if err != nil {
				return fmt.Errorf("couldn't evaluate module %s symlink %s: %s", moduleName, moduleSymLink, err)
			}
			// module symlink leads to some other version.
			// also, if dstDir doesn't exist, its Base evaluates to.
			if filepath.Base(dstDir) != downloader.DefaultDevVersion {
				c.logger.Infof("Module %q symlink is incorrect. Restoring it", moduleName)
				if err := c.createModuleSymlink(moduleName, moduleImageTag, ms, moduleWeight); err != nil {
					return fmt.Errorf("couldn't create module symlink: %s", err)
				}
			}
		}

		// sync registry spec
		if err := syncModuleRegistrySpec(c.externalModulesDir, moduleName, downloader.DefaultDevVersion, ms); err != nil {
			return fmt.Errorf("couldn't sync the %s module's registry settings with the %s module source: %w", moduleName, ms.Name, err)
		}
		c.logger.Infof("Resynced the %s module's registry settings with the %s module source", moduleName, ms.Name)
	}
	return nil
}

func (c *modulePullOverrideReconciler) createModuleSymlink(moduleName, moduleImageTag string, moduleSource *v1alpha1.ModuleSource, moduleWeight uint32) error {
	// removing possible symlink doubles
	err := wipeModuleSymlinks(c.symlinksDir, moduleName)
	if err != nil {
		return err
	}

	// check if module's directory exists on fs
	info, err := os.Stat(path.Join(c.externalModulesDir, moduleName, downloader.DefaultDevVersion))
	if err != nil || !info.IsDir() {
		// download the module to fs
		c.logger.Infof("Downloading module %q from registry", moduleName)
		md := downloader.NewModuleDownloader(c.dc, c.externalModulesDir, moduleSource, utils.GenerateRegistryOptions(moduleSource))
		_, _, err := md.DownloadDevImageTag(moduleName, moduleImageTag, "")
		if err != nil {
			return fmt.Errorf("couldn't get module %q pull override definition: %s", moduleName, err)
		}
	}

	// restore symlink
	moduleRelativePath := filepath.Join("../", moduleName, downloader.DefaultDevVersion)
	symlinkPath := filepath.Join(c.symlinksDir, fmt.Sprintf("%d-%s", moduleWeight, moduleName))
	err = restoreModuleSymlink(c.externalModulesDir, symlinkPath, moduleRelativePath)
	if err != nil {
		return fmt.Errorf("creating symlink for module %v failed: %w", moduleName, err)
	}
	c.logger.Infof("Module %s:%s restored to %s", moduleName, moduleImageTag, moduleRelativePath)
	return nil
}
