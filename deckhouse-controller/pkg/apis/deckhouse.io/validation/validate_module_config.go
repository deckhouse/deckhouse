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

	"github.com/flant/shell-operator/pkg/metric_storage"
	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/configtools"
)

type AnnotationsOnly struct {
	ObjectMeta `json:"metadata,omitempty"`
}

type ObjectMeta struct {
	Annotations map[string]string `json:"annotations,omitempty"`
}

const disableReasonSuffix = "Please annotate ModuleConfig with `modules.deckhouse.io/allow-disable=true` if you're sure that you want to disable the module."

// moduleConfigValidationHandler validations for ModuleConfig creation
func moduleConfigValidationHandler(cli client.Client, moduleStorage moduleStorage, metricStorage *metric_storage.MetricStorage, configValidator *configtools.Validator) http.Handler {
	vf := kwhvalidating.ValidatorFunc(func(_ context.Context, review *kwhmodel.AdmissionReview, obj metav1.Object) (result *kwhvalidating.ValidatorResult, err error) {
		var (
			cfg = new(v1alpha1.ModuleConfig)
			ok  bool
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

				metricStorage.GaugeSet("d8_moduleconfig_allowed_to_disable", 0, map[string]string{"module": cfg.GetName()})

				// if module is already disabled - we don't need to warn user about disabling module
				return allowResult("")
			}

		case kwhmodel.OperationConnect, kwhmodel.OperationUnknown:
			return rejectResult(fmt.Sprintf("operation '%s' is not applicable", review.Operation))

		case kwhmodel.OperationCreate:
			cfg, ok = obj.(*v1alpha1.ModuleConfig)
			if !ok {
				return nil, fmt.Errorf("expect ModuleConfig as unstructured, got %T", obj)
			}

		case kwhmodel.OperationUpdate:
			oldModuleMeta := new(AnnotationsOnly)

			buf := bytes.NewBuffer(review.OldObjectRaw)
			if err = json.NewDecoder(buf).Decode(oldModuleMeta); err != nil {
				return nil, fmt.Errorf("can not parse old module config: %w", err)
			}

			cfg, ok = obj.(*v1alpha1.ModuleConfig)
			if !ok {
				return nil, fmt.Errorf("expect ModuleConfig as unstructured, got %T", obj)
			}

			// if no annotations and module is disabled, check confirmation restriction and confirmation message
			_, ok = cfg.Annotations[v1alpha1.ModuleConfigAnnotationAllowDisable]
			_, oldOk := oldModuleMeta.Annotations[v1alpha1.ModuleConfigAnnotationAllowDisable]
			if !ok && !oldOk && cfg.Spec.Enabled != nil && !*cfg.Spec.Enabled {
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

		module := new(v1alpha1.Module)
		if err = cli.Get(context.Background(), client.ObjectKey{Name: cfg.Name}, module); err != nil {
			if apierrors.IsNotFound(err) {
				return allowResult(fmt.Sprintf("the '%s' module not found", cfg.Name))
			}
			return nil, fmt.Errorf("get the '%s' module: %w", cfg.Name, err)
		}

		if cfg.Spec.Source != "" && !slices.Contains(module.Properties.AvailableSources, cfg.Spec.Source) {
			return rejectResult(fmt.Sprintf("the '%s' module source is an unavailable source for the '%s' module, available sources: %v", cfg.Spec.Source, cfg.Name, module.Properties.AvailableSources))
		}

		var warning string

		// empty policy means module uses deckhouse embedded policy
		if cfg.Spec.UpdatePolicy != "" {
			tmp := new(v1alpha1.ModuleUpdatePolicy)
			if err = cli.Get(context.Background(), client.ObjectKey{Name: cfg.Spec.UpdatePolicy}, tmp); err != nil {
				if !apierrors.IsNotFound(err) {
					return nil, fmt.Errorf("get the '%s' module policy: %w", cfg.Spec.UpdatePolicy, err)
				}
				warning = fmt.Sprintf("the '%s' module policy not found, the policy from the deckhouse settings will be used instead", cfg.Spec.UpdatePolicy)
			}
		}

		// check if spec.version value is valid and the version is the latest.
		if res := configValidator.Validate(cfg); res.HasError() {
			return rejectResult(res.Error)
		} else if res.Warning != "" {
			warning = res.Warning
		}

		metricStorage.GaugeSet("d8_moduleconfig_allowed_to_disable", allowedToDisableMetric, map[string]string{"module": cfg.GetName()})

		// Return allow with warning.
		return allowResult(warning)
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

func allowResult(warnMsg string) (*kwhvalidating.ValidatorResult, error) {
	var warnings []string
	if warnMsg != "" {
		warnings = []string{warnMsg}
	}
	return &kwhvalidating.ValidatorResult{
		Valid:    true,
		Warnings: warnings,
	}, nil
}

func rejectResult(msg string) (*kwhvalidating.ValidatorResult, error) {
	return &kwhvalidating.ValidatorResult{
		Valid:   false,
		Message: msg,
	}, nil
}
