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

	"github.com/flant/addon-operator/pkg/values/validation"
	"github.com/go-openapi/spec"
	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/values/schema/cel"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/configtools"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	globalModuleName              = "global"
	controlPlaneManagerModuleName = "control-plane-manager"

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
	return fmt.Sprintf("the '%s' module is experimental; to allow it, in the 'deckhouse' ModuleConfig either set spec.settings.allowExperimentalModules: true (allows all experimental modules) or add '%s' to spec.settings.allowedExperimentalModules", moduleName, moduleName)
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
	edition *d8edition.Edition,
) http.Handler {
	validator := &moduleConfigValidator{
		client:             cli,
		moduleStorage:      moduleStorage,
		metricStorage:      metricStorage,
		moduleManager:      moduleManager,
		configValidator:    configValidator,
		settings:           setting,
		dependencyExtender: dependencyExtender,
		edition:            edition,
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
	edition            *d8edition.Edition
}

// validate is the admission entrypoint, dispatching to the handler of the
// review operation. Each handler owns its operation's whole flow.
func (v *moduleConfigValidator) validate(ctx context.Context, review *kwhmodel.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
	cfg, ok := obj.(*v1alpha1.ModuleConfig)
	if !ok {
		return nil, fmt.Errorf("expect ModuleConfig as unstructured, got %T", obj)
	}

	r := &moduleConfigReview{moduleConfigValidator: v, cfg: cfg, oldObjectRaw: review.OldObjectRaw}

	switch review.Operation {
	case kwhmodel.OperationDelete:
		return r.validateDelete(ctx)
	case kwhmodel.OperationCreate:
		return r.validateCreate(ctx)
	case kwhmodel.OperationUpdate:
		return r.validateUpdate(ctx)
	default: // CONNECT, UNKNOWN
		return rejectResult(fmt.Sprintf("operation '%s' is not applicable", review.Operation))
	}
}

// moduleConfigReview evaluates a single admission request. Every check answers
// (result, error): a non-nil result is the final verdict, otherwise the flow
// continues. Advisory warnings accumulate on the review and ride into the
// allow response.
type moduleConfigReview struct {
	*moduleConfigValidator

	cfg          *v1alpha1.ModuleConfig
	oldObjectRaw []byte
	warnings     []string
}

func (r *moduleConfigReview) warn(msg string) {
	r.warnings = append(r.warnings, msg)
}

// allow answers the request with every warning accumulated so far.
func (r *moduleConfigReview) allow() (*kwhvalidating.ValidatorResult, error) {
	return allowResult(r.warnings)
}

// validateDelete guards deletion: control-plane-manager may not drop an active
// kubernetesVersion pin, a confirmation-required module that is still enabled,
// and any module that still has a ModulePullOverride, may not be removed.
func (r *moduleConfigReview) validateDelete(ctx context.Context) (*kwhvalidating.ValidatorResult, error) {
	if r.cfg.Name == controlPlaneManagerModuleName {
		// Use raw settings (GetMap), not ExtractLatestSettings/validateCR: a conversion
		// failure on an unrelated field must not hide an existing kubernetesVersion pin.
		if res, err := r.validateControlPlaneManagerKubernetesVersion(ctx, nil, rawModuleConfigSettings(r.cfg)); res != nil || err != nil {
			return res, err
		}
	}

	defaultEnabled := r.isModuleEnabledByBundle(r.cfg.Name)
	if !hasAllowDisableAnnotation(r.cfg.Annotations) && isEnabled(r.cfg, defaultEnabled) {
		if res, err := r.confirmationRejection(r.cfg.Name); res != nil || err != nil {
			return res, err
		}
	}

	exists, err := utils.ModulePullOverrideExists(ctx, r.client, r.cfg.Name)
	if err != nil {
		return nil, fmt.Errorf("get the '%s' module pull override: %w", r.cfg.Name, err)
	}
	if exists {
		return rejectResult("delete the ModulePullOverride before deleting the module config")
	}

	r.setAllowedToDisableMetric(r.cfg, 0)
	// if module is already disabled - we don't need to warn user about disabling module
	return r.allow()
}

// validateCreate handles the CREATE operation: disabling a running module needs
// confirmation, enabling a module runs the enabling checks, and the shared
// validateCommon checks finish the flow.
func (r *moduleConfigReview) validateCreate(ctx context.Context) (*kwhvalidating.ValidatorResult, error) {
	// creating a config that explicitly disables a currently enabled module
	// requires confirmation before the disable is allowed
	if !hasAllowDisableAnnotation(r.cfg.Annotations) && isDisabled(r.cfg) && r.moduleManager.IsModuleEnabled(r.cfg.Name) {
		if res, err := r.confirmationRejection(r.cfg.Name); res != nil || err != nil {
			return res, err
		}
	}

	defaultEnabled := r.isModuleEnabledByBundle(r.cfg.Name)
	if isEnabled(r.cfg, defaultEnabled) {
		// on CREATE the module must be known, so a fully unknown one is rejected
		if res, err := r.validateModuleEnabling(ctx, true); res != nil || err != nil {
			return res, err
		}
	}

	return r.validateCommon(ctx, nil, nil)
}

// validateUpdate handles the UPDATE operation: a disabled->enabled transition
// runs the enabling checks, disabling a currently enabled module needs
// confirmation, and the shared validateCommon checks finish the flow with the
// previous revision's settings.
func (r *moduleConfigReview) validateUpdate(ctx context.Context) (*kwhvalidating.ValidatorResult, error) {
	oldConfig, err := parseOldModuleConfig(r.oldObjectRaw)
	if err != nil {
		return nil, err
	}

	oldEnabled := oldConfig.enabled || r.moduleManager.IsModuleEnabled(r.cfg.Name)

	// ModuleConfig may not have the spec.enabled field at all. In that case the
	// module's effective state does not come from this config but falls back to
	// whatever is enabled by default for the current edition/bundle.
	newEnabled := isEnabled(r.cfg, r.isModuleEnabledByBundle(r.cfg.Name))

	if !oldEnabled && newEnabled {
		// on UPDATE an unknown module is tolerated (validateCommon handles it with a warning)
		if res, err := r.validateModuleEnabling(ctx, false); res != nil || err != nil {
			return res, err
		}
	}

	// the module is being disabled when the new config does not keep it enabled
	// while it is currently enabled - either explicitly (oldConfig.enabled) or by
	// default (e.g. enabled in the bundle, but with no explicit enabled flag).
	disabling := oldEnabled && !newEnabled
	if disabling && !hasAllowDisableAnnotation(r.cfg.Annotations) && !hasAllowDisableAnnotation(oldConfig.annotations) {
		if res, err := r.confirmationRejection(r.cfg.Name); res != nil || err != nil {
			return res, err
		}
	}

	oldSettings, extractErr := r.extractOldSettings(r.oldObjectRaw)

	var oldSettingsForKubernetesVersionGuard map[string]interface{}
	if extractErr != nil {
		// Explicitly clear: CEL transition must not see partial/unconverted settings
		// even if a future extractSettingsFromModuleConfig starts returning them with err.
		oldSettings = nil
		// Keep the kubernetesVersion clear-guard alive when conversion of the old object
		// fails, but do not feed unconverted settings to CEL transition rules.
		oldRevision := new(v1alpha1.ModuleConfig)
		if json.Unmarshal(r.oldObjectRaw, oldRevision) == nil {
			oldSettingsForKubernetesVersionGuard = rawModuleConfigSettings(oldRevision)
		}
	} else {
		oldSettingsForKubernetesVersionGuard = oldSettings
	}

	return r.validateCommon(ctx, oldSettings, oldSettingsForKubernetesVersionGuard)
}

// isModuleEnabledByBundle reports whether the module would be enabled by
// default - i.e. with no explicit ModuleConfig spec.enabled - in the current
// Deckhouse edition and bundle. It is used to resolve the effective enabled
// state of a ModuleConfig whose spec.enabled is unset. An unknown module (not
// present in storage) or the global module (no module.yaml accessibility) is
// treated as not enabled by default.
func (v *moduleConfigValidator) isModuleEnabledByBundle(moduleName string) bool {
	if moduleName == globalModuleName {
		return false
	}
	module, err := v.moduleStorage.GetModuleByName(moduleName)
	if err != nil {
		return false
	}
	access := module.GetModuleDefinition().Accessibility
	if access == nil || len(access.Editions) == 0 {
		// embedded-modules do not declare accessibility; their default enabled state is determined by the enabled script/bundle at runtime
		return v.moduleManager.IsModuleEnabled(moduleName)
	}
	return access.IsEnabled(v.edition.Name, v.edition.Bundle)
}

// validateModuleEnabling runs the checks required before a module may be enabled:
// the experimental gate (from the downloaded module and from the package version)
// and the dependency constraints. rejectMissingModuleCR makes a fully unknown
// module a hard error (CREATE) instead of a tolerated one (UPDATE). A package
// version whose metadata is not loaded yet only warns.
func (r *moduleConfigReview) validateModuleEnabling(ctx context.Context, rejectMissingModuleCR bool) (*kwhvalidating.ValidatorResult, error) {
	allowExperimental := r.settings.ExperimentalModuleAllowed(r.cfg.Name)

	if res, err := r.checkExperimentalFromStorage(r.cfg.Name, allowExperimental); res != nil || err != nil {
		return res, err
	}

	if err := r.dependencyExtender.CheckEnabling(r.cfg.Name); err != nil {
		return rejectResult(err.Error())
	}

	// The metadata below lives on the module's package version, addressed by
	// the v2 Module spec. The global module has neither.
	if r.cfg.Name == globalModuleName {
		return nil, nil
	}

	moduleV2 := new(v1alpha2.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: r.cfg.Name}, moduleV2); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("get the '%s' module: %w", r.cfg.Name, err)
		}

		// a module not yet deployed has no v2 resource; its catalog entry
		// stands in as the existence proof, so a typo still rejects on CREATE
		known, err := r.modulePackageExists(ctx, r.cfg.Name)
		if err != nil {
			return nil, err
		}

		if !known && rejectMissingModuleCR {
			return rejectResult(fmt.Sprintf("the '%s' module not found", r.cfg.Name))
		}

		// an available module carries no metadata until its first deploy; the
		// storage gate above already covered what the disk knows
		return nil, nil
	}

	mpv := r.packageVersionOf(ctx, moduleV2)
	if mpv == nil {
		// a dev module or a version the catalog does not carry has no
		// metadata to check, the way an empty stage always passed here
		return nil, nil
	}

	if mpv.IsDraft() || mpv.Status.PackageMetadata == nil {
		r.warn(fmt.Sprintf("the '%s' module metadata is not loaded yet, the experimental check is skipped", r.cfg.Name))

		return nil, nil
	}

	if mpv.IsModuleExperimental() && !allowExperimental {
		return rejectResult(experimentalRejectMessage(r.cfg.Name))
	}

	return r.checkDependenciesFromPackageVersion(mpv)
}

