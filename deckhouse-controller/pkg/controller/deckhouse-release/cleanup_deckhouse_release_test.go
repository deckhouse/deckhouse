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
