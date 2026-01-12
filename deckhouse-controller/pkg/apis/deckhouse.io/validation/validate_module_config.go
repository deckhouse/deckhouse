/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package validation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/configtools"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

type AnnotationsOnly struct {
	ObjectMeta `json:"metadata,omitempty"`
}

type EnabledOnly struct {
	Spec struct {
		Enabled *bool `json:"enabled,omitempty"`
	} `json:"spec,omitempty"`
}
type ObjectMeta struct {
	Annotations map[string]string `json:"annotations,omitempty"`
}

const disableReasonSuffix = "Please annotate ModuleConfig with `modules.deckhouse.io/allow-disabling=true` if you're sure that you want to disable the module."

// moduleConfigValidationHandler validations for ModuleConfig creation
func moduleConfigValidationHandler(
	cli client.Client,
	moduleStorage moduleStorage,
	metricStorage metricsstorage.Storage,
	moduleManager moduleManager,
	configValidator *configtools.Validator,
	setting *helpers.DeckhouseSettingsContainer,
	exts *extenders.ExtendersStack,
) http.Handler {
	vf := kwhvalidating.ValidatorFunc(func(ctx context.Context, review *kwhmodel.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
		var (
			cfg                      = new(v1alpha1.ModuleConfig)
			ok                       bool
			allowExperimentalModules = setting.Get().AllowExperimentalModules
		)

		switch review.Operation {
		case kwhmodel.OperationDelete:
			{
				cfg, ok = obj.(*v1alpha1.ModuleConfig)
				if !ok {
					return nil, fmt.Errorf("expect ModuleConfig as unstructured, got %T", obj)
				}

				if _, ok = cfg.Annotations[v1alpha1.ModuleConfigAnnotationAllowDisable]; !ok {
					if cfg.Spec.Enabled != nil && *cfg.Spec.Enabled {
						// we can delete unknown module without any further check
						if module, err := moduleStorage.GetModuleByName(obj.GetName()); err == nil {
							if reason, needConfirm := module.GetConfirmationDisableReason(); needConfirm {
								if !strings.HasSuffix(reason, ".") {
									reason += "."
								}
								reason += disableReasonSuffix

								return rejectResult(reason)
							}
						}
					}
				}

				exists, err := utils.ModulePullOverrideExists(ctx, cli, cfg.Name)
				if err != nil {
					return nil, fmt.Errorf("get the '%s' module pull override: %w", cfg.Name, err)
				}
				if exists {
					return rejectResult("delete the ModulePullOverride before deleting the module config")
				}

				metricStorage.GaugeSet(metrics.D8ModuleConfigAllowedToDisable, 0, map[string]string{"module": cfg.GetName()})
				// if module is already disabled - we don't need to warn user about disabling module
				return allowResult(nil)
			}

		case kwhmodel.OperationConnect, kwhmodel.OperationUnknown:
			return rejectResult(fmt.Sprintf("operation '%s' is not applicable", review.Operation))

		case kwhmodel.OperationCreate:
			cfg, ok = obj.(*v1alpha1.ModuleConfig)
			if !ok {
				return nil, fmt.Errorf("expect ModuleConfig as unstructured, got %T", obj)
			}

			if cfg.Spec.Enabled != nil && *cfg.Spec.Enabled {
				if module, err := moduleStorage.GetModuleByName(obj.GetName()); err == nil {
					definition := module.GetModuleDefinition()

					if definition.IsExperimental() && !allowExperimentalModules {
						return rejectResult(fmt.Sprintf("the '%s' module is experimental, set param in 'deckhouse' ModuleConfig - spec.settings.allowExperimentalModules: true to allow it", cfg.Name))
					}
				}

				if err := exts.ModuleDependency.CheckEnabling(cfg.Name); err != nil {
					return rejectResult(err.Error())
				}
			}
		case kwhmodel.OperationUpdate:
			oldModuleMeta := new(AnnotationsOnly)

			buf := bytes.NewBuffer(review.OldObjectRaw)
			if err := json.NewDecoder(buf).Decode(oldModuleMeta); err != nil {
				return nil, fmt.Errorf("can not parse old module config: %w", err)
			}

			buf = bytes.NewBuffer(review.OldObjectRaw)
			oldFlag := new(EnabledOnly)
			if err := json.NewDecoder(buf).Decode(oldFlag); err != nil {
				return nil, fmt.Errorf("can not parse old module config: %w", err)
			}

			cfg, ok = obj.(*v1alpha1.ModuleConfig)
			if !ok {
				return nil, fmt.Errorf("expect ModuleConfig as unstructured, got %T", obj)
			}

			oldEnabled := oldFlag.Spec.Enabled != nil && *oldFlag.Spec.Enabled
			newEnabled := cfg.Spec.Enabled != nil && *cfg.Spec.Enabled

			if !oldEnabled && newEnabled {
				if module, err := moduleStorage.GetModuleByName(obj.GetName()); err == nil {
					definition := module.GetModuleDefinition()

					if definition.IsExperimental() && !allowExperimentalModules {
						return rejectResult(fmt.Sprintf("the '%s' module is experimental, set param in 'deckhouse' ModuleConfig - spec.settings.allowExperimentalModules: true to allow it", cfg.Name))
					}
				}

				if err := exts.ModuleDependency.CheckEnabling(cfg.Name); err != nil {
					return rejectResult(err.Error())
				}
			}

			// if no annotations and module is disabled, check confirmation restriction and confirmation message
			_, ok = cfg.Annotations[v1alpha1.ModuleConfigAnnotationAllowDisable]
			_, oldOk := oldModuleMeta.Annotations[v1alpha1.ModuleConfigAnnotationAllowDisable]

			if !ok && !oldOk && oldEnabled && !newEnabled {
				// we can disable unknown module without any further check
				if module, err := moduleStorage.GetModuleByName(obj.GetName()); err == nil {
					if reason, needConfirm := module.GetConfirmationDisableReason(); needConfirm {
						if !strings.HasSuffix(reason, ".") {
							reason += "."
						}
						reason += disableReasonSuffix

						return rejectResult(reason)
					}
				}
			}
		}

		var allowedToDisableMetric float64
		if _, ok = cfg.Annotations[v1alpha1.ModuleConfigAnnotationAllowDisable]; ok {
			if cfg.Spec.Enabled != nil && *cfg.Spec.Enabled {
				allowedToDisableMetric = 1
			}
		}

		if cfg.Spec.Source == v1alpha1.ModuleSourceEmbedded {
			return rejectResult("'Embedded' is a forbidden source")
		}

		warnings := make([]string, 0, 1)

		// skip checking source for the global module
		if cfg.Name != "global" {
			module := new(v1alpha1.Module)
			if err := cli.Get(ctx, client.ObjectKey{Name: cfg.Name}, module); err != nil {
				if apierrors.IsNotFound(err) {
					return allowResult([]string{fmt.Sprintf("the '%s' module not found", cfg.Name)})
				}
				return nil, fmt.Errorf("get the '%s' module: %w", cfg.Name, err)
			}

			if cfg.Spec.Source != "" && !slices.Contains(module.Properties.AvailableSources, cfg.Spec.Source) {
				return rejectResult(fmt.Sprintf("the '%s' module source is an unavailable source for the '%s' module, available sources: %v", cfg.Spec.Source, cfg.Name, module.Properties.AvailableSources))
			}

			if cfg.Spec.Enabled != nil && *cfg.Spec.Enabled && cfg.Spec.Source == "" && len(module.Properties.AvailableSources) > 1 {
				warnings = append(warnings, fmt.Sprintf("module '%s' is enabled but didn’t run because multiple sources were found (%s), please specify a source in ModuleConfig resource ", cfg.GetName(), strings.Join(module.Properties.AvailableSources, ", ")))
			}
		}

		// empty policy means module uses deckhouse embedded policy
		if cfg.Spec.UpdatePolicy != "" {
			tmp := new(v1alpha2.ModuleUpdatePolicy)
			if err := cli.Get(ctx, client.ObjectKey{Name: cfg.Spec.UpdatePolicy}, tmp); err != nil {
				if !apierrors.IsNotFound(err) {
					return nil, fmt.Errorf("get the '%s' module policy: %w", cfg.Spec.UpdatePolicy, err)
				}
				return rejectResult(fmt.Sprintf("the '%s' module policy does not exist", cfg.Spec.UpdatePolicy))
			}
		}

		// check if spec.version value is valid and the version is the latest.
		if res := configValidator.Validate(cfg); res.HasError() {
			return rejectResult(res.Error)
		} else if res.Warning != "" {
			warnings = append(warnings, res.Warning)
		}

		metricStorage.GaugeSet(metrics.D8ModuleConfigAllowedToDisable, allowedToDisableMetric, map[string]string{"module": cfg.GetName()})

		module, err := moduleStorage.GetModuleByName(cfg.Name)
		if err != nil {
			return allowResult(warnings)
		}
		exclusiveGroup := module.GetModuleExclusiveGroup()
		if exclusiveGroup != nil {
			modules := moduleStorage.GetModulesByExclusiveGroup(*exclusiveGroup)

			for _, moduleName := range modules {
				// if any module with same unique key enabled, return error
				if moduleManager.IsModuleEnabled(moduleName) && moduleName != cfg.Name {
					return rejectResult(
						fmt.Sprintf(
							"can't enable module %q because different module %q with same exclusiveGroup %s enabled",
							cfg.Name,
							moduleName,
							*exclusiveGroup,
						),
					)
				}
			}
		}

		// Return allow with warning.
		return allowResult(warnings)
	})

	// Create webhook.
	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "module-config-operations",
		Validator: vf,
		// logger is nil, because webhook has Info level for reporting about http handler
		// and we get a log of useless spam here. So we decided to use Noop logger here
		Logger: nil,
		Obj:    &v1alpha1.ModuleConfig{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: nil})
}

func allowResult(warnMsgs []string) (*kwhvalidating.ValidatorResult, error) {
	res := &kwhvalidating.ValidatorResult{
		Valid: true,
	}

	if len(warnMsgs) > 0 {
		res.Warnings = warnMsgs
	}

	return res, nil
}

func rejectResult(msg string) (*kwhvalidating.ValidatorResult, error) {
	return &kwhvalidating.ValidatorResult{
		Valid:   false,
		Message: msg,
	}, nil
}