// checkDependenciesFromPackageVersion enforces the "parent module must be
// enabled" part of the requirements on the module's package version, which the
// dependency extender cannot check for a module whose module.yaml has not been
// loaded from disk yet. Conditional parents and version constraints are left
// to the extender.
func (v *moduleConfigValidator) checkDependenciesFromPackageVersion(mpv *v1alpha1.ModulePackageVersion) (*kwhvalidating.ValidatorResult, error) {
	meta := mpv.Status.PackageMetadata
	if meta.Requirements == nil || meta.Requirements.Modules == nil || len(meta.Requirements.Modules.Mandatory) == 0 {
		return nil, nil
	}

	missing := make([]string, 0, len(meta.Requirements.Modules.Mandatory))
	for _, parent := range meta.Requirements.Modules.Mandatory {
		if parent.Name == mpv.Spec.PackageName {
			continue
		}
		if !v.moduleManager.IsModuleEnabled(parent.Name) {
			missing = append(missing, parent.Name)
		}
	}

	if len(missing) == 0 {
		return nil, nil
	}

	slices.Sort(missing)

	return rejectResult(fmt.Sprintf("the '%s' module depends on disabled module(s): %s", mpv.Spec.PackageName, strings.Join(missing, ", ")))
}

// packageVersionOf resolves the package version the module runs, by the spec
// triple of its v2 resource. A dev module, an incomplete triple, a version the
// catalog does not carry or a failed lookup answer nil: the metadata gates
// fail open.
func (v *moduleConfigValidator) packageVersionOf(ctx context.Context, moduleV2 *v1alpha2.Module) *v1alpha1.ModulePackageVersion {
	if moduleV2.IsDev() || moduleV2.Spec.PackageRepositoryName == "" || moduleV2.Spec.PackageVersion == "" {
		return nil
	}

	name := v1alpha1.MakeModulePackageVersionName(moduleV2.Spec.PackageRepositoryName, moduleV2.Name, moduleV2.Spec.PackageVersion)

	mpv := new(v1alpha1.ModulePackageVersion)
	if err := v.client.Get(ctx, client.ObjectKey{Name: name}, mpv); err != nil {
		return nil
	}

	return mpv
}

