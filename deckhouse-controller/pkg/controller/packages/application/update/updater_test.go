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
	"math/rand/v2"
	"slices"
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

func (s *UpdaterSuite) TestPlanFollowsRulesOfInstalledVersion() {
	apvs := apvList("v1.2.3", "v1.3.0", "v2.0.0")
	limitUpgrade(&apvs[0], v1alpha1.PackageVersionCompatibilityRuleStep{Major: ptr.To[int32](1)})

	s.Equal([]string{"v2.0.0"}, computePlan(v1alpha2.UpdateModeAuto, "v1.2.3", apvs))
}

func (s *UpdaterSuite) TestPlanIgnoresStepOfTarget() {
	apvs := apvList("v1.2.3", "v1.3.0", "v2.0.0")
	limitUpgrade(&apvs[2], v1alpha1.PackageVersionCompatibilityRuleStep{Major: ptr.To[int32](1)})

	s.Equal([]string{"v1.3.0"}, computePlan(v1alpha2.UpdateModeAuto, "v1.2.3", apvs))
}

func (s *UpdaterSuite) TestPlanTakesRulesOfEachStep() {
	apvs := apvList("v1.2.3", "v1.3.0", "v1.4.0", "v1.5.0")
	limitUpgrade(&apvs[1], v1alpha1.PackageVersionCompatibilityRuleStep{Minor: ptr.To[int32](2)})

	// v1.3.0 allows two minors, so v1.4.0 is skipped.
	s.Equal([]string{"v1.3.0", "v1.5.0"}, computePlan(v1alpha2.UpdateModeAuto, "v1.2.3", apvs))
}

func (s *UpdaterSuite) TestPlanStepIsBoundByVersionLeft() {
	apvs := patchByPatch(apvList("v1.0.0", "v1.0.1", "v1.0.2", "v1.0.3", "v1.0.4"))
	limitUpgrade(&apvs[0], v1alpha1.PackageVersionCompatibilityRuleStep{Patches: ptr.To[int32](2)})

	// v1.0.0 allows two patches, every later version one.
	s.Equal([]string{"v1.0.2", "v1.0.3", "v1.0.4"}, computePlan(v1alpha2.UpdateModeAutoPatch, "v1.0.0", apvs))
}

func (s *UpdaterSuite) TestPlanCrossesMinorOnlyFromReleaseThatAllowsIt() {
	apvs := apvList("v0.0.1", "v0.0.2", "v0.0.3", "v0.0.4", "v0.0.5", "v0.1.0", "v0.1.1")
	for i := range 4 {
		limitUpgrade(&apvs[i], v1alpha1.PackageVersionCompatibilityRuleStep{Minor: ptr.To[int32](0)})
	}

	// v0.0.1 allows patches only; v0.0.5 keeps the default of one minor.
	s.Equal([]string{"v0.0.5", "v0.1.1"}, computePlan(v1alpha2.UpdateModeAuto, "v0.0.1", apvs))
}

