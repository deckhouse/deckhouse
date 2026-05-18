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
	"strings"

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

		res, err := manager.ValidateSettings(ctx, name, app.Spec.Settings.GetMap())
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
		var err error

		// Parse the minimum Kubernetes version constraint (e.g. ">=1.28").
		var kubernetesConstraint *semver.Constraints
		if len(apv.Status.PackageMetadata.Requirements.Kubernetes) > 0 {
			if kubernetesConstraint, err = semver.NewConstraint(apv.Status.PackageMetadata.Requirements.Kubernetes); err != nil {
				return fmt.Errorf("parse kubernetes requirement: %w", err)
			}
		}

		constraints.Kubernetes = kubernetesConstraint

		// Parse the minimum Deckhouse version constraint (e.g. ">=1.60").
		var deckhouseConstraint *semver.Constraints
		if len(apv.Status.PackageMetadata.Requirements.Deckhouse) > 0 {
			if deckhouseConstraint, err = semver.NewConstraint(apv.Status.PackageMetadata.Requirements.Deckhouse); err != nil {
				return fmt.Errorf("parse deckhouse requirement: %w", err)
			}
		}

		constraints.Deckhouse = deckhouseConstraint

		// Parse module dependency constraints. Each module requirement may have an
		// "!optional" suffix indicating the dependency is not mandatory.
		modules := make(map[string]schedule.Dependency)
		for module, rawConstraint := range apv.Status.PackageMetadata.Requirements.Modules {
			raw, optional := strings.CutSuffix(rawConstraint, "!optional")
			constraint, err := semver.NewConstraint(raw)
			if err != nil {
				return fmt.Errorf("parse module requirement '%s': %w", module, err)
			}

			modules[module] = schedule.Dependency{
				Constraint: constraint,
				Optional:   optional,
			}
		}

		constraints.Dependencies = modules
	}

	// Delegate to the manager which checks the parsed constraints against actual cluster state.
	return manager.CheckConstraints(constraints)
}

// validateAppSettings validates Application.spec.settings against the OpenAPI settings
// schema published by the ApplicationPackageVersion at status.packageSchemas.settingsSchema.
// The schema is re-serialized to JSON, loaded into an addon-operator SchemaStorage, and the
// user-supplied settings are wrapped under the package name (as addon-operator expects) and
// validated. Returns nil when the APV publishes no settings schema — the webhook treats an
// absent schema as "nothing to validate" rather than a rejection, so packages that ship
// without a schema remain installable.
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
