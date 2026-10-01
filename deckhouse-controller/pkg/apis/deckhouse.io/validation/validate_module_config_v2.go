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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/flant/addon-operator/pkg/values/validation"
	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/module-sdk/pkg/settingscheck"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	packageschema "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/values/schema"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/values/schema/cel"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1beta1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

// modulePackageManager is what the Module v2 webhooks read from the package runtime.
type modulePackageManager interface {
	IsPackageEnabled(name string) bool
	GetEnabledModuleNames() []string
	ValidatePackageSettings(ctx context.Context, name string, settingsVersion int, settings addonutils.Values) (settingscheck.Result, error)
	CheckConstraints(name string, constraints schedule.Constraints) error
	ValidateModuleExclusiveGroup(group string) error
}

// packageModuleManager serves the addon-operator-shaped moduleManager from the package runtime.
type packageModuleManager struct {
	modulePackageManager
}

// IsModuleEnabled reports whether the runtime scheduler enabled the module.
func (m packageModuleManager) IsModuleEnabled(name string) bool {
	return m.IsPackageEnabled(name)
}

// moduleRequest is the part of a ModuleConfig or a Module the package version checks read.
type moduleRequest struct {
	name            string
	enabled         *bool
	settingsVersion int
	settings        *v1alpha1.MappedFields
	annotations     map[string]string
}

// moduleConfigRequest reads a ModuleConfig; nil stays nil.
func moduleConfigRequest(cfg *v1alpha1.ModuleConfig) *moduleRequest {
	if cfg == nil {
		return nil
	}

	return &moduleRequest{
		name:            cfg.Name,
		enabled:         cfg.Spec.Enabled,
		settingsVersion: cfg.Spec.Version,
		settings:        cfg.Spec.Settings,
		annotations:     cfg.Annotations,
	}
}

// enabledOr resolves the enabled intent, falling back to def when it is unset.
func (r *moduleRequest) enabledOr(def bool) bool {
	if r.enabled == nil {
		return def
	}

	return *r.enabled
}

// rawSettings returns the settings as written, nil when there are none.
func (r *moduleRequest) rawSettings() map[string]any {
	if r == nil || r.settings.IsEmpty() {
		return nil
	}

	settings := r.settings.GetMap()
	if len(settings) == 0 {
		return nil
	}

	return settings
}

// moduleTarget is the Module a request is about and the package version that Module selects.
type moduleTarget struct {
	module  *v1beta1.Module
	version *v1alpha1.ModulePackageVersion
}

// installed reports whether the runtime runs the selected version, so it can convert older settings.
func (t *moduleTarget) installed() bool {
	current := t.module.Status.CurrentVersion

	return current != nil && current.Version != "" && current.Version == t.module.Spec.PackageVersion
}

// ranBefore reports whether any version has run; one that never ran has frozen no immutable field.
func (t *moduleTarget) ranBefore() bool {
	current := t.module.Status.CurrentVersion

	return current != nil && current.Version != ""
}

// moduleValidatorV2 holds the package version checks shared by the ModuleConfig and Module webhooks.
type moduleValidatorV2 struct {
	client   client.Client
	packages modulePackageManager
	settings *helpers.DeckhouseSettingsContainer
	// the control-plane-manager guards, the update policy check and the metric read nothing else
	shared *moduleConfigValidator
}

// newModuleValidatorV2 wires the shared checks onto the package runtime.
func newModuleValidatorV2(cli client.Client, packages modulePackageManager, metricStorage metricsstorage.Storage, settings *helpers.DeckhouseSettingsContainer) *moduleValidatorV2 {
	return &moduleValidatorV2{
		client:   cli,
		packages: packages,
		settings: settings,
		shared: &moduleConfigValidator{
			client:        cli,
			moduleManager: packageModuleManager{packages},
			metricStorage: metricStorage,
		},
	}
}