// modulePackageExists reports whether the catalog names the module.
func (v *moduleConfigValidator) modulePackageExists(ctx context.Context, moduleName string) (bool, error) {
	if err := v.client.Get(ctx, client.ObjectKey{Name: moduleName}, new(v1alpha1.ModulePackage)); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("get the '%s' module package: %w", moduleName, err)
	}

	return true, nil
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

// validateCommon runs the validation shared by CREATE and UPDATE: source
// resolution, update policy existence, settings validation and the
// exclusive-group conflict check. It answers allow with the accumulated
// warnings when nothing rejects the request.
func (r *moduleConfigReview) validateCommon(
	ctx context.Context,
	oldSettings map[string]interface{},
	oldSettingsForKubernetesVersionGuard map[string]interface{},
) (*kwhvalidating.ValidatorResult, error) {
	if r.cfg.Spec.Source == v1alpha1.ModuleSourceEmbedded {
		return rejectResult("'Embedded' is a forbidden source")
	}

	// check if spec.version value is valid and the version is the latest
	result := r.configValidator.Validate(r.cfg)

	// The kubernetesVersion guard runs before resolveModuleSource on purpose. That call returns a
	// non-nil *allow* result when the module's catalog entry is missing (fresh install, or the
	// window while the catalog is rebuilt), which returns from validateCommon before anything below
	// runs — so a guard placed after it can be bypassed by deleting the catalog entry and then
	// applying an out-of-window pin. The DELETE path already runs the guard first for this reason.
	//
	// Scoped to control-plane-manager so ordering for every other module is untouched: a
	// ModuleConfig for a not-yet-installed module must keep being allowed with a warning.
	if r.cfg.Name == controlPlaneManagerModuleName {
		if result.HasError() {
			return rejectResult(result.Error)
		}
		if res, err := r.validateControlPlaneManagerKubernetesVersion(ctx, result.Settings, oldSettingsForKubernetesVersionGuard); res != nil || err != nil {
			return res, err
		}
	}

	if res, err := r.resolveModuleSource(ctx); res != nil || err != nil {
		return res, err
	}

	if res, err := r.validateUpdatePolicy(ctx, r.cfg); res != nil || err != nil {
		return res, err
	}

	if result.HasError() {
		return rejectResult(result.Error)
	}
	if result.Warning != "" {
		r.warn(result.Warning)
	}

	r.setAllowedToDisableMetric(r.cfg, allowedToDisableMetricValue(r.cfg, r.isModuleEnabledByBundle(r.cfg.Name)))

	// CEL transition rules (x-deckhouse-validations with oldSelf).
	// Executed only on UPDATE (oldSettings != nil).
	// On CREATE oldSettings == nil → this block is skipped.
	if oldSettings != nil {
		if res, err := r.validateCELTransition(r.cfg, result.Settings, oldSettings); res != nil || err != nil {
			return res, err
		}
	}

	if res, err := r.validateExclusiveGroup(r.cfg); res != nil || err != nil {
		return res, err
	}

	return r.allow()
}

