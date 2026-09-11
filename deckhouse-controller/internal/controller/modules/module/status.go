// Copyright 2026 Flant JSC
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

package module

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	dynamicextender "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/dynamically_enabled"
	kubeconfigextender "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/kube_config"
	scriptextender "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/script_enabled"
	staticextender "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/static"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/module/status"
	bootstrappedextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/bootstrapped"
	d7sversionextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/deckhouseversion"
	editionavailablextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/editionavailable"
	editionenabledextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/editionenabled"
	k8sversionextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/kubernetesversion"
	moduledependencyextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/moduledependency"
)

// refreshModule refreshes module in cluster
func (r *reconciler) refreshModule(ctx context.Context, moduleName string) error {
	r.logger.Debug("refresh module status", slog.String("name", moduleName))

	// events happen quite often, so conflicts happen often, default backoff not suitable
	backoff := wait.Backoff{
		Steps: 6,
		// magic number
		Duration: 20 * time.Millisecond,
		Factor:   1.0,
		Jitter:   0.1,
	}

	module := new(v1alpha2.Module)
	if err := retry.OnError(retry.DefaultRetry, apierrors.IsServiceUnavailable, func() error {
		return retry.RetryOnConflict(backoff, func() error {
			if err := r.client.Get(ctx, client.ObjectKey{Name: moduleName}, module); err != nil {
				return fmt.Errorf("get: %w", err)
			}
			r.refreshModuleStatus(module)
			return r.client.Status().Update(ctx, module)
		})
	}); err != nil {
		return fmt.Errorf("on error: %w", err)
	}
	return nil
}

// refreshModuleStatus refreshes the Module status from addon-operator's live
// view of the module. The result is expressed in the v1alpha2 status model:
// the Enabled and Ready conditions and the high-level Summary.State, one of
// Pending, Failed, Updating, Ready, Degraded or Suspended.
//
// The runtime conditions the package status service owns (Installed, Scaled,
// Managed, ...) are deliberately left untouched here — this controller only
// reflects what addon-operator can tell it: whether the scheduler runs the
// module and whether its hooks and run succeeded.
func (r *reconciler) refreshModuleStatus(module *v1alpha2.Module) {
	basicModule := r.moduleManager.GetModule(module.Name)
	if basicModule == nil {
		return
	}

	if module.Status.Summary == nil {
		module.Status.Summary = &v1alpha2.ModuleStatusSummary{}
	}

	// Remember whether the module was already running before this refresh. It is
	// the only signal that separates a first install from a reconcile of a working
	// version, and a scheduler switch-off of a running module from a module that
	// never started.
	wasReady := module.IsCondition(status.ConditionReady, metav1.ConditionTrue)

	if r.moduleManager.IsModuleEnabled(module.Name) {
		r.refreshEnabledModuleStatus(module, basicModule, wasReady)
		return
	}

	r.refreshDisabledModuleStatus(module, basicModule, wasReady)
}

// refreshEnabledModuleStatus reflects the state of a module the scheduler runs.
func (r *reconciler) refreshEnabledModuleStatus(module *v1alpha2.Module, basicModule *modules.BasicModule, wasReady bool) {
	module.SetConditionTrue(status.ConditionEnabled)

	// A module or hook failure means the module is not working. On a first install
	// there is no previous version to fall back on (Failed); on a running module
	// the previous run still stands but has become unhealthy (Degraded).
	brokenState := status.StateFailed
	if wasReady {
		brokenState = status.StateDegraded
	}

	if moduleErr := basicModule.GetModuleError(); moduleErr != nil {
		module.Status.Summary.State = brokenState
		module.SetConditionFalse(status.ConditionReady, v1alpha1.ModuleReasonModuleError, moduleErr.Error())
		return
	}

	if hookErr := basicModule.GetLastHookError(); hookErr != nil {
		module.Status.Summary.State = brokenState
		module.SetConditionFalse(status.ConditionReady, v1alpha1.ModuleReasonHookError, hookErr.Error())
		return
	}

	// The module has fully converged: every run phase completed.
	if basicModule.GetPhase() == modules.Ready {
		module.Status.Summary.State = status.StateReady
		module.SetConditionTrue(status.ConditionReady)
		return
	}

	// Still converging. A running module keeps serving its previous version while
	// it re-applies, so it stays Ready and reports Updating rather than flapping;
	// a first install has nothing to serve yet, so it is Pending with Ready=False.
	if wasReady {
		module.Status.Summary.State = status.StateUpdating
		module.SetConditionTrue(status.ConditionReady)
		return
	}

	module.Status.Summary.State = status.StatePending
	module.SetConditionFalse(status.ConditionReady, v1alpha1.ModuleReasonInstalling, v1alpha1.ModuleMessageInstalling)
}

