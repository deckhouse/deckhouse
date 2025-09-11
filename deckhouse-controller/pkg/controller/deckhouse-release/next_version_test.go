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
	rc         *DeckhouseReleaseFetcher
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
		"v2.0.0",
		"v2.0.1",
		"v2.0.5",
		"v2.1.2",
		"v2.1.12",
		"v2.5.1",
	}, nil)

	suite.ctr, suite.kubeClient = setupFakeController(suite.T(), "", initValues, embeddedMUP)
	cfg := &DeckhouseReleaseFetcherConfig{
		registryClient: dependency.TestDC.CRClient,
		moduleManager:  suite.ctr.moduleManager,
		logger:         suite.ctr.logger,
	}

	suite.rc = NewDeckhouseReleaseFetcher(cfg)
}

func (suite *ReleaseTestSuite) TestCheckRelease() {
	check := func(name string, actual, target string, vers []*semver.Version) {
		suite.Run(name, func() {
			actual, _ := semver.NewVersion(actual)
			target, _ := semver.NewVersion(target)
			v, err := suite.rc.getNewVersions(
				context.Background(),
				actual,
				target,
			)
			require.NoError(suite.T(), err)

			if !cmp.Equal(v, vers) {
				suite.T().Fatalf("version is not equal: %v", cmp.Diff(v, target))
			}
		})
	}

	check("Patch", "1.31.0", "1.31.1", []*semver.Version{semver.MustParse("1.31.1")})

	check("Minor", "1.31.0", "1.32.3", []*semver.Version{
		semver.MustParse("1.32.3")})

	check("Last Minor", "1.31.0", "1.33.1", []*semver.Version{
		semver.MustParse("1.32.3"),
		semver.MustParse("1.33.1")})

	check("Major", "1.31.0", "2.0.5", []*semver.Version{
		semver.MustParse("1.32.3"),
		semver.MustParse("1.33.1"),
		semver.MustParse("2.0.5"),
	})

	check("Last Major Minor", "1.31.0", "2.1.12", []*semver.Version{
		semver.MustParse("1.32.3"),
		semver.MustParse("1.33.1"),
		semver.MustParse("2.0.5"),
		semver.MustParse("2.1.12"),
	})

	check("Last Minor is not equal to target", "1.31.0", "1.33.0", []*semver.Version{
		semver.MustParse("1.32.3"),
		semver.MustParse("1.33.0"),
	})

	check("Last Leap Minor", "1.31.0", "2.5.1", []*semver.Version{
		semver.MustParse("1.32.3"),
		semver.MustParse("1.33.1"),
		semver.MustParse("2.0.5"),
		semver.MustParse("2.1.12"),
		semver.MustParse("2.5.1"),
	})
}
