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
	"testing"

	"github.com/stretchr/testify/suite"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status"
	"github.com/deckhouse/deckhouse/go_lib/project"
)

// White-box: plan and automatic version are internal decision functions, Update only patches their result.
type UpdaterSuite struct {
	suite.Suite
}

func TestUpdater(t *testing.T) {
	suite.Run(t, new(UpdaterSuite))
}

func (s *UpdaterSuite) TestPlanStepsThroughNewestCompatible() {
	apvs := apvList("v1.2.5", "v1.3.0", "v1.3.4", "v1.4.2", "v2.0.0")

	s.Equal([]string{"v1.3.4", "v1.4.2"}, computePlan(v1alpha2.UpdateModeAuto, "v1.2.3", apvs))
}

func (s *UpdaterSuite) TestPlanFollowsTargetRules() {
	apvs := apvList("v1.3.0", "v2.0.0")
	apvs[1].Status.PackageMetadata = &v1alpha1.ApplicationPackageVersionStatusMetadata{
		Compatibility: &v1alpha1.PackageVersionCompatibilityRules{
			Upgrade: &v1alpha1.PackageVersionCompatibilityRule{
				Step: v1alpha1.PackageVersionCompatibilityRuleStep{Major: ptr.To[int32](1)},
			},
		},
	}

	s.Equal([]string{"v2.0.0"}, computePlan(v1alpha2.UpdateModeAuto, "v1.2.3", apvs))
}

func (s *UpdaterSuite) TestPlanTakesRulesOfEachStep() {
	apvs := apvList("v1.3.0", "v1.4.0", "v1.5.0")
	apvs[2].Status.PackageMetadata = &v1alpha1.ApplicationPackageVersionStatusMetadata{
		Compatibility: &v1alpha1.PackageVersionCompatibilityRules{
			Upgrade: &v1alpha1.PackageVersionCompatibilityRule{
				Step: v1alpha1.PackageVersionCompatibilityRuleStep{Minor: ptr.To[int32](2)},
			},
		},
	}

	// v1.5.0 is two minors from v1.3.0, which its own rules allow, so v1.4.0 is skipped.
	s.Equal([]string{"v1.3.0", "v1.5.0"}, computePlan(v1alpha2.UpdateModeAuto, "v1.2.3", apvs))
}

func (s *UpdaterSuite) TestPlanStepsPatchByPatchWhenRulesLimitPatches() {
	apvs := patchByPatch(apvList("v1.2.4", "v1.2.5", "v1.2.6"))

	s.Equal([]string{"v1.2.4", "v1.2.5", "v1.2.6"}, computePlan(v1alpha2.UpdateModeAutoPatch, "v1.2.3", apvs))
}

func (s *UpdaterSuite) TestPlanStaysWithinMinorOnAutoPatch() {
	apvs := apvList("v1.2.4", "v1.2.5", "v1.3.0")

	s.Equal([]string{"v1.2.5"}, computePlan(v1alpha2.UpdateModeAutoPatch, "v1.2.3", apvs))
}

func (s *UpdaterSuite) TestPlanSkipsUnavailableVersions() {
	apvs := apvList("v1.2.4", "v1.2.5", "v1.2.6", "v1.2.2")
	apvs[1].Labels[v1alpha1.PackageLabelDraft] = "true"
	apvs[2].Labels[v1alpha1.PackageLabelExistInRegistry] = "false"

	s.Equal([]string{"v1.2.4"}, computePlan(v1alpha2.UpdateModeAuto, "v1.2.3", apvs))
}

func (s *UpdaterSuite) TestAutoVersionTakesFirstStep() {
	app := installedApp("v1.2.3", "")

	s.Equal("v1.3.4", computeAutoVersion(app, []string{"v1.3.4", "v1.4.2"}, nil))
}

func (s *UpdaterSuite) TestAutoVersionIgnoresStepNotNewerThanCurrent() {
	app := installedApp("v1.2.3", "")

	s.Equal("v1.2.3", computeAutoVersion(app, []string{"v1.2.3"}, nil))
}

func (s *UpdaterSuite) TestAutoVersionKeepsCurrentWithoutNewer() {
	app := installedApp("v1.3.0", "v1.3.0")
	setCondition(app, status.ConditionUpdateInstalled, metav1.ConditionTrue)

	s.Equal("v1.3.0", computeAutoVersion(app, nil, nil))
}

