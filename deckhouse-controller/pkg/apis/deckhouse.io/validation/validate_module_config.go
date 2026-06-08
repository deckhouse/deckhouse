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
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	globalModuleName = "global"

	disableReasonSuffix = "Please annotate ModuleConfig with `modules.deckhouse.io/allow-disabling=true` if you're sure that you want to disable the module."
)

// disableConfirmationReason builds a rejection message for a module that requires
// confirmation before being disabled. The reason/needConfirm pair comes from
// Module.GetConfirmationDisableReason. It returns ("", false) when no confirmation is needed.
func disableConfirmationReason(reason string, needConfirm bool) (string, bool) {
	if !needConfirm {
		return "", false
	}

	if !strings.HasSuffix(reason, ".") {
		reason += "."
	}

	return reason + " " + disableReasonSuffix, true
}

func experimentalRejectMessage(moduleName string) string {
	return fmt.Sprintf("the '%s' module is experimental, set param in 'deckhouse' ModuleConfig - spec.settings.allowExperimentalModules: true to allow it", moduleName)
}

// moduleConfigValidationHandler validates ModuleConfig admission requests.
func moduleConfigValidationHandler(
	cli client.Client,
	moduleStorage moduleStorage,
	metricStorage metricsstorage.Storage,
	moduleManager moduleManager,
	configValidator *configtools.Validator,
	setting *helpers.DeckhouseSettingsContainer,
	dependencyExtender moduleDependencyExtender,
) http.Handler {
	validator := &moduleConfigValidator{
		client:             cli,
		moduleStorage:      moduleStorage,
		metricStorage:      metricStorage,
		moduleManager:      moduleManager,
		configValidator:    configValidator,
		settings:           setting,
		dependencyExtender: dependencyExtender,
	}

	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "module-config-operations",
		Validator: kwhvalidating.ValidatorFunc(validator.validate),
		// logger is nil, because webhook has Info level for reporting about http handler
		// and we get a log of useless spam here. So we decided to use Noop logger here
		Logger: nil,
		Obj:    &v1alpha1.ModuleConfig{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: nil})
}

// moduleConfigValidator carries the dependencies needed to validate ModuleConfig
// admission requests.
type moduleConfigValidator struct {
	client             client.Client
	moduleStorage      moduleStorage
	metricStorage      metricsstorage.Storage
	moduleManager      moduleManager
	configValidator    *configtools.Validator
	settings           *helpers.DeckhouseSettingsContainer
	dependencyExtender moduleDependencyExtender
}

// validate is the admission entrypoint. Operation-specific checks run first;
// CREATE and UPDATE then fall through to the shared validateCommon checks, while
// DELETE / CONNECT / UNKNOWN are fully handled by the switch.
func (v *moduleConfigValidator) validate(ctx context.Context, review *kwhmodel.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
	cfg, ok := obj.(*v1alpha1.ModuleConfig)
	if !ok {
		return nil, fmt.Errorf("expect ModuleConfig as unstructured, got %T", obj)
	}

	allowExperimental := v.settings.Get().AllowExperimentalModules

	switch review.Operation {
	case kwhmodel.OperationDelete:
		return v.validateDelete(ctx, cfg)

	case kwhmodel.OperationConnect, kwhmodel.OperationUnknown:
		return rejectResult(fmt.Sprintf("operation '%s' is not applicable", review.Operation))

	case kwhmodel.OperationCreate:
		if res, err := v.validateCreate(ctx, cfg, allowExperimental); res != nil || err != nil {
			return res, err
		}

	case kwhmodel.OperationUpdate:
		if res, err := v.validateUpdate(ctx, review, cfg, allowExperimental); res != nil || err != nil {
			return res, err
		}
	}

	return v.validateCommon(ctx, cfg)
}

// validateDelete guards deletion: a confirmation-required module that is still
// enabled, and any module that still has a ModulePullOverride, may not be removed.
func (v *moduleConfigValidator) validateDelete(ctx context.Context, cfg *v1alpha1.ModuleConfig) (*kwhvalidating.ValidatorResult, error) {
	if !hasAllowDisableAnnotation(cfg.Annotations) && isEnabled(cfg) {
		if res, err := v.confirmationRejection(cfg.Name); res != nil || err != nil {
			return res, err
		}
	}

	exists, err := utils.ModulePullOverrideExists(ctx, v.client, cfg.Name)
	if err != nil {
		return nil, fmt.Errorf("get the '%s' module pull override: %w", cfg.Name, err)
	}
	if exists {
		return rejectResult("delete the ModulePullOverride before deleting the module config")
	}

	v.setAllowedToDisableMetric(cfg, 0)
	// if module is already disabled - we don't need to warn user about disabling module
	return allowResult(nil)
}

