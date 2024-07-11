package deckhouse_release

import (
	"context"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

func (suite *ControllerTestSuite) TestUpdateByImageHash() {
	ctx := context.Background()

	suite.Run("No new deckhouse image", func() {
		dependency.TestDC.CRClient.DigestMock.Set(func(_ string) (s1 string, err error) {
			return "sha256:d57f01a88e54f863ff5365c989cb4e2654398fa274d46389e0af749090b862d1", nil
		})

		suite.setupController("dev-no-new-deckhouse-image.yaml", initValues, embeddedMUP)
		pods, err := suite.ctr.getDeckhousePods(ctx)
		require.NoError(suite.T(), err)

		err = suite.ctr.tagUpdate(ctx, pods)
		require.NoError(suite.T(), err)
	})

	suite.Run("Have new deckhouse image", func() {
		dependency.TestDC.CRClient.DigestMock.Set(func(_ string) (s1 string, err error) {
			return "sha256:123456", nil
		})

		ds := &helpers.DeckhouseSettings{}
		ds.Update.Mode = embeddedMUP.Update.Mode
		ds.Update.Windows = embeddedMUP.Update.Windows
		ds.Update.DisruptionApprovalMode = "Auto"

		suite.setupControllerSettings("dev-have-new-deckhouse-image.yaml", initValues, ds)
		pods, err := suite.ctr.getDeckhousePods(ctx)
		require.NoError(suite.T(), err)

		err = suite.ctr.tagUpdate(ctx, pods)
		require.NoError(suite.T(), err)
	})

}
