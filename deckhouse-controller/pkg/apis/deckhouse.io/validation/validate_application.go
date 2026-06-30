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
	"fmt"
	"net/http"

	"github.com/Masterminds/semver/v3"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/values/validation"
	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

// applicationValidationHandler validations for Application creation
func applicationValidationHandler(cli client.Client, manager packageManager) http.Handler {
	vf := kwhvalidating.ValidatorFunc(func(ctx context.Context, _ *kwhmodel.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
		app, ok := obj.(*v1alpha1.Application)
		if !ok {
			return nil, fmt.Errorf("expect Application as unstructured, got %T", obj)
		}

		// no sense to check already deleted app
		if !app.DeletionTimestamp.IsZero() {
			return allowResult(nil)
		}

		ap := new(v1alpha1.ApplicationPackage)
		if err := cli.Get(ctx, client.ObjectKey{Name: app.Spec.PackageName}, ap); err != nil {
			return rejectResult(fmt.Sprintf("get application package: %v", err))
		}

		name := apps.BuildName(app.Namespace, app.Name)

		res, err := manager.ValidateAppSettings(ctx, name, app.Spec.Settings.GetMap())
		if err != nil {
			return nil, err
		}

		if !res.Valid {
			return rejectResult(res.Message)
		}

		if err = validateAppAgainstApv(ctx, cli, manager, app); err != nil {
			return rejectResult(err.Error())
		}

		return allowResult(res.Warnings)
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
func validateAppAgainstApv(ctx context.Context, cli client.Client, manager packageManager, app *v1alpha1.Application) error {
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

	if err := validateAppSettings(apv, app); err != nil {
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
func validateAppSettings(apv *v1alpha1.ApplicationPackageVersion, app *v1alpha1.Application) error {
	if apv.Status.PackageSchemas == nil {
		return nil
	}

	schemas := apv.Status.PackageSchemas
	if schemas.SettingsSchema == nil || schemas.SettingsSchema.OpenAPIV3Schema == nil {
		return nil
	}

	schema, err := json.Marshal(schemas.SettingsSchema.OpenAPIV3Schema)
	if err != nil {
		return fmt.Errorf("get settings schema: %w", err)
	}

	storage, err := validation.NewSchemaStorage(schema, nil)
	if err != nil {
		return fmt.Errorf("create storage schema: %w", err)
	}

	values := addonutils.Values{app.Spec.PackageName: app.Spec.Settings.GetMap()}
	return storage.ValidateConfigValues(app.Spec.PackageName, values)
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