// moduleConfigValidationHandlerV2 validates ModuleConfig admission requests against the package version of its Module.
func moduleConfigValidationHandlerV2(
	cli client.Client,
	packages modulePackageManager,
	metricStorage metricsstorage.Storage,
	settings *helpers.DeckhouseSettingsContainer,
) http.Handler {
	validator := &moduleConfigValidatorV2{moduleValidatorV2: newModuleValidatorV2(cli, packages, metricStorage, settings)}

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

// moduleConfigValidatorV2 validates a ModuleConfig as the settings source of its Module.
type moduleConfigValidatorV2 struct {
	*moduleValidatorV2
}

// validate is the admission entrypoint.
func (v *moduleConfigValidatorV2) validate(ctx context.Context, review *kwhmodel.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
	cfg, ok := obj.(*v1alpha1.ModuleConfig)
	if !ok {
		return nil, fmt.Errorf("expect ModuleConfig as unstructured, got %T", obj)
	}

	switch review.Operation {
	case kwhmodel.OperationDelete:
		return v.validateDelete(ctx, cfg)

	case kwhmodel.OperationConnect, kwhmodel.OperationUnknown:
		return rejectResult(fmt.Sprintf("operation '%s' is not applicable", review.Operation))
	}

	var oldCfg *v1alpha1.ModuleConfig
	if review.Operation == kwhmodel.OperationUpdate {
		oldCfg = new(v1alpha1.ModuleConfig)
		if err := decodeOldObject(review.OldObjectRaw, oldCfg); err != nil {
			return nil, err
		}
	}

	return v.validateApply(ctx, cfg, oldCfg)
}

// validateDelete runs the control-plane-manager guards, the disable confirmation and the pull override check.
func (v *moduleConfigValidatorV2) validateDelete(ctx context.Context, cfg *v1alpha1.ModuleConfig) (*kwhvalidating.ValidatorResult, error) {
	req := moduleConfigRequest(cfg)
	if res, err := v.validateDeleteGuards(ctx, req); res != nil || err != nil {
		return res, err
	}

	if !hasAllowDisableAnnotation(cfg.Annotations) && v.packages.IsPackageEnabled(cfg.Name) {
		target, _, err := v.resolveTarget(ctx, cfg.Name)
		if err != nil {
			return nil, err
		}

		if res, err := confirmationRejectionV2(target); res != nil || err != nil {
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

	v.shared.setAllowedToDisableMetric(cfg, 0)

	return allowResult(nil)
}

// validateApply handles CREATE and UPDATE; oldCfg is nil on CREATE.
func (v *moduleConfigValidatorV2) validateApply(ctx context.Context, cfg, oldCfg *v1alpha1.ModuleConfig) (*kwhvalidating.ValidatorResult, error) {
	if cfg.Spec.Source == v1alpha1.ModuleSourceEmbedded {
		return rejectResult("'Embedded' is a forbidden source")
	}

	if res, err := v.shared.validateUpdatePolicy(ctx, cfg); res != nil || err != nil {
		return res, err
	}

	req, oldReq := moduleConfigRequest(cfg), moduleConfigRequest(oldCfg)

	// the guards run before the target lookup, so deleting the Module cannot bypass them
	if res, err := v.validateApplyGuards(ctx, req, oldReq); res != nil || err != nil {
		return res, err
	}

	target, warning, err := v.resolveTarget(ctx, cfg.Name)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return allowResult([]string{warning})
	}

	warnings, res, err := v.validatePackage(ctx, req, oldReq, target)
	if res != nil || err != nil {
		return res, err
	}

	v.shared.setAllowedToDisableMetric(cfg, allowedToDisableMetricValue(cfg, v.packages.IsPackageEnabled(cfg.Name)))

	return allowResult(warnings)
}

// validateApplyGuards runs the control-plane-manager migration guards on CREATE and UPDATE.
func (v *moduleValidatorV2) validateApplyGuards(ctx context.Context, req, oldReq *moduleRequest) (*kwhvalidating.ValidatorResult, error) {
	if req.name != controlPlaneManagerModuleName {
		return nil, nil
	}

	settings, oldSettings := req.rawSettings(), oldReq.rawSettings()
	if res, err := v.shared.validateControlPlaneManagerKubernetesVersion(ctx, settings, oldSettings); res != nil || err != nil {
		return res, err
	}
	if res, err := v.shared.validateControlPlaneManagerNetwork(ctx, settings, oldSettings, req.annotations); res != nil || err != nil {
		return res, err
	}

	return v.shared.validateControlPlaneManagerClusterDomain(ctx, settings, oldSettings)
}

// validateDeleteGuards runs the control-plane-manager migration guards on DELETE.
func (v *moduleValidatorV2) validateDeleteGuards(ctx context.Context, req *moduleRequest) (*kwhvalidating.ValidatorResult, error) {
	if req.name != controlPlaneManagerModuleName {
		return nil, nil
	}

	settings := req.rawSettings()
	if res, err := v.shared.validateControlPlaneManagerKubernetesVersion(ctx, nil, settings); res != nil || err != nil {
		return res, err
	}
	if res, err := v.shared.validateControlPlaneManagerNetwork(ctx, nil, settings, req.annotations); res != nil || err != nil {
		return res, err
	}

	return v.shared.validateControlPlaneManagerClusterDomainDelete(ctx, settings)
}

// resolveTarget finds the Module and its package version; a nil target comes with the warning to admit it under.
func (v *moduleValidatorV2) resolveTarget(ctx context.Context, name string) (*moduleTarget, string, error) {
	module := new(v1beta1.Module)
	if err := v.client.Get(ctx, client.ObjectKey{Name: name}, module); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Sprintf("the '%s' module not found", name), nil
		}

		return nil, "", fmt.Errorf("get the '%s' module: %w", name, err)
	}

	version, warning, err := v.versionFor(ctx, module)
	if version == nil {
		return nil, warning, err
	}

	return &moduleTarget{module: module, version: version}, "", nil
}

