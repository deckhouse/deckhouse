package deckhouse_release

import (
	"context"

	"github.com/stretchr/testify/require"
)

func (suite *ControllerTestSuite) TestCleanupDeckhouseRelease() {
	ctx := context.Background()

	var initValues = `{
	"global": {
		"modulesImages": {
			"registry": {
				"base": "my.registry.com/deckhouse"
			}
		}
	},
	"deckhouse":{
		"bundle": "Default"
	}
}`

	suite.Run("Have a few Deployed Releases", func() {
		suite.setupController("have-a-few-deployed-releases.yaml", initValues, embeddedMUP)
		err := suite.ctr.cleanupDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Have 15 Superseded Releases", func() {
		suite.setupController("have-15-superseded-releases.yaml", initValues, embeddedMUP)
		err := suite.ctr.cleanupDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Have 1 Deployed release and 5 Outdated Releases", func() {
		suite.setupController("have-1-deployed-release-and-5-outdated-releases.yaml", initValues, embeddedMUP)
		err := suite.ctr.cleanupDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Releases from real cluster", func() {
		suite.setupController("releases-from-real-cluster.yaml", initValues, embeddedMUP)
		err := suite.ctr.cleanupDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Has Deployed releases", func() {
		suite.setupController("has-deployed-releases.yaml", initValues, embeddedMUP)
		err := suite.ctr.cleanupDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})
}
