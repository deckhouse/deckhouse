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

package modules_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// ModuleSettingsTestSuite covers how a module converts and applies versioned settings.
type ModuleSettingsTestSuite struct {
	suite.Suite

	module *modules.Module
}

func TestModuleSettingsTestSuite(t *testing.T) {
	suite.Run(t, new(ModuleSettingsTestSuite))
}

func (s *ModuleSettingsTestSuite) SetupTest() {
	converter, err := conversion.NewConverter(map[int][]string{2: {".b = .a", "del(.a)"}})
	s.Require().NoError(err)

	s.module, err = modules.NewModuleByConfig("test", &modules.Config{
		Definition:   modules.Definition{Name: "test", Version: "v1.0.0"},
		ConfigSchema: []byte("type: object\nproperties:\n  b:\n    type: string\n"),
		Conversions:  converter,
	}, log.NewNop())
	s.Require().NoError(err)
}

func (s *ModuleSettingsTestSuite) TestApplyConvertsOlderVersion() {
	s.Require().NoError(s.module.ApplySettings(1, addonutils.Values{"a": "value"}))
	s.Equal(addonutils.Values{"b": "value"}, s.module.GetSettings())
}

func (s *ModuleSettingsTestSuite) TestVersionAboveLatestIsRejected() {
	res, err := s.module.ValidateSettings(context.Background(), 3, addonutils.Values{"b": "value"})
	s.Require().NoError(err)
	s.False(res.Valid)
	s.Contains(res.Message, "settings version 3 is newer than the latest 2")

	s.Require().ErrorContains(s.module.ApplySettings(3, addonutils.Values{"b": "value"}), "newer than the latest")
}