func (s *UpdaterSuite) TestAutoVersionHoldsVersionOnItsWayUntilInstalled() {
	app := installedApp("v1.2.3", "v1.3.0")

	// The engine may have activated v1.3.0 while the status still shows v1.2.3 installed and ready.
	s.Equal("v1.3.0", computeAutoVersion(app, []string{"v1.3.4"}, versionSet("v1.3.0", "v1.3.4")))
}

func (s *UpdaterSuite) TestAutoVersionDropsVersionOnItsWayNoLongerInstallable() {
	app := installedApp("v1.2.3", "v1.3.0")

	s.Equal("v1.3.4", computeAutoVersion(app, []string{"v1.3.4"}, versionSet("v1.3.4")))
}

func (s *UpdaterSuite) TestAutoVersionHoldsCurrentWhenVersionOnItsWayGoneWhileUpdateInstalledFalse() {
	app := installedApp("v1.2.3", "v1.3.0")
	setCondition(app, status.ConditionUpdateInstalled, metav1.ConditionFalse)

	s.Equal("v1.2.3", computeAutoVersion(app, []string{"v1.3.4"}, versionSet("v1.3.4")))
}

func (s *UpdaterSuite) TestAutoVersionHoldsUntilUpdateInstalled() {
	app := installedApp("v1.3.0", "v1.3.0")
	setCondition(app, status.ConditionUpdateInstalled, metav1.ConditionFalse)

	s.Equal("v1.3.0", computeAutoVersion(app, []string{"v1.4.2"}, nil))
}

func (s *UpdaterSuite) TestAutoVersionHoldsSpecVersionUntilUpdateInstalled() {
	app := installedApp("v1.4.0", "v1.3.0")
	setCondition(app, status.ConditionUpdateInstalled, metav1.ConditionFalse)

	s.Equal("v1.4.0", computeAutoVersion(app, []string{"v1.4.2"}, nil))
}

func (s *UpdaterSuite) TestAutoVersionKeepsVersionOnItsWayWhileUpdateInstalledFalse() {
	app := installedApp("v1.3.0", "v1.4.0")
	setCondition(app, status.ConditionUpdateInstalled, metav1.ConditionFalse)

	s.Equal("v1.4.0", computeAutoVersion(app, []string{"v1.5.0"}, versionSet("v1.4.0", "v1.5.0")))
}

func (s *UpdaterSuite) TestAutoVersionHoldsUntilReady() {
	app := installedApp("v1.3.0", "v1.3.0")
	setCondition(app, status.ConditionUpdateInstalled, metav1.ConditionTrue)
	setCondition(app, status.ConditionReady, metav1.ConditionFalse)

	s.Equal("v1.3.0", computeAutoVersion(app, []string{"v1.4.2"}, nil))
}

func (s *UpdaterSuite) TestAutoVersionHoldsWithoutReady() {
	app := installedApp("v1.3.0", "v1.3.0")
	setCondition(app, status.ConditionUpdateInstalled, metav1.ConditionTrue)
	apimeta.RemoveStatusCondition(&app.Status.Conditions, status.ConditionReady)

	s.Equal("v1.3.0", computeAutoVersion(app, []string{"v1.4.2"}, nil))
}

func (s *UpdaterSuite) TestAutoVersionKeepsVersionOnItsWayUntilReady() {
	app := installedApp("v1.3.0", "v1.4.0")
	setCondition(app, status.ConditionReady, metav1.ConditionFalse)

	s.Equal("v1.4.0", computeAutoVersion(app, []string{"v1.5.0"}, versionSet("v1.4.0", "v1.5.0")))
}

func (s *UpdaterSuite) TestAutoVersionAdvancesWhenNeverUpdated() {
	app := installedApp("v1.2.3", "v1.2.3")

	s.Equal("v1.2.4", computeAutoVersion(app, []string{"v1.2.4"}, nil))
}

func (s *UpdaterSuite) TestAutoVersionAdvancesOnceInstalled() {
	app := installedApp("v1.3.0", "v1.3.0")
	setCondition(app, status.ConditionUpdateInstalled, metav1.ConditionTrue)

	s.Equal("v1.4.2", computeAutoVersion(app, []string{"v1.4.2"}, nil))
}

func (s *UpdaterSuite) TestAutoVersionDropsVersionMovedPast() {
	app := installedApp("v1.4.0", "v1.3.0")

	s.Equal("v1.4.2", computeAutoVersion(app, []string{"v1.4.2"}, nil))
}