// validateCreate handles the CREATE operation: disabling a running module needs
// confirmation, and enabling a module runs the enabling checks.
func (v *moduleConfigValidator) validateCreate(ctx context.Context, cfg *v1alpha1.ModuleConfig, allowExperimental bool) (*kwhvalidating.ValidatorResult, error) {
	// creating a config that explicitly disables a currently enabled module
	// requires confirmation before the disable is allowed
	if !hasAllowDisableAnnotation(cfg.Annotations) && isDisabled(cfg) && v.moduleManager.IsModuleEnabled(cfg.Name) {
		if res, err := v.confirmationRejection(cfg.Name); res != nil || err != nil {
			return res, err
		}
	}

	if isEnabled(cfg) {
		// on CREATE the module must exist, so a missing Module CR is rejected
		return v.validateModuleEnabling(ctx, cfg, allowExperimental, true)
	}

	return nil, nil
}

// validateUpdate handles the UPDATE operation: a disabled->enabled transition
// runs the enabling checks, and disabling a currently enabled module needs
// confirmation.
func (v *moduleConfigValidator) validateUpdate(ctx context.Context, review *kwhmodel.AdmissionReview, cfg *v1alpha1.ModuleConfig, allowExperimental bool) (*kwhvalidating.ValidatorResult, error) {
	oldConfig, err := parseOldModuleConfig(review.OldObjectRaw)
	if err != nil {
		return nil, err
	}

	newEnabled := isEnabled(cfg)

	if !oldConfig.enabled && newEnabled {
		// on UPDATE a missing Module CR is tolerated (validateCommon handles it with a warning)
		if res, err := v.validateModuleEnabling(ctx, cfg, allowExperimental, false); res != nil || err != nil {
			return res, err
		}
	}

	// the module is being disabled when the new config does not keep it enabled
	// while it is currently enabled - either explicitly (oldConfig.enabled) or by
	// default (e.g. enabled in the bundle, but with no explicit enabled flag).
	disabling := !newEnabled && (oldConfig.enabled || v.moduleManager.IsModuleEnabled(cfg.Name))
	if disabling && !hasAllowDisableAnnotation(cfg.Annotations) && !hasAllowDisableAnnotation(oldConfig.annotations) {
		if res, err := v.confirmationRejection(cfg.Name); res != nil || err != nil {
			return res, err
		}
	}

	return nil, nil
}

// validateModuleEnabling runs the checks required before a module may be enabled:
// the experimental gate (from the downloaded module and from the Module CR) and
// the dependency constraints. rejectMissingModuleCR makes an absent Module CR a
// hard error (CREATE) instead of a tolerated one (UPDATE).
func (v *moduleConfigValidator) validateModuleEnabling(ctx context.Context, cfg *v1alpha1.ModuleConfig, allowExperimental, rejectMissingModuleCR bool) (*kwhvalidating.ValidatorResult, error) {
	if res, err := v.checkExperimentalFromStorage(cfg.Name, allowExperimental); res != nil || err != nil {
		return res, err
	}

	if err := v.dependencyExtender.CheckEnabling(cfg.Name); err != nil {
		return rejectResult(err.Error())
	}

	return v.checkExperimentalFromModuleCR(ctx, cfg.Name, allowExperimental, rejectMissingModuleCR)
}

// checkExperimentalFromStorage applies the experimental gate using the downloaded
// module definition. An unknown module (not yet downloaded) is left to the
// Module CR check.
func (v *moduleConfigValidator) checkExperimentalFromStorage(moduleName string, allowExperimental bool) (*kwhvalidating.ValidatorResult, error) {
	module, err := v.moduleStorage.GetModuleByName(moduleName)
	if err != nil {
		return nil, nil
	}

	if module.GetModuleDefinition().IsExperimental() && !allowExperimental {
		return rejectResult(experimentalRejectMessage(moduleName))
	}

	return nil, nil
}