// extractOldSettings parses the OldObjectRaw from the AdmissionReview and returns the settings in the latest-version form.
// Returns nil, nil if the old object has no settings or version.
// If an error occurs, the transition rules are simply skipped.
func (v *moduleConfigValidator) extractOldSettings(oldObjectRaw []byte) (map[string]interface{}, error) {
	if len(oldObjectRaw) == 0 {
		return nil, nil
	}

	oldConfig := new(v1alpha1.ModuleConfig)
	if err := json.Unmarshal(oldObjectRaw, oldConfig); err != nil {
		return nil, fmt.Errorf("unmarshal old ModuleConfig: %w", err)
	}

	return v.extractSettingsFromModuleConfig(oldConfig)
}

// extractSettingsFromModuleConfig returns settings in the latest-version form, or nil when
// the object has no version/settings (same skip semantics as extractOldSettings).
func (v *moduleConfigValidator) extractSettingsFromModuleConfig(cfg *v1alpha1.ModuleConfig) (map[string]interface{}, error) {
	if cfg == nil || cfg.Spec.Version == 0 || cfg.Spec.Settings == nil {
		return nil, nil
	}

	settings, err := v.configValidator.ExtractLatestSettings(cfg)
	if err != nil {
		return nil, fmt.Errorf("extract settings: %w", err)
	}

	return settings, nil
}