// versionFor finds the package version a Module selects; a nil version comes with the reason it is missing.
func (v *moduleValidatorV2) versionFor(ctx context.Context, module *v1beta1.Module) (*v1alpha1.ModulePackageVersion, string, error) {
	// named as the module controller names it: the embedded repository keeps one version object
	versionName := fmt.Sprintf("%s-%s", module.Spec.PackageRepositoryName, module.Name)
	if !module.IsEmbedded() {
		versionName = fmt.Sprintf("%s-%s", versionName, module.Spec.PackageVersion)
	}

	version := new(v1alpha1.ModulePackageVersion)
	if err := v.client.Get(ctx, client.ObjectKey{Name: versionName}, version); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Sprintf("the '%s' module package version '%s' not found", module.Name, versionName), nil
		}

		return nil, "", fmt.Errorf("get the '%s' module package version: %w", module.Name, err)
	}

	return version, "", nil
}

// validatePackage runs the package version checks: draft, settings, then the enabling or disabling transition.
func (v *moduleValidatorV2) validatePackage(ctx context.Context, req, oldReq *moduleRequest, target *moduleTarget) ([]string, *kwhvalidating.ValidatorResult, error) {
	if target.version.IsDraft() {
		reject, err := rejectResult(fmt.Sprintf("the '%s' module package version '%s' is draft", req.name, target.version.Name))
		return nil, reject, err
	}

	warnings, res, err := v.validateSettings(ctx, req, oldReq, target)
	if res != nil || err != nil {
		return nil, res, err
	}

	currentEnabled := v.packages.IsPackageEnabled(req.name)
	oldEnabled := currentEnabled || (oldReq != nil && oldReq.enabledOr(false))
	newEnabled := req.enabledOr(currentEnabled)

	if !oldEnabled && newEnabled {
		if res, err := v.validateEnabling(req, target); res != nil || err != nil {
			return nil, res, err
		}
	}

	if oldEnabled && !newEnabled && !hasAllowDisableAnnotation(req.annotations) && (oldReq == nil || !hasAllowDisableAnnotation(oldReq.annotations)) {
		if res, err := confirmationRejectionV2(target); res != nil || err != nil {
			return nil, res, err
		}
	}

	return warnings, nil, nil
}

