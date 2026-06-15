/*
Copyright 2024 Flant JSC

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

package deckhouse_release

import (
	"context"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
)

func blockOnAlertsDeckhouseSettings(mup *v1alpha2.ModuleUpdatePolicySpec, enabled bool, severity int) *helpers.DeckhouseSettings {
	ds := newDeckhouseSettings(mup)
	ds.Update.BlockOnAlerts.Enabled = enabled
	ds.Update.BlockOnAlerts.Severity = severity
	return ds
}

func (suite *ControllerTestSuite) seedClusterAlert(name string, severityLevel string) {
	suite.T().Helper()

	alert := makeClusterAlert(name, severityLevel)
	require.NoError(suite.T(), suite.Client().Create(context.Background(), alert))
}

func (suite *ControllerTestSuite) TestBlockOnAlerts() {
	ctx := context.Background()

	suite.Run("High severity alert blocks release", func() {
		ds := blockOnAlertsDeckhouseSettings(embeddedMUP, true, 4)
		suite.setupControllerSettings("block-on-alerts-high-severity-blocks-release.yaml", initValues, ds)
		suite.seedClusterAlert("blocking-alert", "7")

		dr := suite.getDeckhouseRelease("v1.25.1")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)

		dr = suite.getDeckhouseRelease("v1.25.1")
		require.Equal(suite.T(), v1alpha1.DeckhouseReleasePhasePending, dr.Status.Phase)
	})

	suite.Run("Low severity alert does not block release", func() {
		ds := blockOnAlertsDeckhouseSettings(embeddedMUP, true, 4)
		suite.setupControllerSettings("block-on-alerts-low-severity-does-not-block.yaml", initValues, ds)
		suite.seedClusterAlert("safe-alert", "2")

		dr := suite.getDeckhouseRelease("v1.25.1")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)

		dr = suite.getDeckhouseRelease("v1.25.1")
		require.Equal(suite.T(), v1alpha1.DeckhouseReleasePhaseDeployed, dr.Status.Phase)
	})

	suite.Run("Severity equals threshold blocks release", func() {
		ds := blockOnAlertsDeckhouseSettings(embeddedMUP, true, 4)
		suite.setupControllerSettings("block-on-alerts-severity-equals-threshold.yaml", initValues, ds)
		suite.seedClusterAlert("alert-eq", "4")

		dr := suite.getDeckhouseRelease("v1.25.1")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)

		dr = suite.getDeckhouseRelease("v1.25.1")
		require.Equal(suite.T(), v1alpha1.DeckhouseReleasePhasePending, dr.Status.Phase)
	})

	suite.Run("BlockOnAlerts disabled does not block release", func() {
		ds := blockOnAlertsDeckhouseSettings(embeddedMUP, false, 4)
		suite.setupControllerSettings("block-on-alerts-disabled.yaml", initValues, ds)
		suite.seedClusterAlert("blocking-alert", "7")

		dr := suite.getDeckhouseRelease("v1.25.1")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)

		dr = suite.getDeckhouseRelease("v1.25.1")
		require.Equal(suite.T(), v1alpha1.DeckhouseReleasePhaseDeployed, dr.Status.Phase)
	})

	suite.Run("Forced release bypasses block on alerts", func() {
		ds := blockOnAlertsDeckhouseSettings(embeddedMUP, true, 4)
		suite.setupControllerSettings("block-on-alerts-forced-release.yaml", initValues, ds)
		suite.seedClusterAlert("blocking-alert", "7")

		dr := suite.getDeckhouseRelease("v1.31.1")
		_, err := suite.ctr.createOrUpdateReconcile(ctx, dr)
		require.NoError(suite.T(), err)

		dr = suite.getDeckhouseRelease("v1.31.1")
		require.Equal(suite.T(), v1alpha1.DeckhouseReleasePhaseDeployed, dr.Status.Phase)
	})
}