// validateCELTransition runs x-deckhouse-validations CEL transition rules—those that reference oldSelf. Called only on UPDATE (oldSettings != nil).
// Uses cel.ValidateTransition from the internal deckhouse-controller package.
func (v *moduleConfigValidator) validateCELTransition(cfg *v1alpha1.ModuleConfig, newSettings, oldSettings map[string]interface{}) (*kwhvalidating.ValidatorResult, error) {
	if newSettings == nil {
		return nil, nil
	}

	// Get spec.Schema from addon-operator SchemaStorage.
	addonSchema := v.configSchema(cfg.GetName())
	if addonSchema == nil {
		return nil, nil
	}

	errs, celErr := cel.ValidateTransition(addonSchema, newSettings, oldSettings)
	if celErr != nil {
		return rejectResult(fmt.Sprintf("cel transition validation: %v", celErr))
	}
	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return rejectResult(fmt.Sprintf(
			"spec.settings are not valid (version %d): %s",
			cfg.Spec.Version,
			strings.Join(msgs, "; "),
		))
	}
	return nil, nil
}

// configSchema returns the spec.Schema for the module's config values.
// Chain: v.moduleStorage.GetModuleByName → GetBasicModule → GetSchemaStorage → Schemas[ConfigValuesSchema]
// The addon-operator uses the same schema in ValidateConfigValues().
func (v *moduleConfigValidator) configSchema(moduleName string) *spec.Schema {
	mod, err := v.moduleStorage.GetModuleByName(moduleName)
	if err != nil {
		return nil
	}

	basic := mod.GetBasicModule()
	if basic == nil {
		return nil
	}

	ss := basic.GetSchemaStorage()
	if ss == nil {
		return nil
	}

	// validation.ConfigValuesSchema - constant from addon-operator pkg/values/validation
	return ss.Schemas[validation.ConfigValuesSchema]
}