// checkExperimentalFromModuleCR applies the experimental gate using the Module CR
// (whose properties are synced from the registry even before the module is
// downloaded). The global module has no Module CR and is skipped.
func (v *moduleConfigValidator) checkExperimentalFromModuleCR(ctx context.Context, moduleName string, allowExperimental, rejectMissing bool) (*kwhvalidating.ValidatorResult, error) {
	if moduleName == globalModuleName {
		return nil, nil
	}

	module := new(v1alpha1.Module)
	if err := v.client.Get(ctx, client.ObjectKey{Name: moduleName}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("get the '%s' module: %w", moduleName, err)
		}
		if rejectMissing {
			return rejectResult(fmt.Sprintf("the '%s' module not found", moduleName))
		}
		return nil, nil
	}

	if module.IsExperimental() && !allowExperimental {
		return rejectResult(experimentalRejectMessage(moduleName))
	}

	return nil, nil
}

// validateCommon runs the validation shared by CREATE and UPDATE: source
// resolution, update policy existence, settings validation and the
// exclusive-group conflict check. It returns an allow result with any
// accumulated warnings when nothing rejects the request.
func (v *moduleConfigValidator) validateCommon(ctx context.Context, cfg *v1alpha1.ModuleConfig) (*kwhvalidating.ValidatorResult, error) {
	if cfg.Spec.Source == v1alpha1.ModuleSourceEmbedded {
		return rejectResult("'Embedded' is a forbidden source")
	}

	warnings := make([]string, 0, 1)

	sourceResult, sourceWarnings, err := v.resolveModuleSource(ctx, cfg)
	if sourceResult != nil || err != nil {
		return sourceResult, err
	}
	warnings = append(warnings, sourceWarnings...)

	if res, err := v.validateUpdatePolicy(ctx, cfg); res != nil || err != nil {
		return res, err
	}

	// check if spec.version value is valid and the version is the latest
	if result := v.configValidator.Validate(cfg); result.HasError() {
		return rejectResult(result.Error)
	} else if result.Warning != "" {
		warnings = append(warnings, result.Warning)
	}

	v.setAllowedToDisableMetric(cfg, allowedToDisableMetricValue(cfg))

	if res, err := v.validateExclusiveGroup(cfg); res != nil || err != nil {
		return res, err
	}

	return allowResult(warnings)
}

// resolveModuleSource fetches the Module CR and validates the configured source.
// The returned result, when non-nil, is final (a missing module is allowed with
// a warning, an unavailable source is rejected). Otherwise it returns any
// warnings to accumulate. The global module has no Module CR and is skipped.
func (v *moduleConfigValidator) resolveModuleSource(ctx context.Context, cfg *v1alpha1.ModuleConfig) (*kwhvalidating.ValidatorResult, []string, error) {
	if cfg.Name == globalModuleName {
		return nil, nil, nil
	}

	module := new(v1alpha1.Module)
	if err := v.client.Get(ctx, client.ObjectKey{Name: cfg.Name}, module); err != nil {
		if apierrors.IsNotFound(err) {
			result, _ := allowResult([]string{fmt.Sprintf("the '%s' module not found", cfg.Name)})
			return result, nil, nil
		}
		return nil, nil, fmt.Errorf("get the '%s' module: %w", cfg.Name, err)
	}

	if cfg.Spec.Source != "" && !slices.Contains(module.Properties.AvailableSources, cfg.Spec.Source) {
		result, _ := rejectResult(fmt.Sprintf("the '%s' module source is an unavailable source for the '%s' module, available sources: %v", cfg.Spec.Source, cfg.Name, module.Properties.AvailableSources))
		return result, nil, nil
	}

	var warnings []string
	if isEnabled(cfg) && cfg.Spec.Source == "" && len(module.Properties.AvailableSources) > 1 {
		warnings = append(warnings, fmt.Sprintf("module '%s' is enabled but didn’t run because multiple sources were found (%s), please specify a source in ModuleConfig resource ", cfg.GetName(), strings.Join(module.Properties.AvailableSources, ", ")))
	}

	return nil, warnings, nil
}

