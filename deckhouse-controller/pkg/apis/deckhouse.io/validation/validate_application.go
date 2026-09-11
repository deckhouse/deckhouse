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
	"strings"

	"github.com/Masterminds/semver/v3"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/values/validation"
	"github.com/go-openapi/spec"
	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	packageschema "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/values/schema"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
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

		// The previously stored object is what immutable settings fields are frozen
		// against. It is only present on UPDATE; on CREATE every field is still free.
		oldApp, err := extractOldApplication(review)
		if err != nil {
			return nil, err
		}

		// no sense to check already deleted app
		if !app.DeletionTimestamp.IsZero() {
			return allowResult(nil)
		}

		if len(app.Name) > maxApplicationNameLength {
			return rejectResult(fmt.Sprintf("Application name '%s' must be no longer than %d characters", app.Name, maxApplicationNameLength))
		}

		ap := new(v1alpha1.ApplicationPackage)
		if err := cli.Get(ctx, client.ObjectKey{Name: app.Spec.PackageName}, ap); err != nil {
			return rejectResult(fmt.Sprintf("get application package: %v", err))
		}

		name := apps.BuildName(app.Namespace, app.Name)

		var warnings []string
		if app.Status.CurrentVersion != nil && app.Status.CurrentVersion.Version == app.Spec.PackageVersion {
			res, err := manager.ValidatePackageSettings(ctx, name, 0, app.Spec.Settings.GetMap())
			if err != nil {
				return nil, err
			}

			if !res.Valid {
				return rejectResult(res.Message)
			}

			warnings = res.Warnings
		}

		if err := validateAppAgainstApv(ctx, cli, manager, app, oldApp); err != nil {
			// The denial message is the only feedback `kubectl apply` prints, and
			// it arrives without the object it belongs to: name the Application
			// and the package version whose requirements were evaluated, so the
			// rejection is traceable when several manifests are applied at once.
			return rejectResult(fmt.Sprintf("Application '%s/%s' (package '%s' version '%s'): %s",
				app.Namespace, app.Name, app.Spec.PackageName, app.Spec.PackageVersion, err))
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

// validateAppAgainstApv validates an Application against its corresponding
// ApplicationPackageVersion (APV). It fetches the APV once and performs two checks:
//
//  1. Settings schema validation — Application.spec.settings are validated against the
//     OpenAPI schema published at APV.status.packageSchemas.settingsSchema (if present).
//  2. Requirement constraints — the APV's requirements (Kubernetes version, Deckhouse
//     version, and module dependencies) are parsed into semver constraints and delegated
//     to the package manager's CheckConstraints method.
//
// This is called during admission webhook validation to reject Applications whose
// settings are malformed or whose cluster requirements are not satisfied.
func validateAppAgainstApv(ctx context.Context, cli client.Client, manager packageManager, app, oldApp *v1alpha1.Application) error {
	// Build the deterministic APV name from the Application's spec fields (repo, package, version).
	name := v1alpha1.MakeApplicationPackageVersionName(app.Spec.PackageRepositoryName, app.Spec.PackageName, app.Spec.PackageVersion)

	// Fetch the corresponding ApplicationPackageVersion to read its metadata requirements.
	apv := new(v1alpha1.ApplicationPackageVersion)
	if err := cli.Get(ctx, client.ObjectKey{Name: name}, apv); err != nil {
		return fmt.Errorf("get application package version: %w", err)
	}

	if apv.IsDraft() {
		return fmt.Errorf("application package version '%s' is draft", name)
	}

	if err := validateAppSettings(apv, app, oldApp); err != nil {
		return fmt.Errorf("validate settings: %w", err)
	}

	// Parse the APV's requirements into schedule.Constraints if metadata is present.
	constraints := schedule.Constraints{
		Order: schedule.FunctionalOrder,
	}
	if apv.Status.PackageMetadata != nil && apv.Status.PackageMetadata.Requirements != nil {
		reqs := apv.Status.PackageMetadata.Requirements

		// Parse the minimum Kubernetes version constraint (e.g. ">= 1.28").
		kubernetesConstraint, err := parsePackageConstraint(reqs.Kubernetes)
		if err != nil {
			return fmt.Errorf("parse kubernetes requirement: %w", err)
		}

		constraints.Kubernetes = kubernetesConstraint

		// Parse the minimum Deckhouse version constraint (e.g. ">= 1.60").
		deckhouseConstraint, err := parsePackageConstraint(reqs.Deckhouse)
		if err != nil {
			return fmt.Errorf("parse deckhouse requirement: %w", err)
		}

		constraints.Deckhouse = deckhouseConstraint

		// Parse module dependency constraints. Mandatory entries must be present;
		// conditional entries (formerly the "!optional" suffix) are skippable;
		// anyOf groups require ≥1 installed member that satisfies its constraint;
		// noneOf groups require zero installed members that match their constraints.
		// A name listed in both mandatory and conditional is rejected — silently
		// letting conditional overwrite mandatory would weaken the requirement
		// without telling the user.
		modules := make(map[string]schedule.Dependency)
		var anyOfGroups []schedule.AnyOfGroup
		var noneOfGroups []schedule.NoneOfGroup
		if reqs.Modules != nil {
			for _, dep := range reqs.Modules.Mandatory {
				constraint, err := parsePackageDependencyConstraint(dep.Constraint)
				if err != nil {
					return fmt.Errorf("parse mandatory module requirement '%s': %w", dep.Name, err)
				}

				modules[dep.Name] = schedule.Dependency{
					Constraint: constraint,
					Optional:   false,
				}
			}
			for _, dep := range reqs.Modules.Conditional {
				if _, ok := modules[dep.Name]; ok {
					return fmt.Errorf("parse conditional module requirement '%s': also listed as mandatory", dep.Name)
				}

				if len(dep.Constraint) == 0 {
					return fmt.Errorf("parse conditional module requirement '%s': constraint is required", dep.Name)
				}

				constraint, err := parsePackageDependencyConstraint(dep.Constraint)
				if err != nil {
					return fmt.Errorf("parse conditional module requirement '%s': %w", dep.Name, err)
				}

				modules[dep.Name] = schedule.Dependency{
					Constraint: constraint,
					Optional:   true,
				}
			}

			anyOfGroups = make([]schedule.AnyOfGroup, 0, len(reqs.Modules.AnyOf))
			seenAnyOfNames := make(map[string]struct{}, len(reqs.Modules.AnyOf))
			for i, group := range reqs.Modules.AnyOf {
				if len(group.Name) == 0 {
					return fmt.Errorf("parse anyOf group [%d]: name is required", i)
				}

				if _, dup := seenAnyOfNames[group.Name]; dup {
					return fmt.Errorf("parse anyOf group '%s': duplicate group name", group.Name)
				}

				seenAnyOfNames[group.Name] = struct{}{}

				if len(group.Modules) == 0 {
					return fmt.Errorf("parse anyOf group '%s': at least one member is required", group.Name)
				}

				members := make(map[string]*semver.Constraints, len(group.Modules))
				for _, m := range group.Modules {
					if len(m.Name) == 0 {
						return fmt.Errorf("parse anyOf group '%s': member name is required", group.Name)
					}

					if _, dup := members[m.Name]; dup {
						return fmt.Errorf("parse anyOf group '%s': duplicate member '%s'", group.Name, m.Name)
					}

					if existing, clash := modules[m.Name]; clash {
						bucket := "mandatory"
						if existing.Optional {
							bucket = "conditional"
						}

						return fmt.Errorf("parse anyOf group '%s' member '%s': also listed as %s", group.Name, m.Name, bucket)
					}

					constraint, err := parsePackageDependencyConstraint(m.Constraint)
					if err != nil {
						return fmt.Errorf("parse anyOf group '%s' member '%s': %w", group.Name, m.Name, err)
					}

					members[m.Name] = constraint
				}

				anyOfGroups = append(anyOfGroups, schedule.AnyOfGroup{
					Name:    group.Name,
					Members: members,
				})
			}

			noneOfGroups = make([]schedule.NoneOfGroup, 0, len(reqs.Modules.NoneOf))
			seenNoneOfNames := make(map[string]struct{}, len(reqs.Modules.NoneOf))
			for i, group := range reqs.Modules.NoneOf {
				if len(group.Name) == 0 {
					return fmt.Errorf("parse noneOf group [%d]: name is required", i)
				}

				if _, dup := seenNoneOfNames[group.Name]; dup {
					return fmt.Errorf("parse noneOf group '%s': duplicate group name", group.Name)
				}

				seenNoneOfNames[group.Name] = struct{}{}

				if len(group.Modules) == 0 {
					return fmt.Errorf("parse noneOf group '%s': at least one member is required", group.Name)
				}

				members := make(map[string]*semver.Constraints, len(group.Modules))
				for _, m := range group.Modules {
					if len(m.Name) == 0 {
						return fmt.Errorf("parse noneOf group '%s': member name is required", group.Name)
					}

					if _, dup := members[m.Name]; dup {
						return fmt.Errorf("parse noneOf group '%s': duplicate member '%s'", group.Name, m.Name)
					}

					if existing, clash := modules[m.Name]; clash {
						bucket := "mandatory"
						if existing.Optional {
							bucket = "conditional"
						}

						return fmt.Errorf("parse noneOf group '%s' member '%s': also listed as %s", group.Name, m.Name, bucket)
					}

					for _, ag := range anyOfGroups {
						if _, clash := ag.Members[m.Name]; clash {
							return fmt.Errorf("parse noneOf group '%s' member '%s': also listed in anyOf group '%s'", group.Name, m.Name, ag.Name)
						}
					}

					constraint, err := parsePackageDependencyConstraint(m.Constraint)
					if err != nil {
						return fmt.Errorf("parse noneOf group '%s' member '%s': %w", group.Name, m.Name, err)
					}

					members[m.Name] = constraint
				}

				noneOfGroups = append(noneOfGroups, schedule.NoneOfGroup{
					Name:    group.Name,
					Members: members,
				})
			}
		}

		constraints.Dependencies = modules
		constraints.AnyOf = anyOfGroups
		constraints.NoneOf = noneOfGroups
	}

	// Delegate to the manager which checks the parsed constraints against the
	// actual cluster state and rejects on dependency cycles. The name is the
	// scheduler-side identifier (namespace.name) used by the cycle simulation.
	return manager.CheckConstraints(apps.BuildName(app.Namespace, app.Name), constraints)
}

// validateAppSettings validates Application.spec.settings against the OpenAPI settings
// schema published by the ApplicationPackageVersion at status.packageSchemas.settingsSchema.
// The schema is a typed openapi.OpenAPIV3Schema, marshalled to JSON and passed to
// addon-operator's SchemaStorage, which validates the user-supplied settings wrapped
// under the package name. Returns nil when the APV publishes no settings schema — the
// webhook treats an absent schema as "nothing to validate" rather than a rejection,
// so packages that ship without a schema remain installable.
//
// On UPDATE it additionally enforces x-deckhouse-immutable against oldApp.
func validateAppSettings(apv *v1alpha1.ApplicationPackageVersion, app, oldApp *v1alpha1.Application) error {
	if apv.Status.PackageSchemas == nil {
		return nil
	}

	schemas := apv.Status.PackageSchemas
	if schemas.SettingsSchema == nil || schemas.SettingsSchema.OpenAPIV3Schema == nil {
		return nil
	}

	rawSchema, err := json.Marshal(schemas.SettingsSchema.OpenAPIV3Schema)
	if err != nil {
		return fmt.Errorf("get settings schema: %w", err)
	}

	storage, err := validation.NewSchemaStorage(rawSchema, nil)
	if err != nil {
		return fmt.Errorf("create storage schema: %w", err)
	}

	values := addonutils.Values{app.Spec.PackageName: app.Spec.Settings.GetMap()}
	if err = storage.ValidateConfigValues(app.Spec.PackageName, values); err != nil {
		return err
	}

	return checkImmutableSettings(storage.Schemas[validation.ConfigValuesSchema], app, oldApp)
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
func checkImmutableSettings(settingsSchema *spec.Schema, app, oldApp *v1alpha1.Application) error {
	if oldApp == nil || settingsSchema == nil {
		return nil
	}

	oldSettings, err := effectiveSettings(oldApp)
	if err != nil {
		return err
	}

	// Defaults go on both sides so that a key left out of the manifest compares equal
	// to the value the application actually runs with — otherwise dropping the key
	// would read as "never set" and slip past the check. ApplyDefaults mutates in
	// place, which is safe here because both maps are freshly decoded.
	newSettings := app.Spec.Settings.GetMap()
	validation.ApplyDefaults(oldSettings, settingsSchema)
	validation.ApplyDefaults(newSettings, settingsSchema)
	dropUnsetGrants(settingsSchema, oldSettings, newSettings)

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

// effectiveSettings returns the settings the application actually runs with, which is
// what an immutable field is frozen at: status.lastAppliedConfiguration, written by the
// controller after every successful apply with the schema defaults and the project's
// grant defaults already merged in. spec.settings is the fallback for installs that have
// not been applied since that field was introduced; it is the weaker side, because
// values the controller resolves never appear there.
func effectiveSettings(app *v1alpha1.Application) (map[string]any, error) {
	raw := app.Status.LastAppliedConfiguration.Raw
	if len(raw) == 0 {
		return app.Spec.Settings.GetMap(), nil
	}

	settings := make(map[string]any)
	if err := json.Unmarshal(raw, &settings); err != nil {
		// Same direction as a failed decode of the old object: this is the value
		// immutability is measured against, so guessing at it is not an option.
		return nil, fmt.Errorf("unmarshal last applied configuration: %w", err)
	}

	return settings, nil
}

// dropUnsetGrants removes from oldSettings every grantable field the new manifest leaves
// empty. The controller fills those from the project, so the effective old side holds a
// value the manifest never spelled out: compared as-is, an edit to an unrelated field
// would read as a change to a field nobody touched. An explicit value still compares.
//
// ponytail: grants are the only controller-resolved settings today. Another source of
// them would need the same exclusion here.
func dropUnsetGrants(settingsSchema *spec.Schema, oldSettings, newSettings map[string]any) {
	refs, err := packageschema.CollectGrantRefs(settingsSchema)
	if err != nil {
		return
	}

	for _, ref := range refs {
		// Absent and empty are one case: the grant transformer fills both.
		if value := valueAtPath(newSettings, ref.Path); value == nil || value == "" {
			deleteAtPath(oldSettings, ref.Path)
		}
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

// parsePackageConstraint parses the optional semver expression on a PackageConstraint,
// returning a nil *semver.Constraints when the wrapper is nil or its constraint is empty.
func parsePackageConstraint(c *v1alpha1.VersionConstraint) (*semver.Constraints, error) {
	if c == nil {
		return nil, nil
	}

	return parsePackageDependencyConstraint(c.Constraint)
}

// parsePackageDependencyConstraint parses a raw semver expression, treating an empty
// string as "no constraint" rather than an error.
func parsePackageDependencyConstraint(raw string) (*semver.Constraints, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	return semver.NewConstraint(raw)
}