// resolveModuleSource validates the configured source against the module's
// catalog entry. The returned result, when non-nil, is final: a module the
// catalog does not know is allowed with a warning, an unavailable source is
// rejected. The global module has no catalog entry and is skipped.
func (r *moduleConfigReview) resolveModuleSource(ctx context.Context) (*kwhvalidating.ValidatorResult, error) {
	if r.cfg.Name == globalModuleName {
		return nil, nil
	}

	pkg := new(v1alpha1.ModulePackage)
	if err := r.client.Get(ctx, client.ObjectKey{Name: r.cfg.Name}, pkg); err != nil {
		if apierrors.IsNotFound(err) {
			r.warn(fmt.Sprintf("the '%s' module not found", r.cfg.Name))

			return r.allow()
		}
		return nil, fmt.Errorf("get the '%s' module package: %w", r.cfg.Name, err)
	}

	repositories := pkg.Status.AvailableRepositories

	if r.cfg.Spec.Source != "" && !slices.Contains(repositories, v1alpha1.PackageRepositoryNameForModuleSource(r.cfg.Spec.Source)) {
		// an unscanned repository proves nothing about its source: reject only
		// what the scanned world positively contradicts
		scanned, err := r.packageRepositoryExists(ctx, v1alpha1.PackageRepositoryNameForModuleSource(r.cfg.Spec.Source))
		if err != nil {
			return nil, err
		}

		// an unknown source availability skips only this check: the module is
		// known, so the settings and the other checks below still apply
		if !scanned {
			r.warn(fmt.Sprintf("the '%s' source of the '%s' module has no repository to validate against yet", r.cfg.Spec.Source, r.cfg.Name))

			return nil, nil
		}

		return rejectResult(fmt.Sprintf("the '%s' module source is an unavailable source for the '%s' module, available sources: %v", r.cfg.Spec.Source, r.cfg.Name, sourceNames(repositories)))
	}

	if isEnabled(r.cfg, r.isModuleEnabledByBundle(r.cfg.Name)) && r.cfg.Spec.Source == "" && len(repositories) > 1 {
		r.warn(fmt.Sprintf("module '%s' is enabled but didn’t run because multiple sources were found (%s), please specify a source in ModuleConfig resource ", r.cfg.GetName(), strings.Join(sourceNames(repositories), ", ")))
	}

	return nil, nil
}

// packageRepositoryExists reports whether the named repository is part of the
// scanned world.
func (v *moduleConfigValidator) packageRepositoryExists(ctx context.Context, name string) (bool, error) {
	if err := v.client.Get(ctx, client.ObjectKey{Name: name}, new(v1alpha1.PackageRepository)); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("get the '%s' package repository: %w", name, err)
	}

	return true, nil
}

// sourceNames maps repository names back to the module source vocabulary the
// config and its user speak.
func sourceNames(repositories []string) []string {
	sources := make([]string, 0, len(repositories))
	for _, repository := range repositories {
		sources = append(sources, v1alpha1.ModuleSourceNameForPackageRepository(repository))
	}

	return sources
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

	if !isEnabled(cfg, v.isModuleEnabledByBundle(cfg.Name)) {
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

func isEnabled(cfg *v1alpha1.ModuleConfig, defaultEnabled bool) bool {
	if cfg.Spec.Enabled == nil {
		return defaultEnabled
	}
	return cfg.Spec.Enabled != nil && *cfg.Spec.Enabled
}

func isDisabled(cfg *v1alpha1.ModuleConfig) bool {
	return cfg.Spec.Enabled != nil && !*cfg.Spec.Enabled
}

// allowedToDisableMetricValue is 1 when the config keeps the module enabled while
// carrying the allow-disabling annotation, and 0 otherwise.
func allowedToDisableMetricValue(cfg *v1alpha1.ModuleConfig, defaultEnabled bool) float64 {
	if hasAllowDisableAnnotation(cfg.Annotations) && isEnabled(cfg, defaultEnabled) {
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