// validateUpdatePolicy rejects the request when it references a non-existent
// ModuleUpdatePolicy. An empty policy means the module uses the embedded policy.
func (v *moduleConfigValidator) validateUpdatePolicy(ctx context.Context, cfg *v1alpha1.ModuleConfig) (*kwhvalidating.ValidatorResult, error) {
	if cfg.Spec.UpdatePolicy == "" {
		return nil, nil
	}

	policy := new(v1alpha2.ModuleUpdatePolicy)
	if err := v.client.Get(ctx, client.ObjectKey{Name: cfg.Spec.UpdatePolicy}, policy); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("get the '%s' module policy: %w", cfg.Spec.UpdatePolicy, err)
		}
		return rejectResult(fmt.Sprintf("the '%s' module policy does not exist", cfg.Spec.UpdatePolicy))
	}

	return nil, nil
}

// validateExclusiveGroup rejects enabling a module when another module from the
// same exclusive group is already enabled. An unknown module (absent from
// storage) or a disabled config has nothing to check.
func (v *moduleConfigValidator) validateExclusiveGroup(cfg *v1alpha1.ModuleConfig) (*kwhvalidating.ValidatorResult, error) {
	module, err := v.moduleStorage.GetModuleByName(cfg.Name)
	if err != nil {
		return nil, nil
	}

	if !isEnabled(cfg) {
		return nil, nil
	}

	exclusiveGroup := module.GetModuleExclusiveGroup()
	if exclusiveGroup == nil {
		return nil, nil
	}

	for _, moduleName := range v.moduleStorage.GetModulesByExclusiveGroup(*exclusiveGroup) {
		// if any module with the same exclusive group is enabled, reject
		if v.moduleManager.IsModuleEnabled(moduleName) && moduleName != cfg.Name {
			return rejectResult(fmt.Sprintf(
				"can't enable module %q because different module %q with same exclusiveGroup %s enabled",
				cfg.Name,
				moduleName,
				*exclusiveGroup,
			))
		}
	}

	return nil, nil
}

// confirmationRejection rejects the request when the module declares a disable
// confirmation requirement. Unknown modules (absent from storage) are not
// guarded. It returns (nil, nil) when there is nothing to reject.
func (v *moduleConfigValidator) confirmationRejection(moduleName string) (*kwhvalidating.ValidatorResult, error) {
	module, err := v.moduleStorage.GetModuleByName(moduleName)
	if err != nil {
		// we can disable/delete an unknown module without any further check
		return nil, nil
	}

	if reason, ok := disableConfirmationReason(module.GetConfirmationDisableReason()); ok {
		return rejectResult(reason)
	}

	return nil, nil
}

func (v *moduleConfigValidator) setAllowedToDisableMetric(cfg *v1alpha1.ModuleConfig, value float64) {
	v.metricStorage.GaugeSet(metrics.D8ModuleConfigAllowedToDisable, value, map[string]string{metrics.LabelModule: cfg.GetName()})
}

// oldModuleConfig holds the fields of the previous ModuleConfig revision that the
// UPDATE validation needs.
type oldModuleConfig struct {
	annotations map[string]string
	enabled     bool
}

func parseOldModuleConfig(raw []byte) (oldModuleConfig, error) {
	var decoded struct {
		Metadata struct {
			Annotations map[string]string `json:"annotations,omitempty"`
		} `json:"metadata,omitempty"`
		Spec struct {
			Enabled *bool `json:"enabled,omitempty"`
		} `json:"spec,omitempty"`
	}

	if err := json.Unmarshal(raw, &decoded); err != nil {
		return oldModuleConfig{}, fmt.Errorf("can not parse old module config: %w", err)
	}

	return oldModuleConfig{
		annotations: decoded.Metadata.Annotations,
		enabled:     decoded.Spec.Enabled != nil && *decoded.Spec.Enabled,
	}, nil
}

func hasAllowDisableAnnotation(annotations map[string]string) bool {
	_, ok := annotations[v1alpha1.ModuleConfigAnnotationAllowDisable]
	return ok
}

func isEnabled(cfg *v1alpha1.ModuleConfig) bool {
	return cfg.Spec.Enabled != nil && *cfg.Spec.Enabled
}

func isDisabled(cfg *v1alpha1.ModuleConfig) bool {
	return cfg.Spec.Enabled != nil && !*cfg.Spec.Enabled
}

// allowedToDisableMetricValue is 1 when the config keeps the module enabled while
// carrying the allow-disabling annotation, and 0 otherwise.
func allowedToDisableMetricValue(cfg *v1alpha1.ModuleConfig) float64 {
	if hasAllowDisableAnnotation(cfg.Annotations) && isEnabled(cfg) {
		return 1
	}
	return 0
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