func (s *UpdaterSuite) TestPlanStepsPatchByPatchWhenRulesLimitPatches() {
	apvs := patchByPatch(apvList("v1.2.3", "v1.2.4", "v1.2.5", "v1.2.6"))

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

func (s *UpdaterSuite) TestPlanTakesUpgradeFromOfTargetOverStep() {
	apvs := apvList("v1.9.0", "v2.0.0")
	acceptUpgradeFrom(&apvs[1], ">=1.8.0 <2.0.0")

	// v1.9.0 forbids another major by default, v2.0.0 accepts it.
	s.Equal([]string{"v2.0.0"}, computePlan(v1alpha2.UpdateModeAuto, "v1.9.0", apvs))
}

func (s *UpdaterSuite) TestPlanStepsToVersionUpgradeFromAccepts() {
	apvs := apvList("v1.2.0", "v1.2.5", "v1.3.0")
	acceptUpgradeFrom(&apvs[2], ">=1.2.5")

	// v1.2.0 allows the next minor by default, v1.3.0 accepts only v1.2.5 and later.
	s.Equal([]string{"v1.2.5", "v1.3.0"}, computePlan(v1alpha2.UpdateModeAuto, "v1.2.0", apvs))
}

func (s *UpdaterSuite) TestPlanJumpsSeveralMinorsUpgradeFromAccepts() {
	apvs := apvList("v1.2.0", "v1.3.0", "v1.4.0", "v1.5.0")
	acceptUpgradeFrom(&apvs[3], ">=1.2.0")

	// The step allows one minor by default, v1.5.0 accepts v1.2.0 three minors back.
	s.Equal([]string{"v1.5.0"}, computePlan(v1alpha2.UpdateModeAuto, "v1.2.0", apvs))
}

func (s *UpdaterSuite) TestPlanStepsMinorByMinorUntilUpgradeFromAccepts() {
	apvs := apvList("v1.1.0", "v1.2.0", "v1.3.0", "v1.4.0", "v2.0.0")
	acceptUpgradeFrom(&apvs[4], ">=1.3.0 <2.0.0")

	// One minor at a time up to v1.3.0, the lowest version v2.0.0 accepts; from there v2.0.0 is newer than v1.4.0.
	s.Equal([]string{"v1.2.0", "v1.3.0", "v2.0.0"}, computePlan(v1alpha2.UpdateModeAuto, "v1.1.0", apvs))
}

func (s *UpdaterSuite) TestPlanTakesHighestOfVersionsUpgradeFromAccepts() {
	apvs := apvList("v1.9.0", "v1.10.0", "v1.2.0")
	for i := range apvs {
		acceptUpgradeFrom(&apvs[i], ">=1.0.0")
	}

	// Every version accepts v1.0.0: the highest by SemVer wins, not the first listed and not the greatest name.
	s.Equal([]string{"v1.10.0"}, computePlan(v1alpha2.UpdateModeAuto, "v1.0.0", apvs))
}

func (s *UpdaterSuite) TestPlanTakesHighestVersionUpgradeFromAccepts() {
	apvs := apvList("v1.3.0", "v1.6.0", "v1.5.0", "v1.4.0")
	acceptUpgradeFrom(&apvs[0], ">=1.0.0")
	acceptUpgradeFrom(&apvs[1], ">=1.5.0")
	acceptUpgradeFrom(&apvs[2], ">=1.0.0")
	acceptUpgradeFrom(&apvs[3], ">=1.0.0")

	// v1.6.0 does not accept v1.0.0, so the plan goes to v1.5.0, the highest version that does, and on to v1.6.0.
	s.Equal([]string{"v1.5.0", "v1.6.0"}, computePlan(v1alpha2.UpdateModeAuto, "v1.0.0", apvs))
}

func (s *UpdaterSuite) TestPlanSkipsVersionWithInvalidUpgradeFrom() {
	for _, constraint := range invalidConstraints {
		s.Run(constraint, func() {
			apvs := apvList("v1.2.4", "v1.2.5")
			acceptUpgradeFrom(&apvs[1], constraint)

			// The step of v1.2.3 and of v1.2.4 allows v1.2.5, its constraint allows nothing.
			s.Equal([]string{"v1.2.4"}, computePlan(v1alpha2.UpdateModeAuto, "v1.2.3", apvs))
		})
	}
}

func (s *UpdaterSuite) TestPlanStaysWithinMinorOnAutoPatchDespiteUpgradeFrom() {
	apvs := apvList("v1.2.4", "v1.3.0")
	acceptUpgradeFrom(&apvs[1], ">=1.2.0")

	s.Equal([]string{"v1.2.4"}, computePlan(v1alpha2.UpdateModeAutoPatch, "v1.2.3", apvs))
}

func (s *UpdaterSuite) TestPlanGoesThroughStepsAndUpgradeFromToNewestVersion() {
	// The step takes v1.0.0 to v1.1.1, v2.0.0 accepts v1.1.1, the step takes v2.0.0 to v2.1.0, v3.0.0 accepts v2.1.0.
	s.Equal([]string{"v1.1.1", "v2.0.0", "v2.1.0", "v3.0.0"}, computePlan(v1alpha2.UpdateModeAuto, "v1.0.0", history()))
}

func (s *UpdaterSuite) TestPlanReachesNewestVersionPastVersionLeadingNowhere() {
	apvs := apvList("v1.1.0", "v1.5.0", "v2.0.0")
	acceptUpgradeFrom(&apvs[1], ">=1.0.0")
	acceptUpgradeFrom(&apvs[2], "~1.1.0")

	// v1.5.0 is the newest version v1.0.0 steps to, but v2.0.0 accepts only v1.1.*.
	s.Equal([]string{"v1.1.0", "v2.0.0"}, computePlan(v1alpha2.UpdateModeAuto, "v1.0.0", apvs))
}

func (s *UpdaterSuite) TestPlanTakesNewestStepTowardsNewestVersion() {
	apvs := apvList("v1.3.0", "v1.4.0", "v2.0.0")
	acceptUpgradeFrom(&apvs[0], ">=1.0.0")
	acceptUpgradeFrom(&apvs[1], ">=1.0.0")
	acceptUpgradeFrom(&apvs[2], ">=1.3.0 <2.0.0")

	// v1.3.0 and v1.4.0 both accept v1.0.0 and lead to v2.0.0.
	s.Equal([]string{"v1.4.0", "v2.0.0"}, computePlan(v1alpha2.UpdateModeAuto, "v1.0.0", apvs))
}

func (s *UpdaterSuite) TestPlanEndsAtNewestReachableVersion() {
	apvs := apvList("v1.1.0", "v1.2.0", "v2.0.0")
	acceptUpgradeFrom(&apvs[2], "~1.5.0")

	// v2.0.0 accepts only v1.5.*, which the cluster does not have.
	s.Equal([]string{"v1.1.0", "v1.2.0"}, computePlan(v1alpha2.UpdateModeAuto, "v1.0.0", apvs))
}

func (s *UpdaterSuite) TestPlanNeverStepsOntoVersionWithInvalidUpgradeFromOnTheOnlyWay() {
	apvs := apvList("v1.0.5", "v1.1.0", "v2.0.0")
	acceptUpgradeFrom(&apvs[1], "not a constraint")
	acceptUpgradeFrom(&apvs[2], "~1.1.0")

	// Only v1.1.0 leads to v2.0.0, and its constraint allows nothing.
	s.Equal([]string{"v1.0.5"}, computePlan(v1alpha2.UpdateModeAuto, "v1.0.0", apvs))
}

func (s *UpdaterSuite) TestPlanDoesNotDependOnOrderOfVersions() {
	apvs := history()

	slices.Reverse(apvs)
	s.Equal([]string{"v1.1.1", "v2.0.0", "v2.1.0", "v3.0.0"}, computePlan(v1alpha2.UpdateModeAuto, "v1.0.0", apvs))

	rand.New(rand.NewPCG(1, 2)).Shuffle(len(apvs), func(i, j int) { apvs[i], apvs[j] = apvs[j], apvs[i] })
	s.Equal([]string{"v1.1.1", "v2.0.0", "v2.1.0", "v3.0.0"}, computePlan(v1alpha2.UpdateModeAuto, "v1.0.0", apvs))
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

func (s *UpdaterSuite) TestUpdateWritesAutomaticVersionUpgradeFromAccepts() {
	app := installedApp("v1.9.0", "")
	app.Spec.Update = &v1alpha1.ApplicationUpdate{Mode: string(v1alpha2.UpdateModeAuto)}
	apvs := apvList("v1.9.0", "v2.0.0")
	acceptUpgradeFrom(&apvs[1], ">=1.9.0")
	cli := s.client(app, apvs...)

	s.update(cli, app)

	s.Equal(&v1alpha1.ApplicationStatusUpdate{AutomaticVersion: "v2.0.0"}, s.get(cli, app).Status.Update)
}

func (s *UpdaterSuite) TestUpdateWritesHighestAutomaticVersionUpgradeFromAccepts() {
	app := installedApp("v1.0.0", "")
	app.Spec.Update = &v1alpha1.ApplicationUpdate{Mode: string(v1alpha2.UpdateModeAuto)}
	apvs := apvList("v1.0.0", "v1.9.0", "v1.10.0", "v1.2.0")
	for i := 1; i < len(apvs); i++ {
		acceptUpgradeFrom(&apvs[i], ">=1.0.0")
	}
	cli := s.client(app, apvs...)

	s.update(cli, app)

	s.Equal(&v1alpha1.ApplicationStatusUpdate{AutomaticVersion: "v1.10.0"}, s.get(cli, app).Status.Update)
}

func (s *UpdaterSuite) TestUpdatePlanStartsAfterAutomaticVersion() {
	app := installedApp("v1.2.3", "")
	cli := s.client(app, patchByPatch(apvList("v1.2.3", "v1.2.4", "v1.2.5", "v1.2.6"))...)

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

func (s *UpdaterSuite) TestUpdateShowsOnlyPlanOnManual() {
	app := installedApp("v1.2.3", "v1.2.5")
	app.Spec.Update = &v1alpha1.ApplicationUpdate{Mode: string(v1alpha2.UpdateModeManual)}
	cli := s.client(app, apvList("v1.2.5", "v1.3.0")...)

	s.update(cli, app)

	s.Equal(&v1alpha1.ApplicationStatusUpdate{Plan: []string{"v1.3.0"}}, s.get(cli, app).Status.Update)
}

func (s *UpdaterSuite) TestUpdateShowsOnlyPlanWithoutMode() {
	app := installedApp("v1.2.3", "v1.2.5")
	app.Spec.Update = nil
	cli := s.client(app, apvList("v1.2.5")...)

	s.update(cli, app)

	s.Equal(&v1alpha1.ApplicationStatusUpdate{Plan: []string{"v1.2.5"}}, s.get(cli, app).Status.Update)
}

func (s *UpdaterSuite) TestUpdateShowsWholePlanOnAutoPatch() {
	app := installedApp("v1.2.3", "")
	cli := s.client(app, apvList("v1.2.4", "v1.3.0")...)

	s.update(cli, app)

	// The automatic update stays within v1.2, the plan goes on to the next minor.
	s.Equal(&v1alpha1.ApplicationStatusUpdate{Plan: []string{"v1.3.0"}, AutomaticVersion: "v1.2.4"},
		s.get(cli, app).Status.Update)
}

func (s *UpdaterSuite) TestUpdateClearsBeforeInstall() {
	app := installedApp("v1.2.3", "v1.2.5")
	app.Status.CurrentVersion = nil
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

func (s *UpdaterSuite) TestFlowAutoInstallsEveryStepToNewestVersion() {
	app := installedApp("v1.0.0", "")
	app.Spec.Update = &v1alpha1.ApplicationUpdate{Mode: string(v1alpha2.UpdateModeAuto)}
	cli := s.client(app, history()...)

	s.Equal([]flowStep{
		{installed: "v1.0.0", automatic: "v1.1.1", plan: []string{"v2.0.0", "v2.1.0", "v3.0.0"}},
		{installed: "v1.1.1", automatic: "v2.0.0", plan: []string{"v2.1.0", "v3.0.0"}},
		{installed: "v2.0.0", automatic: "v2.1.0", plan: []string{"v3.0.0"}},
		{installed: "v2.1.0", automatic: "v3.0.0"},
		{installed: "v3.0.0", automatic: "v3.0.0"},
	}, s.walk(cli, app))
}

func (s *UpdaterSuite) TestFlowAutoBypassesVersionLeadingNowhere() {
	apvs := apvList("v1.0.0", "v1.1.0", "v1.5.0", "v2.0.0")
	acceptUpgradeFrom(&apvs[2], ">=1.0.0")
	acceptUpgradeFrom(&apvs[3], "~1.1.0")
	app := installedApp("v1.0.0", "")
	app.Spec.Update = &v1alpha1.ApplicationUpdate{Mode: string(v1alpha2.UpdateModeAuto)}
	cli := s.client(app, apvs...)

	// v1.5.0 accepts v1.0.0, but v2.0.0 accepts only v1.1.*.
	s.Equal([]flowStep{
		{installed: "v1.0.0", automatic: "v1.1.0", plan: []string{"v2.0.0"}},
		{installed: "v1.1.0", automatic: "v2.0.0"},
		{installed: "v2.0.0", automatic: "v2.0.0"},
	}, s.walk(cli, app))
}

func (s *UpdaterSuite) TestFlowAutoPatchInstallsPatchesAndWaitsForConfirmedSteps() {
	app := installedApp("v1.0.0", "")
	cli := s.client(app, history()...)

	// The automatic update takes the patches of the installed minor, the user confirms every other step of the plan.
	s.Equal([]flowStep{
		{installed: "v1.0.0", automatic: "v1.0.2", plan: []string{"v1.1.1", "v2.0.0", "v2.1.0", "v3.0.0"}},
		{installed: "v1.0.2", automatic: "v1.0.2", plan: []string{"v1.1.1", "v2.0.0", "v2.1.0", "v3.0.0"}},
		{installed: "v1.1.1", automatic: "v1.1.1", plan: []string{"v2.0.0", "v2.1.0", "v3.0.0"}},
		{installed: "v2.0.0", automatic: "v2.0.1", plan: []string{"v2.1.0", "v3.0.0"}},
		{installed: "v2.0.1", automatic: "v2.0.1", plan: []string{"v2.1.0", "v3.0.0"}},
		{installed: "v2.1.0", automatic: "v2.1.0", plan: []string{"v3.0.0"}},
		{installed: "v3.0.0", automatic: "v3.0.0"},
	}, s.walk(cli, app))
}

func (s *UpdaterSuite) TestFlowManualInstallsOnlyConfirmedSteps() {
	app := installedApp("v1.0.0", "")
	app.Spec.Update = &v1alpha1.ApplicationUpdate{Mode: string(v1alpha2.UpdateModeManual)}
	cli := s.client(app, history()...)

	s.Equal([]flowStep{
		{installed: "v1.0.0", plan: []string{"v1.1.1", "v2.0.0", "v2.1.0", "v3.0.0"}},
		{installed: "v1.1.1", plan: []string{"v2.0.0", "v2.1.0", "v3.0.0"}},
		{installed: "v2.0.0", plan: []string{"v2.1.0", "v3.0.0"}},
		{installed: "v2.1.0", plan: []string{"v3.0.0"}},
		{installed: "v3.0.0"},
	}, s.walk(cli, app))
}

func (s *UpdaterSuite) TestFlowNeverInstallsVersionWithInvalidUpgradeFrom() {
	tests := []struct {
		mode v1alpha2.UpdateMode
		want []flowStep
	}{
		{
			mode: v1alpha2.UpdateModeAuto,
			want: []flowStep{
				{installed: "v1.0.0", automatic: "v1.1.1", plan: []string{"v1.2.0"}},
				{installed: "v1.1.1", automatic: "v1.2.0"},
				{installed: "v1.2.0", automatic: "v1.2.0"},
			},
		},
		{
			mode: v1alpha2.UpdateModeAutoPatch,
			want: []flowStep{
				{installed: "v1.0.0", automatic: "v1.0.2", plan: []string{"v1.1.1", "v1.2.0"}},
				{installed: "v1.0.2", automatic: "v1.0.2", plan: []string{"v1.1.1", "v1.2.0"}},
				{installed: "v1.1.1", automatic: "v1.1.1", plan: []string{"v1.2.0"}},
				{installed: "v1.2.0", automatic: "v1.2.0"},
			},
		},
		{
			mode: v1alpha2.UpdateModeManual,
			want: []flowStep{
				{installed: "v1.0.0", plan: []string{"v1.1.1", "v1.2.0"}},
				{installed: "v1.1.1", plan: []string{"v1.2.0"}},
				{installed: "v1.2.0"},
			},
		},
	}

	for _, tt := range tests {
		s.Run(string(tt.mode), func() {
			apvs := history()
			// Every version from v2.0.0 on is reached only through v2.0.0.
			acceptUpgradeFrom(&apvs[6], "not a constraint")
			app := installedApp("v1.0.0", "")
			app.Spec.Update = &v1alpha1.ApplicationUpdate{Mode: string(tt.mode)}
			cli := s.client(app, apvs...)

			s.Equal(tt.want, s.walk(cli, app))
		})
	}
}

func (s *UpdaterSuite) TestFlowWaitsUntilEachStepIsReady() {
	app := installedApp("v1.0.0", "")
	app.Spec.Update = &v1alpha1.ApplicationUpdate{Mode: string(v1alpha2.UpdateModeAuto)}
	cli := s.client(app, history()...)

	s.update(cli, app)
	s.install(cli, app, "v1.1.1")
	s.report(cli, app, status.ConditionReady, metav1.ConditionFalse)
	s.update(cli, app)

	s.Equal(&v1alpha1.ApplicationStatusUpdate{AutomaticVersion: "v1.1.1", Plan: []string{"v2.0.0", "v2.1.0", "v3.0.0"}},
		s.get(cli, app).Status.Update)

	s.report(cli, app, status.ConditionReady, metav1.ConditionTrue)
	s.update(cli, app)

	s.Equal(&v1alpha1.ApplicationStatusUpdate{AutomaticVersion: "v2.0.0", Plan: []string{"v2.1.0", "v3.0.0"}},
		s.get(cli, app).Status.Update)
}

func (s *UpdaterSuite) TestVersionTakesNewerAutomaticVersion() {
	app := installedApp("v1.2.3", "v1.3.0")

	s.Equal("v1.3.0", app.TargetVersion())
}

func (s *UpdaterSuite) TestVersionKeepsNewerSpec() {
	app := installedApp("v1.2.3", "v1.3.0")
	app.Spec.PackageVersion = "v1.4.0"

	s.Equal("v1.4.0", app.TargetVersion())
}

func (s *UpdaterSuite) TestVersionIgnoresAutomaticVersionOnManual() {
	app := installedApp("v1.2.3", "v1.3.0")
	app.Spec.Update = &v1alpha1.ApplicationUpdate{Mode: string(v1alpha2.UpdateModeManual)}

	s.Equal("v1.2.3", app.TargetVersion())
}

func (s *UpdaterSuite) TestVersionIgnoresAutomaticVersionWithoutMode() {
	app := installedApp("v1.2.3", "v1.3.0")
	app.Spec.Update = &v1alpha1.ApplicationUpdate{}

	s.Equal("v1.2.3", app.TargetVersion())
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

// flowStep is one reconcile of a walk: the version installed when it ran, and the automatic version and the plan it
// left in status.update.
type flowStep struct {
	installed string
	automatic string
	plan      []string
}

// walk reconciles the application until nothing is left to install, the way a cluster runs it: after every reconcile
// the engine installs the version the application targets and reports it ready, and when that is the installed one,
// the user confirms the first version of the plan by setting it in spec.packageVersion.
func (s *UpdaterSuite) walk(cli client.Client, app *v1alpha1.Application) []flowStep {
	var steps []flowStep
	for range 20 {
		s.update(cli, app)

		got := s.get(cli, app)
		step := flowStep{installed: got.Status.CurrentVersion.Version}
		if got.Status.Update != nil {
			step.automatic, step.plan = got.Status.Update.AutomaticVersion, got.Status.Update.Plan
		}

		steps = append(steps, step)

		if got.TargetVersion() == step.installed {
			if len(step.plan) == 0 {
				return steps
			}

			s.confirm(cli, app, step.plan[0])
		}

		s.install(cli, app, s.get(cli, app).TargetVersion())
	}

	s.FailNow("the application never settles", "%v", steps)

	return nil
}

// confirm sets the version in spec.packageVersion, the way a user confirms an update.
func (s *UpdaterSuite) confirm(cli client.Client, app *v1alpha1.Application, version string) {
	got := s.get(cli, app)
	got.Spec.PackageVersion = version
	s.Require().NoError(cli.Update(context.Background(), got))
}

// install reports the version installed and ready, as the status service does once the engine activates it.
func (s *UpdaterSuite) install(cli client.Client, app *v1alpha1.Application, version string) {
	got := s.get(cli, app)
	patch := client.MergeFrom(got.DeepCopy())
	got.Status.CurrentVersion = &v1alpha1.ApplicationStatusVersion{Version: version}
	setCondition(got, status.ConditionUpdateInstalled, metav1.ConditionTrue)
	setCondition(got, status.ConditionReady, metav1.ConditionTrue)
	s.Require().NoError(cli.Status().Patch(context.Background(), got, patch))
}

// report sets a condition of the application, as the status service does.
func (s *UpdaterSuite) report(cli client.Client, app *v1alpha1.Application, condType string, condStatus metav1.ConditionStatus) {
	got := s.get(cli, app)
	patch := client.MergeFrom(got.DeepCopy())
	setCondition(got, condType, condStatus)
	s.Require().NoError(cli.Status().Patch(context.Background(), got, patch))
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

// patchByPatch limits the upgrade from every version to one patch.
func patchByPatch(apvs []v1alpha1.ApplicationPackageVersion) []v1alpha1.ApplicationPackageVersion {
	for i := range apvs {
		limitUpgrade(&apvs[i], v1alpha1.PackageVersionCompatibilityRuleStep{Patches: ptr.To[int32](1)})
	}

	return apvs
}

// limitUpgrade sets the rules of the upgrade from the package version.
func limitUpgrade(apv *v1alpha1.ApplicationPackageVersion, step v1alpha1.PackageVersionCompatibilityRuleStep) {
	apv.Status.PackageMetadata = &v1alpha1.ApplicationPackageVersionStatusMetadata{
		Compatibility: &v1alpha1.PackageVersionCompatibilityRules{
			Upgrade: &v1alpha1.PackageVersionUpgradeRule{Step: step},
		},
	}
}

// acceptUpgradeFrom sets the constraint on the versions that may be upgraded to the package version.
func acceptUpgradeFrom(apv *v1alpha1.ApplicationPackageVersion, constraint string) {
	apv.Status.PackageMetadata = &v1alpha1.ApplicationPackageVersionStatusMetadata{
		Compatibility: &v1alpha1.PackageVersionCompatibilityRules{
			Upgrade: &v1alpha1.PackageVersionUpgradeRule{From: constraint},
		},
	}
}

// invalidConstraints are upgrade.from values that do not parse as a SemVer constraint.
var invalidConstraints = []string{"not a constraint", ">=", ">=1.0.0 <", "1.2.3.4", "=>>1.0.0", ">=1.0.0 ||", "v1.x.y", " "}

// history is the package history the flow tests walk: the step allows any patch and the next minor, v2.0.0 accepts
// v1.1.0 and the later v1 versions, v3.0.0 accepts only v2.1.*.
func history() []v1alpha1.ApplicationPackageVersion {
	apvs := apvList("v1.0.0", "v1.0.1", "v1.0.2", "v1.1.0", "v1.1.1", "v1.2.0", "v2.0.0", "v2.0.1", "v2.1.0", "v3.0.0")
	acceptUpgradeFrom(&apvs[6], ">=1.1.0 <2.0.0")
	acceptUpgradeFrom(&apvs[9], "~2.1.0")

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
