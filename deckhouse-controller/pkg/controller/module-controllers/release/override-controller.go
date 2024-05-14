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

	modulesValidator   moduleValidator
	externalModulesDir string
	symlinksDir        string
}

// NewModulePullOverrideController returns a new sample controller
func NewModulePullOverrideController(
	mgr manager.Manager,
	dc dependency.Container,
	modulesValidator moduleValidator,
) error {
	lg := log.WithField("component", "ModulePullOverrideController")

	rc := &modulePullOverrideReconciler{
		client: mgr.GetClient(),
		dc:     dc,
		logger: lg,

		modulesValidator:   modulesValidator,
		externalModulesDir: os.Getenv("EXTERNAL_MODULES_DIR"),
		symlinksDir:        filepath.Join(os.Getenv("EXTERNAL_MODULES_DIR"), "modules"),
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
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(ctr)
}

func (c *modulePullOverrideReconciler) PreflightCheck(ctx context.Context) error {
	// Check if controller's dependencies have been initialized
	return wait.PollUntilContextCancel(ctx, utils.SyncedPollPeriod, false,
		func(context.Context) (bool, error) {
			// TODO: add modulemanager initialization check c.modulesValidator.AreModulesInited() (required for reloading modules without restarting deckhouse)
			return deckhouseconfig.IsServiceInited(), nil
		})
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
	// check if RegistrySpecChangedAnnotation annotation is set and processes it
	if _, set := mo.GetAnnotations()[RegistrySpecChangedAnnotation]; set {
		// if module is enabled - push runModule task in the main queue
		c.logger.Infof("Applying new registry settings to the %s module", mo.Name)
		err := c.modulesValidator.RunModuleWithNewStaticValues(mo.Name, mo.ObjectMeta.Labels["source"], filepath.Join(c.externalModulesDir, mo.Name, "dev"))
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		// delete annotation and requeue
		delete(mo.ObjectMeta.Annotations, RegistrySpecChangedAnnotation)
		err = c.client.Update(ctx, mo)
		return ctrl.Result{Requeue: true}, err
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
		err := c.client.Update(ctx, mo)
		return ctrl.Result{RequeueAfter: 500 * time.Millisecond}, err
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

	err = validateModule(c.modulesValidator, *moduleDef)
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
	if c.modulesValidator.GetModule(mo.Name) != nil {
		c.modulesValidator.DisableModuleHooks(mo.Name)
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

	return enableModule(c.externalModulesDir, currentModuleSymlink, symlinkPath, path.Join("../", moduleName, "dev"))
}

func (c *modulePullOverrideReconciler) updateModulePullOverrideStatus(ctx context.Context, mo *v1alpha1.ModulePullOverride) error {
	mo.Status.UpdatedAt = metav1.Now()
	return c.client.Status().Update(ctx, mo)
}
