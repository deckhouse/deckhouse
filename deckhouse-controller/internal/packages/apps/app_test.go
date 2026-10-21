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

package apps_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// convertedSettingsSchema is the latest (version 2) settings schema: version 1 named the field a.
const convertedSettingsSchema = `
type: object
additionalProperties: false
required: [b]
properties:
  b:
    type: string
`

// AppSettingsTestSuite covers how an application converts and applies versioned settings.
type AppSettingsTestSuite struct {
	suite.Suite

	app *apps.Application
}

func TestAppSettingsTestSuite(t *testing.T) {
	suite.Run(t, new(AppSettingsTestSuite))
}

func (s *AppSettingsTestSuite) SetupTest() {
	converter, err := conversion.NewConverter(map[int][]string{
		2: {`if .a != null then .b = .a else .b = "default" end`, "del(.a)"},
	})
	s.Require().NoError(err)

	s.app = s.newApp(converter)
}

// newApp builds an application with the converted settings schema and converter, nil for none.
func (s *AppSettingsTestSuite) newApp(converter *conversion.Converter) *apps.Application {
	app, err := apps.NewAppByConfig("tenant.app", &apps.Config{
		Definition:   apps.Definition{Name: "app", Version: "v1.0.0"},
		ConfigSchema: []byte(convertedSettingsSchema),
		Conversions:  converter,
	}, log.NewNop())
	s.Require().NoError(err)

	return app
}

func (s *AppSettingsTestSuite) TestValidateConvertsOlderVersion() {
	res, err := s.app.ValidateSettings(context.Background(), 1, addonutils.Values{"a": "value"})
	s.Require().NoError(err)
	s.True(res.Valid, res.Message)
}

func (s *AppSettingsTestSuite) TestValidateRejectsOlderShapeAtLatest() {
	res, err := s.app.ValidateSettings(context.Background(), 2, addonutils.Values{"a": "value"})
	s.Require().NoError(err)
	s.False(res.Valid, "version 2 settings are not converted, so the version 1 field is unknown")
}

func (s *AppSettingsTestSuite) TestApplyStoresConvertedSettings() {
	s.Require().NoError(s.app.ApplySettings(1, addonutils.Values{"a": "value"}))
	s.Equal(addonutils.Values{"b": "value"}, s.app.GetSettings())
}

func (s *AppSettingsTestSuite) TestZeroVersionIsReadAsOne() {
	s.Require().NoError(s.app.ApplySettings(0, addonutils.Values{"a": "value"}))
	s.Equal(addonutils.Values{"b": "value"}, s.app.GetSettings())
}

func (s *AppSettingsTestSuite) TestAbsentSettingsRunTheChain() {
	s.Require().NoError(s.app.ApplySettings(0, nil))
	s.Equal(addonutils.Values{"b": "default"}, s.app.GetSettings())
}

func (s *AppSettingsTestSuite) TestLatestSettingsVersion() {
	s.Equal(2, s.app.LatestSettingsVersion())
	s.Equal(0, s.newApp(nil).LatestSettingsVersion(), "without conversions the version is unversioned")
}

func (s *AppSettingsTestSuite) TestVersionIgnoredWithoutConverter() {
	app := s.newApp(nil)

	s.Require().NoError(app.ApplySettings(5, addonutils.Values{"b": "value"}))
	s.Equal(addonutils.Values{"b": "value"}, app.GetSettings())
}

func (s *AppSettingsTestSuite) TestVersionAboveLatestIsRejected() {
	res, err := s.app.ValidateSettings(context.Background(), 3, addonutils.Values{"b": "value"})
	s.Require().NoError(err)
	s.False(res.Valid)
	s.Contains(res.Message, "settings version 3 is newer than the latest 2")

	s.Require().ErrorContains(s.app.ApplySettings(3, addonutils.Values{"b": "value"}), "newer than the latest")
}
