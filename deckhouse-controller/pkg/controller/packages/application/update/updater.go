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

// Update patches status.update of the application; a not yet installed application clears it. The plan is the whole
// path after automaticVersion in every mode, so it shows where the application may go next; automaticVersion is
// empty in Manual mode.
func (u *Updater) Update(ctx context.Context, app *v1alpha1.Application) error {
	var desired *v1alpha1.ApplicationStatusUpdate

	if app.Status.CurrentVersion != nil {
		apvs := new(v1alpha1.ApplicationPackageVersionList)
		if err := u.client.List(ctx, apvs, client.MatchingLabels{
			v1alpha1.PackageLabelRepository: app.Spec.PackageRepositoryName,
			v1alpha1.PackageLabelPackage:    app.Spec.PackageName,
		}); err != nil {
			return fmt.Errorf("list application package versions: %w", err)
		}

		current := app.Status.CurrentVersion.Version

		var automatic string
		if mode := app.UpdateMode(); mode != v1alpha2.UpdateModeManual {
			automatic = computeAutoVersion(app, computePlan(mode, current, apvs.Items), installableVersions(apvs.Items))
		}

		from := current
		if newer(automatic, current) {
			from = automatic
		}

		desired = &v1alpha1.ApplicationStatusUpdate{
			// Auto, unlike AutoPatch, does not stop at the current minor: the whole path.
			Plan:             computePlan(v1alpha2.UpdateModeAuto, from, apvs.Items),
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

// candidate is a version the plan may step to, with the rules that bound the next step from it.
type candidate struct {
	version *semver.Version
	rules   *v1alpha1.PackageVersionCompatibilityRules
}

// computePlan returns the update path from current, each step the newest version the previous one's rules allow.
// Drafts and versions gone from the registry are skipped; on AutoPatch so are versions outside the current minor.
func computePlan(mode v1alpha2.UpdateMode, current string, apvs []v1alpha1.ApplicationPackageVersion) []string {
	from, err := semver.NewVersion(current)
	if err != nil {
		return nil
	}

	rules := versionRules(from, apvs)

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

		candidates = append(candidates, candidate{version: version, rules: compatibility(apv)})
	}

	// Newest first: the first compatible candidate is the step, and only the ones before it are newer.
	slices.SortFunc(candidates, func(a, b candidate) int {
		return b.version.Compare(a.version)
	})

	var plan []string
	for {
		idx := slices.IndexFunc(candidates, func(c candidate) bool {
			return rules.IsCompatible(from.Original(), c.version.Original())
		})
		if idx < 0 {
			return plan
		}

		from, rules = candidates[idx].version, candidates[idx].rules
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

// versionRules returns the compatibility rules of the package version of version; nil, the defaults, when it is gone.
func versionRules(version *semver.Version, apvs []v1alpha1.ApplicationPackageVersion) *v1alpha1.PackageVersionCompatibilityRules {
	for _, apv := range apvs {
		if v, err := semver.NewVersion(apv.Spec.PackageVersion); err == nil && v.Equal(version) {
			return compatibility(apv)
		}
	}

	return nil
}

// compatibility returns the compatibility rules the package version publishes; nil, the defaults, without metadata.
func compatibility(apv v1alpha1.ApplicationPackageVersion) *v1alpha1.PackageVersionCompatibilityRules {
	if apv.Status.PackageMetadata == nil {
		return nil
	}

	return apv.Status.PackageMetadata.Compatibility
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
