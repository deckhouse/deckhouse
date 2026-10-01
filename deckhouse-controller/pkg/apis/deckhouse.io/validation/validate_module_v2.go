/*
Copyright 2026 Flant JSC

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
	"context"
	"fmt"
	"net/http"

	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1beta1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

// deckhouseServiceAccount is the identity the controller writes Modules under.
const deckhouseServiceAccount = "system:serviceaccount:d8-system:deckhouse"

// moduleValidationHandlerV2 validates Module admission requests against the package version the Module selects.
func moduleValidationHandlerV2(
	cli client.Client,
	packages modulePackageManager,
	metricStorage metricsstorage.Storage,
	settings *helpers.DeckhouseSettingsContainer,
) http.Handler {
	validator := &moduleValidatorV2Handler{moduleValidatorV2: newModuleValidatorV2(cli, packages, metricStorage, settings)}

	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "module-operations",
		Validator: kwhvalidating.ValidatorFunc(validator.validate),
		// logger is nil, because webhook has Info level for reporting about http handler
		// and we get a log of useless spam here. So we decided to use Noop logger here
		Logger: nil,
		Obj:    &v1beta1.Module{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: nil})
}

// moduleValidatorV2Handler validates a Module edited directly by a user.
type moduleValidatorV2Handler struct {
	*moduleValidatorV2
}

// moduleRequestFrom reads a Module; nil stays nil.
func moduleRequestFrom(module *v1beta1.Module) *moduleRequest {
	if module == nil {
		return nil
	}

	return &moduleRequest{
		name:            module.Name,
		enabled:         module.Spec.Enabled,
		settingsVersion: module.Spec.SettingsVersion,
		settings:        module.Spec.Settings,
		annotations:     module.Annotations,
	}
}

// validate is the admission entrypoint.
func (v *moduleValidatorV2Handler) validate(ctx context.Context, review *kwhmodel.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
	module, ok := obj.(*v1beta1.Module)
	if !ok {
		return nil, fmt.Errorf("expect Module as unstructured, got %T", obj)
	}

	// the controller proxies ModuleConfig into Module, and that ModuleConfig passed its own webhook
	if review.UserInfo.Username == deckhouseServiceAccount {
		return allowResult(nil)
	}

	switch review.Operation {
	case kwhmodel.OperationDelete:
		return v.validateDelete(ctx, module)

	case kwhmodel.OperationConnect, kwhmodel.OperationUnknown:
		return rejectResult(fmt.Sprintf("operation '%s' is not applicable", review.Operation))
	}

	// no sense to check an already deleted module
	if !module.DeletionTimestamp.IsZero() {
		return allowResult(nil)
	}

	var oldModule *v1beta1.Module
	if review.Operation == kwhmodel.OperationUpdate {
		oldModule = new(v1beta1.Module)
		if err := decodeOldObject(review.OldObjectRaw, oldModule); err != nil {
			return nil, err
		}
	}

	req, oldReq := moduleRequestFrom(module), moduleRequestFrom(oldModule)
	if res, err := v.validateApplyGuards(ctx, req, oldReq); res != nil || err != nil {
		return res, err
	}

	// unlike a ModuleConfig, a Module names its version, so a version that does not exist is the user's mistake
	version, reason, err := v.versionFor(ctx, module)
	if err != nil {
		return nil, err
	}
	if version == nil {
		return rejectResult(reason)
	}

	warnings, res, err := v.validatePackage(ctx, req, oldReq, &moduleTarget{module: module, version: version})
	if res != nil || err != nil {
		return res, err
	}

	return allowResult(warnings)
}

// validateDelete runs the control-plane-manager guards and the disable confirmation.
func (v *moduleValidatorV2Handler) validateDelete(ctx context.Context, module *v1beta1.Module) (*kwhvalidating.ValidatorResult, error) {
	req := moduleRequestFrom(module)
	if res, err := v.validateDeleteGuards(ctx, req); res != nil || err != nil {
		return res, err
	}

	if hasAllowDisableAnnotation(module.Annotations) || !v.packages.IsPackageEnabled(module.Name) {
		return allowResult(nil)
	}

	version, _, err := v.versionFor(ctx, module)
	if err != nil {
		return nil, err
	}
	if version == nil {
		return allowResult(nil)
	}

	if res, err := confirmationRejectionV2(&moduleTarget{module: module, version: version}); res != nil || err != nil {
		return res, err
	}

	return allowResult(nil)
}