func (s *UpdaterSuite) TestUpdateWritesAutomaticVersion() {
	app := installedApp("v1.2.3", "")
	app.Spec.Update = &v1alpha1.ApplicationUpdate{Mode: string(v1alpha2.UpdateModeAuto)}
	cli := s.client(app, apvList("v1.2.5", "v1.3.0")...)

	s.update(cli, app)

	s.Equal(&v1alpha1.ApplicationStatusUpdate{AutomaticVersion: "v1.3.0"}, s.get(cli, app).Status.Update)
}

func (s *UpdaterSuite) TestUpdatePlanStartsAfterAutomaticVersion() {
	app := installedApp("v1.2.3", "")
	cli := s.client(app, patchByPatch(apvList("v1.2.4", "v1.2.5", "v1.2.6"))...)

	s.update(cli, app)

	s.Equal(&v1alpha1.ApplicationStatusUpdate{Plan: []string{"v1.2.5", "v1.2.6"}, AutomaticVersion: "v1.2.4"},
		s.get(cli, app).Status.Update)
}

func (s *UpdaterSuite) TestUpdateKeepsWholePlanWhileHolding() {
	app := installedApp("v1.2.4", "v1.2.4")
	setCondition(app, status.ConditionUpdateInstalled, metav1.ConditionFalse)
	cli := s.client(app, patchByPatch(apvList("v1.2.4", "v1.2.5", "v1.2.6"))...)

	s.update(cli, app)

	s.Equal(&v1alpha1.ApplicationStatusUpdate{Plan: []string{"v1.2.5", "v1.2.6"}, AutomaticVersion: "v1.2.4"},
		s.get(cli, app).Status.Update)
}

func (s *UpdaterSuite) TestUpdateHoldsVersionOnItsWay() {
	app := installedApp("v1.2.3", "v1.2.4")
	cli := s.client(app, apvList("v1.2.4", "v1.2.5")...)

	s.update(cli, app)

	s.Equal(&v1alpha1.ApplicationStatusUpdate{Plan: []string{"v1.2.5"}, AutomaticVersion: "v1.2.4"},
		s.get(cli, app).Status.Update)
}

func (s *UpdaterSuite) TestUpdateDropsDraftVersionOnItsWay() {
	app := installedApp("v1.2.3", "v1.2.4")
	apvs := apvList("v1.2.4", "v1.2.5")
	apvs[0].Labels[v1alpha1.PackageLabelDraft] = "true"
	cli := s.client(app, apvs...)

	s.update(cli, app)

	s.Equal(&v1alpha1.ApplicationStatusUpdate{AutomaticVersion: "v1.2.5"}, s.get(cli, app).Status.Update)
}

func (s *UpdaterSuite) TestUpdateClearsOnManual() {
	app := installedApp("v1.2.3", "v1.2.5")
	app.Status.Update.Plan = []string{"v1.2.5"}
	app.Spec.Update = &v1alpha1.ApplicationUpdate{Mode: string(v1alpha2.UpdateModeManual)}
	cli := s.client(app, apvList("v1.2.5")...)

	s.update(cli, app)

	s.Nil(s.get(cli, app).Status.Update)
}

func (s *UpdaterSuite) TestUpdateClearsWithoutMode() {
	app := installedApp("v1.2.3", "v1.2.5")
	app.Spec.Update = nil
	cli := s.client(app, apvList("v1.2.5")...)

	s.update(cli, app)

	s.Nil(s.get(cli, app).Status.Update)
}

func (s *UpdaterSuite) TestUpdateKeepsCurrentWithoutOwnPackageVersions() {
	app := installedApp("v1.2.3", "")
	other := apvList("v1.2.5")
	other[0].Labels[v1alpha1.PackageLabelPackage] = "other"
	cli := s.client(app, other...)

	s.update(cli, app)

	s.Equal(&v1alpha1.ApplicationStatusUpdate{AutomaticVersion: "v1.2.3"}, s.get(cli, app).Status.Update)
}

func (s *UpdaterSuite) TestVersionTakesNewerAutomaticVersion() {
	app := installedApp("v1.2.3", "v1.3.0")

	s.Equal("v1.3.0", Version(app))
}

func (s *UpdaterSuite) TestVersionKeepsNewerSpec() {
	app := installedApp("v1.2.3", "v1.3.0")
	app.Spec.PackageVersion = "v1.4.0"

	s.Equal("v1.4.0", Version(app))
}

