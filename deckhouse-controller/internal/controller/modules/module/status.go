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

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/module/status"
	bootstrappedextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/bootstrapped"
	d7sversionextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/deckhouseversion"
	editionavailablextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/editionavailable"
	editionenabledextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/editionenabled"
	k8sversionextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/kubernetesversion"
	moduledependencyextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/moduledependency"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	dynamicextender "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/dynamically_enabled"
	kubeconfigextender "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/kube_config"
	scriptextender "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/script_enabled"
	staticextender "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/static"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// refreshModuleStatus refreshes module status by addon-operator
func (r *reconciler) refreshModuleStatus(module *v1alpha2.Module) {
	basicModule := r.moduleManager.GetModule(module.Name)
	if basicModule == nil {
		return
	}

	if r.moduleManager.IsModuleEnabled(module.Name) {
		module.SetConditionTrue(status.ConditionEnabled)

		if hookErr := basicModule.GetLastHookError(); hookErr != nil {
			module.Status.Summary.State = status.StateFailed
			module.SetConditionFalse(status.ConditionReady, "HookFailed", hookErr.Error())
			return
		}

		if moduleError := basicModule.GetModuleError(); moduleError != nil {
			module.Status.Summary.State = status.StateFailed
			module.SetConditionFalse(status.ConditionReady, "LoadFromFilesystemFailed", moduleError.Error())
			return
		}

		switch basicModule.GetPhase() {
		// Best effort alarm!
		//
		// Actually, this condition is not correct because the `CanRunHelm` status appears right before the first run.c
		// The right approach is to check the queue for the module run task.
		// However, there are too many addon-operator internals involved.
		// We should consider moving these statuses to the `Module` resource,
		// which is directly controlled by addon-operator.
		case modules.Ready:
			if !basicModule.HasReadiness() {
				module.Status.Summary.State = status.StateReady
				module.SetConditionTrue(status.ConditionReady)
			}

		case modules.Startup:
			if module.Status.Summary.State == status.StateUpdating {
				module.Status.Summary.State = status.StateDegraded
				module.SetConditionFalse(status.ConditionReady, "Pending", "installing")
			} else {
				module.Status.Summary.State = status.StateDegraded
				module.SetConditionFalse(status.ConditionReady, "Reconciling", "reconciling")
			}

		case modules.OnStartupDone:
			if module.Status.Summary.State != status.StateUpdating {
				module.SetConditionFalse(status.ConditionReady, v1alpha1.ModuleReasonReconciling, v1alpha1.ModuleMessageOnStartupHook)
			} else {
				module.SetConditionFalse(status.ConditionReady, v1alpha1.ModuleReasonInstalling, v1alpha1.ModuleMessageOnStartupHook)
			}
		}

		return
	}

	updatedBy, updatedByErr := r.moduleManager.GetUpdatedByExtender(module.Name)
	if updatedByErr != nil {
		module.Status.Summary.State = status.StateFailed
		module.SetConditionFalse(status.ConditionEnabled, v1alpha1.ModuleReasonError, updatedByErr.Error())
		module.SetConditionFalse(status.ConditionReady, v1alpha1.ModuleReasonError, updatedByErr.Error())
		return
	}

	var reason string
	var message string

	switch extenders.ExtenderName(updatedBy) {
	case "", staticextender.Name:
		reason = v1alpha1.ModuleReasonBundle
		message = v1alpha1.ModuleMessageBundle
		if !module.IsEmbedded() {
			reason = v1alpha1.ModuleReasonDisabled
			message = v1alpha1.ModuleMessageDisabled
		}

	case kubeconfigextender.Name:
		reason = v1alpha1.ModuleReasonModuleConfig
		message = v1alpha1.ModuleMessageModuleConfig

	case dynamicextender.Name:
		reason = v1alpha1.ModuleReasonDynamicGlobalHookExtender
		message = v1alpha1.ModuleMessageDynamicGlobalHookExtender

	case scriptextender.Name:
		reason = v1alpha1.ModuleReasonEnabledScriptExtender
		message = v1alpha1.ModuleMessageEnabledScriptExtender
		if txt := basicModule.GetEnabledScriptReason(); txt != nil && *txt != "" {
			message += ": " + *txt
		}
	case d7sversionextender.Name:
		reason = v1alpha1.ModuleReasonDeckhouseVersionExtender
		_, errMsg := r.exts.GetDeckhouseVersion().Filter(module.Name, map[string]string{})
		message = v1alpha1.ModuleMessageDeckhouseVersionExtender
		if errMsg != nil {
			message += ": " + errMsg.Error()
		}

	case editionavailablextender.Name:
		module.Status.Summary.State = status.StateFailed
		reason = v1alpha1.ModuleReasonEditionAvailableExtender
		_, errMsg := r.exts.GetEditionAvailable().Filter(module.Name, map[string]string{})
		if errMsg != nil {
			message = errMsg.Error()
		}

	case editionenabledextender.Name:
		module.Status.Summary.State = status.StateUpdating
		reason = v1alpha1.ModuleReasonEditionEnabledExtender
		_, errMsg := r.exts.GetEditionEnabled().Filter(module.Name, map[string]string{})
		if errMsg != nil {
			message = errMsg.Error()
		}

	case k8sversionextender.Name:
		reason = v1alpha1.ModuleReasonKubernetesVersionExtender
		_, errMsg := k8sversionextender.Instance().Filter(module.Name, map[string]string{})
		message = v1alpha1.ModuleMessageKubernetesVersionExtender
		if errMsg != nil {
			message += ": " + errMsg.Error()
		}

	case bootstrappedextender.Name:
		reason = v1alpha1.ModuleReasonBootstrappedExtender
		message = v1alpha1.ModuleMessageBootstrappedExtender

	case moduledependencyextender.Name:
		reason = v1alpha1.ModuleReasonModuleDependencyExtender
		_, errMsg := moduledependencyextender.Instance().Filter(module.Name, map[string]string{})
		message = v1alpha1.ModuleMessageModuleDependencyExtender
		if errMsg != nil {
			message += ": " + errMsg.Error()
		}
	}

	// do not change phase of not installed module
	if module.Status.Summary.State != status.StateFailed && module.Status.Summary.State != status.StateSuspended {
		module.Status.Summary.State = status.StateUpdating
	}

	module.SetConditionFalse(status.ConditionEnabled, reason, message)
	module.SetConditionFalse(status.ConditionReady, reason, message)
}
