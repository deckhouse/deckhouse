// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_mocks "github.com/deckhouse/deckhouse/dhctl/pkg/config/registrymocks"
)

type PreflightChecksTestSuite struct {
	suite.Suite
	checker Checker
}

func (s *PreflightChecksTestSuite) SetupSuite() {
	s.checker = NewChecker(nil, nil, nil, nil)
}

func (s *PreflightChecksTestSuite) SetupTest() {
	app.AppVersion = "v1.50.6"
	s.checker.installConfig = &config.DeckhouseInstaller{
		Registry: registry_mocks.ConfigBuilder(
			registry_mocks.WithImagesRepo("registry.deckhouse.io/deckhouse/ce"),
			registry_mocks.WithSchemeHTTPS(),
		),
		DevBranch: "pr1111",
	}
}

func TestPreflightChecks(t *testing.T) {
	suite.Run(t, new(PreflightChecksTestSuite))
}