func (s *UpdaterSuite) TestVersionIgnoresAutomaticVersionOnManual() {
	app := installedApp("v1.2.3", "v1.3.0")
	app.Spec.Update = &v1alpha1.ApplicationUpdate{Mode: string(v1alpha2.UpdateModeManual)}

	s.Equal("v1.2.3", Version(app))
}

func (s *UpdaterSuite) TestVersionIgnoresAutomaticVersionWithoutMode() {
	app := installedApp("v1.2.3", "v1.3.0")
	app.Spec.Update = &v1alpha1.ApplicationUpdate{}

	s.Equal("v1.2.3", Version(app))
}

// client returns a fake client holding the application and the package versions.
func (s *UpdaterSuite) client(app *v1alpha1.Application, apvs ...v1alpha1.ApplicationPackageVersion) client.Client {
	scheme, err := project.Scheme()
	s.Require().NoError(err)

	objs := []client.Object{app}
	for i := range apvs {
		objs = append(objs, &apvs[i])
	}

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&v1alpha1.Application{}).
		Build()
}

// update runs Update on the application as stored in the client.
func (s *UpdaterSuite) update(cli client.Client, app *v1alpha1.Application) {
	s.Require().NoError(NewUpdater(cli).Update(context.Background(), s.get(cli, app)))
}

// get reads the application back from the client.
func (s *UpdaterSuite) get(cli client.Client, app *v1alpha1.Application) *v1alpha1.Application {
	got := new(v1alpha1.Application)
	s.Require().NoError(cli.Get(context.Background(), client.ObjectKeyFromObject(app), got))

	return got
}

// installedApp returns a ready application of the "console" package running current in AutoPatch, with the given
// automatic version.
func installedApp(current, automatic string) *v1alpha1.Application {
	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: "console", Namespace: "default"},
		Spec: v1alpha1.ApplicationSpec{
			PackageName:           "console",
			PackageRepositoryName: "deckhouse",
			PackageVersion:        current,
			Update:                &v1alpha1.ApplicationUpdate{Mode: string(v1alpha2.UpdateModeAutoPatch)},
		},
		Status: v1alpha1.ApplicationStatus{
			CurrentVersion: &v1alpha1.ApplicationStatusVersion{Version: current},
		},
	}

	if automatic != "" {
		app.Status.Update = &v1alpha1.ApplicationStatusUpdate{AutomaticVersion: automatic}
	}

	setCondition(app, status.ConditionReady, metav1.ConditionTrue)

	return app
}

// setCondition sets a condition of the application.
func setCondition(app *v1alpha1.Application, condType string, condStatus metav1.ConditionStatus) {
	apimeta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{Type: condType, Status: condStatus})
}

// versionSet returns the given versions as a set of installable versions.
func versionSet(versions ...string) map[string]bool {
	set := make(map[string]bool, len(versions))
	for _, version := range versions {
		set[version] = true
	}

	return set
}

// patchByPatch limits the upgrade of every version to one patch.
func patchByPatch(apvs []v1alpha1.ApplicationPackageVersion) []v1alpha1.ApplicationPackageVersion {
	for i := range apvs {
		apvs[i].Status.PackageMetadata = &v1alpha1.ApplicationPackageVersionStatusMetadata{
			Compatibility: &v1alpha1.PackageVersionCompatibilityRules{
				Upgrade: &v1alpha1.PackageVersionCompatibilityRule{
					Step: v1alpha1.PackageVersionCompatibilityRuleStep{Patches: ptr.To[int32](1)},
				},
			},
		}
	}

	return apvs
}

// apvList returns loaded package versions of the "console" package in the "deckhouse" repository.
func apvList(versions ...string) []v1alpha1.ApplicationPackageVersion {
	apvs := make([]v1alpha1.ApplicationPackageVersion, 0, len(versions))
	for _, version := range versions {
		apvs = append(apvs, v1alpha1.ApplicationPackageVersion{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1alpha1.MakeApplicationPackageVersionName("deckhouse", "console", version),
				Labels: map[string]string{
					v1alpha1.PackageLabelRepository: "deckhouse",
					v1alpha1.PackageLabelPackage:    "console",
				},
			},
			Spec: v1alpha1.ApplicationPackageVersionSpec{
				PackageName:           "console",
				PackageRepositoryName: "deckhouse",
				PackageVersion:        version,
			},
		})
	}

	return apvs
}