// refreshDisabledModuleStatus reflects the state of a module the scheduler does
// not run. A module that was working and got switched off is Suspended; a module
// that never started and is blocked by the bundle, an extender or an explicit
// disable is Pending. Either way the reason explains which gate keeps it off.
func (r *reconciler) refreshDisabledModuleStatus(module *v1alpha2.Module, basicModule *modules.BasicModule, wasReady bool) {
	updatedBy, err := r.moduleManager.GetUpdatedByExtender(module.Name)
	if err != nil {
		module.Status.Summary.State = status.StateFailed
		module.SetConditionFalse(status.ConditionEnabled, v1alpha1.ModuleReasonError, err.Error())
		module.SetConditionFalse(status.ConditionReady, v1alpha1.ModuleReasonError, err.Error())
		return
	}

	reason, message := r.disabledReason(module, basicModule, updatedBy)

	if wasReady {
		module.Status.Summary.State = status.StateSuspended
	} else {
		module.Status.Summary.State = status.StatePending
	}

	module.SetConditionFalse(status.ConditionEnabled, reason, message)
	module.SetConditionFalse(status.ConditionReady, reason, message)
}

// disabledReason translates the extender that owns the scheduler's decision into
// a user-facing reason and message explaining why the module is switched off.
func (r *reconciler) disabledReason(module *v1alpha2.Module, basicModule *modules.BasicModule, updatedBy string) (string, string) {
	switch extenders.ExtenderName(updatedBy) {
	case "", staticextender.Name:
		if module.IsEmbedded() {
			return v1alpha1.ModuleReasonBundle, v1alpha1.ModuleMessageBundle
		}
		return v1alpha1.ModuleReasonDisabled, v1alpha1.ModuleMessageDisabled

	case kubeconfigextender.Name:
		return v1alpha1.ModuleReasonModuleConfig, v1alpha1.ModuleMessageModuleConfig

	case dynamicextender.Name:
		return v1alpha1.ModuleReasonDynamicGlobalHookExtender, v1alpha1.ModuleMessageDynamicGlobalHookExtender

	case scriptextender.Name:
		message := v1alpha1.ModuleMessageEnabledScriptExtender
		if txt := basicModule.GetEnabledScriptReason(); txt != nil && *txt != "" {
			message += ": " + *txt
		}
		return v1alpha1.ModuleReasonEnabledScriptExtender, message

	case d7sversionextender.Name:
		message := v1alpha1.ModuleMessageDeckhouseVersionExtender
		if _, errMsg := r.exts.GetDeckhouseVersion().Filter(module.Name, map[string]string{}); errMsg != nil {
			message += ": " + errMsg.Error()
		}
		return v1alpha1.ModuleReasonDeckhouseVersionExtender, message

	case editionavailablextender.Name:
		message := ""
		if _, errMsg := r.exts.GetEditionAvailable().Filter(module.Name, map[string]string{}); errMsg != nil {
			message = errMsg.Error()
		}
		return v1alpha1.ModuleReasonEditionAvailableExtender, message

	case editionenabledextender.Name:
		message := ""
		if _, errMsg := r.exts.GetEditionEnabled().Filter(module.Name, map[string]string{}); errMsg != nil {
			message = errMsg.Error()
		}
		return v1alpha1.ModuleReasonEditionEnabledExtender, message

	case k8sversionextender.Name:
		message := v1alpha1.ModuleMessageKubernetesVersionExtender
		if _, errMsg := k8sversionextender.Instance().Filter(module.Name, map[string]string{}); errMsg != nil {
			message += ": " + errMsg.Error()
		}
		return v1alpha1.ModuleReasonKubernetesVersionExtender, message

	case bootstrappedextender.Name:
		return v1alpha1.ModuleReasonBootstrappedExtender, v1alpha1.ModuleMessageBootstrappedExtender

	case moduledependencyextender.Name:
		message := v1alpha1.ModuleMessageModuleDependencyExtender
		if _, errMsg := moduledependencyextender.Instance().Filter(module.Name, map[string]string{}); errMsg != nil {
			message += ": " + errMsg.Error()
		}
		return v1alpha1.ModuleReasonModuleDependencyExtender, message
	}

	return v1alpha1.ModuleReasonDisabled, v1alpha1.ModuleMessageDisabled
}
