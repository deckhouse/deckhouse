/*
Copyright 2025 Flant JSC

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
	"reflect"
	"strings"

	"github.com/flant/addon-operator/pkg/values/validation"
	"github.com/go-openapi/spec"
	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/gate"
	packageschema "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/values/schema"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
)

// maxApplicationNameLength limits the instance name: it prefixes the name of every object
// the application creates, and those must fit the 63-character Kubernetes name limit.
const maxApplicationNameLength = 24

// applicationValidationHandler validates Application create and update requests.
func applicationValidationHandler(cli client.Client, manager packageManager) http.Handler {
	vf := kwhvalidating.ValidatorFunc(func(ctx context.Context, review *kwhmodel.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
		app, ok := obj.(*v1alpha1.Application)
		if !ok {
			return nil, fmt.Errorf("expect Application as unstructured, got %T", obj)
		}

		// no sense to check already deleted app
		if !app.DeletionTimestamp.IsZero() {
			return allowResult(nil)
		}

		// The previously stored object is what immutable settings fields are frozen
		// against. It is only present on UPDATE; on CREATE every field is still free.
		oldApp, err := extractOldApplication(review)
		if err != nil {
			return nil, err
		}

		if len(app.Name) > maxApplicationNameLength {
			return rejectResult(fmt.Sprintf("Application name '%s' must be no longer than %d characters", app.Name, maxApplicationNameLength))
		}

		ap := new(v1alpha1.ApplicationPackage)
		if err := cli.Get(ctx, client.ObjectKey{Name: app.Spec.PackageName}, ap); err != nil {
			return rejectResult(fmt.Sprintf("get application package: %v", err))
		}

		adm := newAdmission(app, oldApp)

		warnings, err := validateAppAgainstApv(ctx, cli, manager, adm)
		if err != nil {
			// The denial message is the only feedback `kubectl apply` prints, and
			// it arrives without the object it belongs to: name the Application
			// and the package version whose requirements were evaluated, so the
			// rejection is traceable when several manifests are applied at once.
			return rejectResult(fmt.Sprintf("Application '%s/%s' (package '%s' version '%s'): %s",
				app.Namespace, app.Name, app.Spec.PackageName, adm.version, err))
		}

		// The running version is checked by its object, which converts older settings with the
		// same chain; the settings version is known to the chain by now.
		if adm.installed {
			res, err := manager.ValidatePackageSettings(ctx, apps.BuildName(app.Namespace, app.Name), app.Spec.SettingsVersion, app.Spec.Settings.GetMap())
			if err != nil {
				return nil, err
			}

			if !res.Valid {
				return rejectResult(res.Message)
			}

			warnings = append(warnings, res.Warnings...)
		}

		return allowResult(warnings)
	})

	// Create webhook.
	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "application-operations",
		Validator: vf,
		// logger is nil, because webhook has Info level for reporting about http handler
		// and we get a log of useless spam here. So we decided to use Noop logger here
		Logger: nil,
		Obj:    &v1alpha1.Application{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: nil})
}

// admission is what a request is validated against, decided once per request.
type admission struct {
	app *v1alpha1.Application
	// oldApp is the stored object the request replaces; nil on CREATE.
	oldApp *v1alpha1.Application
	// version is spec.packageVersion on a create and on a spec version change, the running version on any other
	// update: a settings edit reaches the running version, which the automatic update may have moved past spec.
	version string
	// installed reports whether version runs, so its object validates the settings instead of the schema.
	installed bool
	// versionChanged reports a create or a spec version change, which gates the step and the requirements.
	versionChanged bool
}

// newAdmission decides what the request is validated against.
func newAdmission(app, oldApp *v1alpha1.Application) admission {
	adm := admission{
		app:            app,
		oldApp:         oldApp,
		version:        app.Spec.PackageVersion,
		versionChanged: oldApp == nil || specVersionChanged(app, oldApp),
	}

	running := app.Status.CurrentVersion
	if !adm.versionChanged && running != nil && running.Version != "" {
		adm.version = running.Version
	}

	adm.installed = running != nil && running.Version == adm.version

	return adm
}

// validateAppAgainstApv validates the request against the package version it admits: the settings, and on a create
// or a version change the step and the requirements. The warnings tell about settings of an older settings version.
func validateAppAgainstApv(ctx context.Context, cli client.Client, manager packageManager, adm admission) ([]string, error) {
	app := adm.app
	apvName := v1alpha1.MakeApplicationPackageVersionName(app.Spec.PackageRepositoryName, app.Spec.PackageName, adm.version)

	apv := new(v1alpha1.ApplicationPackageVersion)
	if err := cli.Get(ctx, client.ObjectKey{Name: apvName}, apv); err != nil {
		return nil, fmt.Errorf("get application package version: %w", err)
	}

	if apv.IsDraft() {
		return nil, fmt.Errorf("application package version '%s' is draft", apvName)
	}

	warnings, err := validateAppSettings(ctx, cli, apv, adm)
	if err != nil {
		return nil, fmt.Errorf("validate settings: %w", err)
	}

	// Unmet requirements of a version admitted earlier are the engine's (the Candidate is Blocked), and a
	// settings-only edit must still reach the running version, as the Module webhook checks them only on enabling.
	if !adm.versionChanged {
		return warnings, nil
	}

	// Only a spec version change is gated, so the controller's own patches never trip it; nothing is installed on create.
	if adm.oldApp != nil && adm.oldApp.Spec.PackageVersion != app.Spec.PackageVersion {
		if err := checkVersionCompatibility(ctx, cli, app, adm.oldApp); err != nil {
			return nil, err
		}
	}

	metadata := apv.Status.PackageMetadata
	if metadata == nil {
		metadata = new(v1alpha1.ApplicationPackageVersionStatusMetadata)
	}

	constraints, err := metadata.Requirements.ToScheduleConstraints()
	if err != nil {
		return nil, err
	}

	// The name is the scheduler-side identifier (namespace.name) used by the cycle simulation;
	// admission shows the scheduler's message, without the reason prefix.
	if err := manager.CheckConstraints(apps.BuildName(app.Namespace, app.Name), constraints); err != nil {
		return nil, errors.New(gate.ConstraintMessage(err))
	}

	return warnings, nil
}

// specVersionChanged reports whether the update names another package version in spec (repository, package or version).
func specVersionChanged(app, oldApp *v1alpha1.Application) bool {
	return v1alpha1.MakeApplicationPackageVersionName(oldApp.Spec.PackageRepositoryName, oldApp.Spec.PackageName, oldApp.Spec.PackageVersion) !=
		v1alpha1.MakeApplicationPackageVersionName(app.Spec.PackageRepositoryName, app.Spec.PackageName, app.Spec.PackageVersion)
}

// checkVersionCompatibility gates a change of the spec version by the rules of every version the engine may leave
// for it, in either direction.
func checkVersionCompatibility(ctx context.Context, cli client.Client, app, oldApp *v1alpha1.Application) error {
	wanted := app.Spec.PackageVersion

	for _, current := range leftVersions(app, oldApp) {
		rules, err := previousCompatibility(ctx, cli, oldApp, current)
		if err != nil {
			return err
		}

		if !rules.IsCompatible(current, wanted) {
			return fmt.Errorf("incompatible version: current=%s target=%s", current, wanted)
		}
	}

	return nil
}

// leftVersions returns the versions the engine may move from: the running one and the one it was heading to,
// which may be activated before the status shows it.
func leftVersions(app, oldApp *v1alpha1.Application) []string {
	heading := oldApp.TargetVersion()

	running := app.Status.CurrentVersion
	if running == nil || running.Version == "" || running.Version == heading {
		return []string{heading}
	}

	return []string{running.Version, heading}
}

// previousCompatibility reads the rules of a version the request leaves, from the repository the previous spec
// named; a vanished version yields nil, so the defaults apply.
func previousCompatibility(ctx context.Context, cli client.Client, oldApp *v1alpha1.Application, previous string) (*v1alpha1.PackageVersionCompatibilityRules, error) {
	apv, err := findAPV(ctx, cli, oldApp.Spec.PackageRepositoryName, oldApp.Spec.PackageName, previous)
	if err != nil {
		return nil, fmt.Errorf("get previous application package version: %w", err)
	}

	if apv == nil || apv.Status.PackageMetadata == nil {
		return nil, nil
	}

	return apv.Status.PackageMetadata.Compatibility, nil
}

// findAPV returns the package version of the repository, package and version; nil when it is gone.
func findAPV(ctx context.Context, cli client.Client, repo, pkg, version string) (*v1alpha1.ApplicationPackageVersion, error) {
	apv := new(v1alpha1.ApplicationPackageVersion)
	if err := cli.Get(ctx, client.ObjectKey{Name: v1alpha1.MakeApplicationPackageVersionName(repo, pkg, version)}, apv); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return apv, nil
}

// validateAppSettings converts spec.settings from spec.settingsVersion to the latest settings version of the package
// version, rejecting a version its chain does not know; validates them against its settings schema unless the version
// is installed (its object checks them); and on UPDATE enforces x-deckhouse-immutable. Without a settings schema
// there is nothing to validate, so packages that ship none stay installable.
func validateAppSettings(ctx context.Context, cli client.Client, apv *v1alpha1.ApplicationPackageVersion, adm admission) ([]string, error) {
	app := adm.app

	converter, err := settingsConverter(apv.Status.PackageSchemas)
	if err != nil {
		return nil, err
	}

	settings, err := convertSettingsToLatest(converter, app.Spec.SettingsVersion, app.Spec.Settings.GetMap())
	if err != nil {
		return nil, err
	}

	warnings := obsoleteSettingsWarnings("spec.settingsVersion", app.Spec.SettingsVersion, app.Spec.Settings.GetMap(), converter)

	storage, err := settingsSchemaStorage(apv.Status.PackageSchemas)
	if err != nil || storage == nil {
		return warnings, err
	}

	if !adm.installed {
		values := addonutils.Values{app.Spec.PackageName: settings}
		if err = storage.ValidateConfigValues(app.Spec.PackageName, values); err != nil {
			return nil, err
		}
	}

	if err = checkImmutableSettings(ctx, cli, storage.Schemas[validation.ConfigValuesSchema], converter, settings, app, adm.oldApp); err != nil {
		return nil, err
	}

	return warnings, nil
}

// obsoleteSettingsWarnings tells about settings written for an older settings version than the
// latest of converter, or without a version (read as version 1); none for empty settings.
func obsoleteSettingsWarnings(field string, version int, settings map[string]any, converter *conversion.Converter) []string {
	if len(settings) == 0 {
		return nil
	}

	latest := latestSettingsVersion(converter)
	switch {
	case version == 0:
		return []string{fmt.Sprintf("%s is not set, spec.settings are read as version 1: set it, the latest version is %d", field, latest)}
	case version < latest:
		return []string{fmt.Sprintf("%s=%d is obsolete. Please migrate spec.settings to the latest version %d", field, version, latest)}
	default:
		return nil
	}
}

// settingsConverter builds the converter of the settings conversions a package version
// publishes; nil when it publishes none, its latest settings version being 1.
func settingsConverter(schemas *v1alpha1.PackageVersionStatusSchemas) (*conversion.Converter, error) {
	if schemas == nil || len(schemas.SettingsConversions) == 0 {
		return nil, nil
	}

	conversions := make(map[int][]string, len(schemas.SettingsConversions))
	for _, c := range schemas.SettingsConversions {
		conversions[c.Version] = c.Expr
	}

	converter, err := conversion.NewConverter(conversions)
	if err != nil {
		return nil, fmt.Errorf("build the settings converter: %w", err)
	}

	return converter, nil
}

// latestSettingsVersion is the latest settings version of converter's chain; 1 without one.
func latestSettingsVersion(converter *conversion.Converter) int {
	if converter == nil {
		return 1
	}

	return converter.LatestVersion()
}

// convertSettingsToLatest converts settings written for version (0 reads as 1) to the latest
// settings version of converter, rejecting a version the chain does not know.
func convertSettingsToLatest(converter *conversion.Converter, version int, settings map[string]any) (map[string]any, error) {
	if version == 0 {
		version = 1
	}

	latest := latestSettingsVersion(converter)
	if version < 1 || version > latest {
		return nil, fmt.Errorf("settings version %d is not supported, the latest is %d", version, latest)
	}

	return convertSettings(converter, version, latest, settings)
}

// convertSettings converts settings from version to the later version to; the same version is a no-op.
func convertSettings(converter *conversion.Converter, from, to int, settings map[string]any) (map[string]any, error) {
	if converter == nil || from == to {
		return settings, nil
	}

	// the converter skips a nil document, and an absent spec.settings is exactly
	// the case the chain has to fill in
	if settings == nil {
		settings = make(map[string]any)
	}

	_, converted, err := converter.ConvertTo(from, to, settings)
	if err != nil {
		return nil, fmt.Errorf("convert settings from version %d: %w", from, err)
	}

	return converted, nil
}

// settingsSchemaStorage builds a schema storage from a package version's settings schema; nil when it publishes none.
func settingsSchemaStorage(schemas *v1alpha1.PackageVersionStatusSchemas) (*validation.SchemaStorage, error) {
	if schemas == nil || schemas.SettingsSchema == nil || schemas.SettingsSchema.OpenAPIV3Schema == nil {
		return nil, nil
	}

	rawSchema, err := json.Marshal(schemas.SettingsSchema.OpenAPIV3Schema)
	if err != nil {
		return nil, fmt.Errorf("get settings schema: %w", err)
	}

	storage, err := validation.NewSchemaStorage(rawSchema, nil)
	if err != nil {
		return nil, fmt.Errorf("create storage schema: %w", err)
	}

	return storage, nil
}

// extractOldApplication decodes the stored object the admission request replaces.
// Returns nil on CREATE, where there is nothing an immutable field could deviate from.
func extractOldApplication(review *kwhmodel.AdmissionReview) (*v1alpha1.Application, error) {
	if review == nil || review.Operation != kwhmodel.OperationUpdate {
		return nil, nil
	}

	// An UPDATE always carries the object it replaces. Failing the request is the same
	// safe direction the decode error below takes: admitting an update whose old object
	// never arrived would pass every immutable field unchecked.
	if len(review.OldObjectRaw) == 0 {
		return nil, errors.New("update review carries no old Application")
	}

	oldApp := new(v1alpha1.Application)
	if err := json.Unmarshal(review.OldObjectRaw, oldApp); err != nil {
		// Failing the request is the safe direction: silently skipping the check would
		// turn a decoding glitch into a way past immutability.
		return nil, fmt.Errorf("unmarshal old Application: %w", err)
	}

	return oldApp, nil
}

// checkImmutableSettings rejects an update that changes a settings field marked
// x-deckhouse-immutable. Skipped when there is no previous object to compare against.
// newSettings are the update's settings converted to the latest version of converter,
// the target package version's chain.
func checkImmutableSettings(ctx context.Context, cli client.Client, settingsSchema *spec.Schema, converter *conversion.Converter, newSettings map[string]any, app, oldApp *v1alpha1.Application) error {
	if oldApp == nil || settingsSchema == nil {
		return nil
	}

	// Nothing applied, nothing frozen: an install whose first run failed (an immutable field
	// with a typo) is fixed in place. status.currentVersion is no signal: it is written at
	// activation, before the first run.
	if !hasRun(oldApp.Status.LastAppliedConfiguration) {
		return nil
	}

	oldSettings, err := effectiveSettings(oldApp)
	if err != nil {
		return err
	}

	// The effective settings are in the latest version of the package that applied them,
	// which a version change may have moved past or behind: bring them to the target's
	// latest. A later version cannot be converted back, so after a downgrade of the
	// settings schema there is nothing to compare like with like.
	latest := latestSettingsVersion(converter)
	applied, err := appliedSettingsVersion(ctx, cli, app, oldApp, latest)
	if err != nil {
		return err
	}

	if applied > latest {
		return nil
	}

	if oldSettings, err = convertSettings(converter, applied, latest, oldSettings); err != nil {
		return fmt.Errorf("convert the last applied settings: %w", err)
	}

	// The manifest the user last wrote is compared at the latest version too; one the chain
	// cannot convert is taken as written.
	oldManifest := oldApp.Spec.Settings.GetMap()
	if converted, err := convertSettingsToLatest(converter, oldApp.Spec.SettingsVersion, oldManifest); err == nil {
		oldManifest = converted
	}

	// Defaults go on both sides so that a key left out of the manifest compares equal
	// to the value the application actually runs with — otherwise dropping the key
	// would read as "never set" and slip past the check. ApplyDefaults mutates in
	// place, which is safe here because both maps are freshly decoded or converted.
	validation.ApplyDefaults(oldSettings, settingsSchema)
	validation.ApplyDefaults(newSettings, settingsSchema)
	dropUnsetGrants(settingsSchema, oldSettings, newSettings, oldManifest)

	errs := packageschema.CheckImmutable(settingsSchema, oldSettings, newSettings)
	if len(errs) == 0 {
		return nil
	}

	msgs := make([]string, 0, len(errs))
	for _, err := range errs {
		msgs = append(msgs, err.Error())
	}

	return errors.New(strings.Join(msgs, "; "))
}

// appliedSettingsVersion is the settings version status.lastAppliedConfiguration is in: the latest
// version of the package version that applied it, status.currentVersion. targetLatest stands for
// the target package version itself and for a current version whose package version is gone.
func appliedSettingsVersion(ctx context.Context, cli client.Client, app, oldApp *v1alpha1.Application, targetLatest int) (int, error) {
	current := oldApp.Status.CurrentVersion
	if current == nil || current.Version == "" || current.Version == app.Spec.PackageVersion {
		return targetLatest, nil
	}

	// the current version came from the repository the previous spec named
	apv, err := findAPV(ctx, cli, oldApp.Spec.PackageRepositoryName, oldApp.Spec.PackageName, current.Version)
	if err != nil {
		return 0, fmt.Errorf("get current application package version: %w", err)
	}

	if apv == nil {
		return targetLatest, nil
	}

	converter, err := settingsConverter(apv.Status.PackageSchemas)
	if err != nil {
		return 0, fmt.Errorf("current application package version: %w", err)
	}

	return latestSettingsVersion(converter), nil
}

// effectiveSettings returns the settings the application actually runs with, which is
// what an immutable field is frozen at: status.lastAppliedConfiguration, written by the
// controller after every successful apply with the schema defaults and the project's
// grant defaults already merged in. Only an application that has run (hasRun) reaches here.
func effectiveSettings(app *v1alpha1.Application) (map[string]any, error) {
	settings := make(map[string]any)
	if err := json.Unmarshal(app.Status.LastAppliedConfiguration.Raw, &settings); err != nil {
		// Same direction as a failed decode of the old object: this is the value
		// immutability is measured against, so guessing at it is not an option.
		return nil, fmt.Errorf("unmarshal last applied configuration: %w", err)
	}

	return settings, nil
}

// dropUnsetGrants removes from oldSettings every grantable field that neither manifest
// spells out. The controller fills those from the project, so the effective old side
// holds a value the manifest never named: compared as-is, an edit to an unrelated field
// would read as a change to a field nobody touched. oldManifest is spec.settings as the
// user last wrote it, undefaulted, which is the only place an explicit choice shows.
func dropUnsetGrants(settingsSchema *spec.Schema, oldSettings, newSettings, oldManifest map[string]any) {
	refs, err := packageschema.CollectGrantRefs(settingsSchema)
	if err != nil {
		return
	}

	for _, ref := range refs {
		// Absent and empty are one case: the grant transformer fills both.
		newValue := valueAtPath(newSettings, ref.Path)
		if newValue != nil && newValue != "" {
			continue
		}

		// What the stored settings hold here was resolved by the controller, not chosen
		// by the user, so it may be dropped only while the manifest keeps saying what it
		// said before. An empty string is a choice like any other: it cancels out against
		// an empty string, never against a value the update is deleting.
		if !reflect.DeepEqual(valueAtPath(oldManifest, ref.Path), newValue) {
			continue
		}

		deleteAtPath(oldSettings, ref.Path)
	}
}

// valueAtPath returns the value at a property path, or nil when any segment is missing.
func valueAtPath(settings map[string]any, path []string) any {
	var value any = settings
	for _, segment := range path {
		parent, ok := value.(map[string]any)
		if !ok {
			return nil
		}

		value = parent[segment]
	}

	return value
}

// deleteAtPath removes the key at a property path, leaving the objects above it in place.
func deleteAtPath(settings map[string]any, path []string) {
	if len(path) == 0 {
		return
	}

	parent, ok := valueAtPath(settings, path[:len(path)-1]).(map[string]any)
	if !ok {
		return
	}

	delete(parent, path[len(path)-1])
}
