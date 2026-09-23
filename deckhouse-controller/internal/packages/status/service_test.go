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

package status_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

// pkgName is the tracked package every case operates on.
const pkgName = "test-module"

// SettingsSuite covers the SettingsChanged marker UpdateSettings puts on ConditionConfigured.
type SettingsSuite struct {
	suite.Suite

	svc *status.Service
}

// TestSettingsSuite runs SettingsSuite.
func TestSettingsSuite(t *testing.T) {
	suite.Run(t, new(SettingsSuite))
}

// SetupTest registers a fresh package status.
func (s *SettingsSuite) SetupTest() {
	s.svc = status.NewService()
	s.svc.NewStatus(pkgName)
}

// TearDownTest shuts the notification queues down; Shutdown would wait on a resync never started.
func (s *SettingsSuite) TearDownTest() {
	s.svc.AppQueue().ShutDown()
	s.svc.ModuleQueue().ShutDown()
}

// configured returns the current ConditionConfigured of the package.
func (s *SettingsSuite) configured() status.Condition {
	for _, cond := range s.svc.GetStatus(pkgName).Conditions {
		if cond.Type == status.ConditionConfigured {
			return cond
		}
	}

	s.FailNow("condition Configured not found")

	return status.Condition{}
}

// applied stores settings and runs them through as the Run task would.
func (s *SettingsSuite) applied(settings addonutils.Values) {
	s.svc.UpdateSettings(pkgName, settings)
	s.svc.SetConditionTrue(pkgName, status.ConditionConfigured)
}

// TestFirstSettingsLeaveConfigured checks the first settings after NewStatus set no marker.
func (s *SettingsSuite) TestFirstSettingsLeaveConfigured() {
	s.svc.UpdateSettings(pkgName, addonutils.Values{"a": 1})

	s.Equal(metav1.ConditionUnknown, s.configured().Status)
}

// TestChangedSettingsSetMarker checks changed settings reset Configured to False/SettingsChanged.
func (s *SettingsSuite) TestChangedSettingsSetMarker() {
	s.applied(addonutils.Values{"a": 1})

	s.svc.UpdateSettings(pkgName, addonutils.Values{"a": 2})

	cond := s.configured()
	s.Equal(metav1.ConditionFalse, cond.Status)
	s.Equal(status.ConditionReasonSettingsChanged, cond.Reason)
}

// TestUnchangedSettingsKeepConfigured checks a re-Configure with the same settings sets no marker.
func (s *SettingsSuite) TestUnchangedSettingsKeepConfigured() {
	s.applied(addonutils.Values{"a": 1})

	s.svc.UpdateSettings(pkgName, addonutils.Values{"a": 1})

	s.Equal(metav1.ConditionTrue, s.configured().Status)
}

// TestRunClearsMarker checks the Run after a settings change sets Configured True.
func (s *SettingsSuite) TestRunClearsMarker() {
	s.applied(addonutils.Values{"a": 1})

	s.applied(addonutils.Values{"a": 2})

	s.Equal(metav1.ConditionTrue, s.configured().Status)
}

// TestNewStatusExemptsVersionChange checks settings after a status reset set no marker.
func (s *SettingsSuite) TestNewStatusExemptsVersionChange() {
	s.applied(addonutils.Values{"a": 1})
	s.svc.NewStatus(pkgName)

	s.svc.UpdateSettings(pkgName, addonutils.Values{"a": 2})

	s.Equal(metav1.ConditionUnknown, s.configured().Status)
}
