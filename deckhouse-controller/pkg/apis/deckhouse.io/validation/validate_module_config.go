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

	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	"github.com/slok/kubewebhook/v2/pkg/model"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	d8config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
)

type AnnotationsOnly struct {
	ObjectMeta `json:"metadata,omitempty"`
}

type ObjectMeta struct {
	Annotations map[string]string `json:"annotations,omitempty"`
}

// moduleConfigValidationHandler validations for ModuleConfig creation
func moduleConfigValidationHandler(moduleStorage ModuleStorage) http.Handler {
	vf := kwhvalidating.ValidatorFunc(func(_ context.Context, review *model.AdmissionReview, obj metav1.Object) (result *kwhvalidating.ValidatorResult, err error) {
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

				// if we have no annotation and module enabled
				//
				// we check module
				// we check confirmation restriction and confirmation message
				_, ok = cfg.Annotations[v1alpha1.AllowDisableAnnotion]
				if !ok && *cfg.Spec.Enabled {
					module, err := moduleStorage.GetModuleByName(obj.GetName())
					if err != nil {
						return rejectResult(fmt.Sprintf("Module '%s' not registered in deckhouse", obj.GetName()))
					}

					reason, needConfirm := module.GetConfirmationReason()
					if needConfirm {
						return rejectResult(reason)
					}
				}

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
			err = json.NewDecoder(buf).Decode(oldModuleMeta)
			if err != nil {
				return nil, fmt.Errorf("can not parse old module config: %w", err)
			}

			cfg, ok = obj.(*v1alpha1.ModuleConfig)
			if !ok {
				return nil, fmt.Errorf("expect ModuleConfig as unstructured, got %T", obj)
			}

			// if we have no annotation on current ModuleConfig
			// and we have no annotation on previous ModuleConfig (if we want to take off annotation, while disabled)
			// and module is disabled
			//
			// we check module
			// we check confirmation restriction and confirmation message
			_, ok = cfg.Annotations[v1alpha1.AllowDisableAnnotion]
			_, oldOk := oldModuleMeta.Annotations[v1alpha1.AllowDisableAnnotion]
			if !ok && !oldOk && !*cfg.Spec.Enabled {
				module, err := moduleStorage.GetModuleByName(obj.GetName())
				if err != nil {
					return rejectResult(fmt.Sprintf("Module '%s' not registered in deckhouse", obj.GetName()))
				}

				reason, needConfirm := module.GetConfirmationReason()
				if needConfirm {
					return rejectResult(reason)
				}
			}
		}

		// Allow changing configuration for unknown modules.
		if !d8config.Service().PossibleNames().Has(cfg.Name) {
			return allowResult(fmt.Sprintf("module name '%s' is unknown for deckhouse", cfg.Name))
		}

		// Check if spec.version value is valid and the version is the latest.
		// Validate spec.settings using the OpenAPI schema.
		res := d8config.Service().ConfigValidator().Validate(cfg)
		if res.HasError() {
			return rejectResult(res.Error)
		}

		// Return allow with warning.
		return allowResult(res.Warning)
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