// validateSettings checks the settings against the package version's schema and, on UPDATE, its transition rules.
func (v *moduleValidatorV2) validateSettings(ctx context.Context, req, oldReq *moduleRequest, target *moduleTarget) ([]string, *kwhvalidating.ValidatorResult, error) {
	settings := req.settings.GetMap()

	var warnings []string
	if target.installed() {
		res, err := v.packages.ValidatePackageSettings(ctx, req.name, req.settingsVersion, settings)
		if err != nil {
			return nil, nil, fmt.Errorf("validate the '%s' module settings: %w", req.name, err)
		}
		if !res.Valid {
			reject, err := rejectResult(res.Message)
			return nil, reject, err
		}

		warnings = res.Warnings
	}

	storage, err := settingsSchemaStorage(target.version.Status.PackageSchemas)
	if err != nil {
		return nil, nil, fmt.Errorf("the '%s' module settings schema: %w", req.name, err)
	}
	if storage == nil {
		return warnings, nil, nil
	}

	// without a running version nothing converts older settings, so the latest schema is the only check
	if !target.installed() {
		if err := storage.ValidateConfigValues(req.name, addonutils.Values{req.name: settings}); err != nil {
			reject, err := rejectResult(fmt.Sprintf("spec.settings are not valid: %v", err))
			return nil, reject, err
		}
	}

	// transition rules compare like with like, so a settings version change skips them
	schema := storage.Schemas[validation.ConfigValuesSchema]
	if oldReq == nil || schema == nil || oldReq.settingsVersion != req.settingsVersion {
		return warnings, nil, nil
	}

	oldSettings := oldReq.settings.GetMap()

	errs, celErr := cel.ValidateTransition(schema, settings, oldSettings)
	if celErr != nil {
		reject, err := rejectResult(fmt.Sprintf("cel transition validation: %v", celErr))
		return nil, reject, err
	}

	if target.ranBefore() {
		validation.ApplyDefaults(oldSettings, schema)
		validation.ApplyDefaults(settings, schema)
		errs = append(errs, packageschema.CheckImmutable(schema, oldSettings, settings)...)
	}

	if len(errs) > 0 {
		reject, err := rejectResult(fmt.Sprintf("spec.settings are not valid (version %d): %s", req.settingsVersion, errors.Join(errs...)))
		return nil, reject, err
	}

	return warnings, nil, nil
}

// validateEnabling runs the checks a disabled module must pass to be enabled: experimental gate, requirements, exclusive group.
func (v *moduleValidatorV2) validateEnabling(req *moduleRequest, target *moduleTarget) (*kwhvalidating.ValidatorResult, error) {
	meta := target.version.Status.PackageMetadata
	if meta == nil {
		return nil, nil
	}

	if target.version.IsModuleExperimental() && !v.settings.ExperimentalModuleAllowed(req.name) {
		return rejectResult(experimentalRejectMessage(req.name))
	}

	constraints, err := buildPackageConstraints(meta.Requirements)
	if err != nil {
		return rejectResult(fmt.Sprintf("the '%s' module requirements: %v", req.name, err))
	}

	// the order the runtime gives the module, so the bootstrap gate applies to the same modules
	constraints.Order = schedule.Order(meta.Weight)
	if constraints.Order == 0 {
		constraints.Order = schedule.FunctionalOrder
	}

	if err := v.packages.CheckConstraints(req.name, constraints); err != nil {
		return rejectResult(fmt.Sprintf("the '%s' module requirements are not met: %v", req.name, err))
	}

	if meta.ExclusiveGroup != "" {
		if err := v.packages.ValidateModuleExclusiveGroup(meta.ExclusiveGroup); err != nil {
			return rejectResult(fmt.Sprintf("can't enable module %q: exclusiveGroup %s: %v", req.name, meta.ExclusiveGroup, err))
		}
	}

	return nil, nil
}

// confirmationRejectionV2 rejects when the package version asks for disable confirmation; an unknown target is not guarded.
func confirmationRejectionV2(target *moduleTarget) (*kwhvalidating.ValidatorResult, error) {
	if target == nil || target.version.Status.PackageMetadata == nil {
		return nil, nil
	}

	options := target.version.Status.PackageMetadata.DisableOptions
	if options == nil {
		return nil, nil
	}

	message := fmt.Sprintf("the '%s' module requires confirmation to be disabled", target.module.Name)
	if options.Messages != nil && strings.TrimSpace(options.Messages.En) != "" {
		message = strings.TrimSpace(options.Messages.En)
	}

	if reason, ok := disableConfirmationReason(message, options.Confirmation); ok {
		return rejectResult(reason)
	}

	return nil, nil
}

// decodeOldObject decodes the stored object an UPDATE replaces; failing is the safe direction.
func decodeOldObject(raw []byte, into any) error {
	if len(raw) == 0 {
		return errors.New("update review carries no old object")
	}

	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("unmarshal old object: %w", err)
	}

	return nil
}
