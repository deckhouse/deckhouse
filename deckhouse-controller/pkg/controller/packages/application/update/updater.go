// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package update

import (
	"context"
	"fmt"
	"slices"

	"github.com/Masterminds/semver/v3"
	"k8s.io/apimachinery/pkg/api/equality"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status"
)

// Updater keeps status.update of an application: the update plan and the version the automatic update installs next.
type Updater struct {
	client client.Client
}

// NewUpdater returns an Updater; the application controller runs it first on every reconcile.
func NewUpdater(cli client.Client) *Updater {
	return &Updater{client: cli}
}

// Update patches status.update of the application; Manual mode and a not yet installed application clear it.
// The plan starts after automaticVersion, so the version being installed is not listed twice.
func (u *Updater) Update(ctx context.Context, app *v1alpha1.Application) error {
	var desired *v1alpha1.ApplicationStatusUpdate

	if mode := updateMode(app); mode != v1alpha2.UpdateModeManual && app.Status.CurrentVersion != nil {
		apvs := new(v1alpha1.ApplicationPackageVersionList)
		if err := u.client.List(ctx, apvs, client.MatchingLabels{
			v1alpha1.PackageLabelRepository: app.Spec.PackageRepositoryName,
			v1alpha1.PackageLabelPackage:    app.Spec.PackageName,
		}); err != nil {
			return fmt.Errorf("list application package versions: %w", err)
		}

		plan := computePlan(mode, app.Status.CurrentVersion.Version, apvs.Items)
		automatic := computeAutoVersion(app, plan, installableVersions(apvs.Items))
		if len(plan) > 0 && plan[0] == automatic {
			plan = plan[1:]
		}

		desired = &v1alpha1.ApplicationStatusUpdate{
			Plan:             plan,
			AutomaticVersion: automatic,
		}

		if len(desired.Plan) == 0 && desired.AutomaticVersion == "" {
			desired = nil
		}
	}

	if equality.Semantic.DeepEqual(app.Status.Update, desired) {
		return nil
	}

	patch := client.MergeFrom(app.DeepCopy())
	app.Status.Update = desired
	if err := u.client.Status().Patch(ctx, app, patch); err != nil {
		return fmt.Errorf("patch status: %w", err)
	}

	return nil
}

// updateMode returns the mode of spec.update; without it the default Manual applies.
func updateMode(app *v1alpha1.Application) v1alpha2.UpdateMode {
	if app.Spec.Update == nil {
		return v1alpha2.UpdateModeManual
	}

	return ParseMode(app.Spec.Update.Mode)
}

// ParseMode parses an application update mode; unlike v1alpha2.ParseUpdateMode, an empty one is Manual.
func ParseMode(mode string) v1alpha2.UpdateMode {
	if mode == "" {
		return v1alpha2.UpdateModeManual
	}

	return v1alpha2.ParseUpdateMode(mode)
}

// Version returns the version the engine runs: in Auto and AutoPatch modes the newer of spec.packageVersion and
// status.update.automaticVersion, otherwise spec.packageVersion.
func Version(app *v1alpha1.Application) string {
	if updateMode(app) == v1alpha2.UpdateModeManual || app.Status.Update == nil {
		return app.Spec.PackageVersion
	}

	if newer(app.Status.Update.AutomaticVersion, app.Spec.PackageVersion) {
		return app.Status.Update.AutomaticVersion
	}

	return app.Spec.PackageVersion
}

// candidate is a version the plan may step to, with the rules that gate an upgrade to it.
type candidate struct {
	version *semver.Version
	rules   *v1alpha1.PackageVersionCompatibilityRules
}

// computePlan returns the update path from current, each step the newest version the previous one may upgrade to.
// Drafts and versions gone from the registry are skipped; on AutoPatch so are versions outside the current minor.
func computePlan(mode v1alpha2.UpdateMode, current string, apvs []v1alpha1.ApplicationPackageVersion) []string {
	from, err := semver.NewVersion(current)
	if err != nil {
		return nil
	}

	candidates := make([]candidate, 0, len(apvs))
	for _, apv := range apvs {
		if !installable(apv) {
			continue
		}

		version, err := semver.NewVersion(apv.Spec.PackageVersion)
		if err != nil || !version.GreaterThan(from) {
			continue
		}

		if mode == v1alpha2.UpdateModeAutoPatch && (version.Major() != from.Major() || version.Minor() != from.Minor()) {
			continue
		}

		var rules *v1alpha1.PackageVersionCompatibilityRules
		if apv.Status.PackageMetadata != nil {
			rules = apv.Status.PackageMetadata.Compatibility
		}

		candidates = append(candidates, candidate{version: version, rules: rules})
	}

	// Newest first: the first compatible candidate is the step, and only the ones before it are newer.
	slices.SortFunc(candidates, func(a, b candidate) int {
		return b.version.Compare(a.version)
	})

	var plan []string
	for {
		idx := slices.IndexFunc(candidates, func(c candidate) bool {
			return c.rules.IsCompatible(from.Original(), c.version.Original())
		})
		if idx < 0 {
			return plan
		}

		from = candidates[idx].version
		plan = append(plan, from.Original())
		candidates = candidates[:idx]
	}
}

// computeAutoVersion returns the version the automatic update moves the application to: the previous automatic
// version until status.currentVersion reaches it, while it is still installable; status.currentVersion while
// UpdateInstalled is False or Ready is not True; otherwise the first step of the plan. So every step installs and
// gets ready before the next one is chosen. It never falls below status.currentVersion, so Version never sends the
// engine back to an older spec.
func computeAutoVersion(app *v1alpha1.Application, plan []string, available map[string]bool) string {
	current := app.Status.CurrentVersion.Version

	// The engine may have activated it already while the status still shows the previous step.
	if app.Status.Update != nil && newer(app.Status.Update.AutomaticVersion, current) && available[app.Status.Update.AutomaticVersion] {
		return app.Status.Update.AutomaticVersion
	}

	if holding(app) || len(plan) == 0 || !newer(plan[0], current) {
		return current
	}

	return plan[0]
}

// holding reports whether the version on its way is not done yet: UpdateInstalled is False or Ready is not True.
// UpdateInstalled is absent until the first update, so only False holds; Ready holds until it is reported True.
func holding(app *v1alpha1.Application) bool {
	return apimeta.IsStatusConditionFalse(app.Status.Conditions, status.ConditionUpdateInstalled) ||
		!apimeta.IsStatusConditionTrue(app.Status.Conditions, status.ConditionReady)
}

// installable reports whether the package version may be installed: not a draft and still in the registry.
func installable(apv v1alpha1.ApplicationPackageVersion) bool {
	return !apv.IsDraft() && apv.Labels[v1alpha1.PackageLabelExistInRegistry] != "false"
}

// installableVersions returns the versions of the installable package versions.
func installableVersions(apvs []v1alpha1.ApplicationPackageVersion) map[string]bool {
	versions := make(map[string]bool, len(apvs))
	for _, apv := range apvs {
		if installable(apv) {
			versions[apv.Spec.PackageVersion] = true
		}
	}

	return versions
}

// newer reports whether version is greater than current; unparsable versions are not.
func newer(version, current string) bool {
	v, err := semver.NewVersion(version)
	if err != nil {
		return false
	}

	c, err := semver.NewVersion(current)
	if err != nil {
		return false
	}

	return v.GreaterThan(c)
}
