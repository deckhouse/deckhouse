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
	"net/http"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
)

func TestReleaseTestSuite(t *testing.T) {
	suite.Run(t, new(ReleaseTestSuite))
}

type ReleaseTestSuite struct {
	suite.Suite

	kubeClient client.Client
	ctr        *deckhouseReleaseReconciler
	rc         *DeckhouseReleaseChecker
}

func (suite *ReleaseTestSuite) SetupSuite() {
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
}

func (suite *ReleaseTestSuite) SetupSubTest() {
	dependency.TestDC.CRClient = cr.NewClientMock(suite.T())
	dependency.TestDC.HTTPClient.DoMock.
		Expect(&http.Request{}).
		Return(&http.Response{
			StatusCode: http.StatusOK,
		}, nil)

	dependency.TestDC.CRClient.ListTagsMock.Return([]string{
		"v1.31.0",
		"v1.31.1",
		"v1.32.0",
		"v1.32.1",
		"v1.32.2",
		"v1.32.3",
		"v1.33.0",
		"v1.33.1",
	}, nil)

	suite.ctr, suite.kubeClient = setupFakeController(suite.T(), "", initValues, embeddedMUP)
	var err error
	suite.rc, err = NewDeckhouseReleaseChecker([]cr.Option{}, suite.ctr.logger, suite.ctr.dc,
		suite.ctr.moduleManager, "", "")
	require.NoError(suite.T(), err)
}

func (suite *ReleaseTestSuite) TestCheckRelease() {
	check := func(name string, actual, target string, fail bool) {
		suite.Run(name, func() {
			actual, _ := semver.NewVersion(actual)
			target, _ := semver.NewVersion(target)
			v, err := suite.rc.nextVersion(
				actual,
				target,
			)
			require.NoError(suite.T(), err)

			if !cmp.Equal(v, target) && !fail {
				suite.T().Fatalf("version is not equal: %v", cmp.Diff(v, target))
			}
		})
	}

	check("Patch", "1.31.0", "1.31.1", false)
	check("Minor", "1.31.0", "1.32.3", false)
	check("Last Minor", "1.31.0", "1.32.3", false)
	check("Fail Update", "1.31.0", "1.33.0", true)
}
