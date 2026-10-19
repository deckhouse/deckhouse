// Copyright 2026 Flant JSC
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

package gate_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	et "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/enginetest"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

// gateSuite covers the final gate's settings check as the package Reconciler drives it.
type gateSuite struct {
	suite.Suite
}

func TestGate(t *testing.T) {
	suite.Run(t, new(gateSuite))
}

// F62: a settings check that fails to decide at the final gate (ValidateSettings returns an error:
// the settings-check hook could not reach the cluster) blocks the update as SettingsInvalid with
// the error's message, is not memoized and is retried with backoff on the Reconciler's timer.
func (s *gateSuite) TestF62TransientSettingsCheckErrorIsRetried() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.ApplyModule(et.Module("mod", "v1.0.0"), true)
	k.WaitReport("mod", "v1 installed", func(r model.Report) bool { return et.Running(r, "v1.0.0") })

	k.Hooks.SettingsError("mod", errors.New("settings check hook: connection refused"))
	k.ApplyModule(et.Module("mod", "v2.0.0"), true)
	k.WaitReport("mod", "v2 blocked by the failing check", func(r model.Report) bool {
		return r.Candidate.Blocked == model.ReasonSettingsInvalid && strings.Contains(r.Candidate.Message, "connection refused")
	})

	mark := k.Mark()
	s.Equal(retry.Min, k.AdvanceToNextTimer("the first retry"))
	k.WaitLog(mark, "validate:mod@v2.0.0")
	k.WaitArmed("the retried check failed")
	k.WaitReport("mod", "still blocked", func(r model.Report) bool { return r.Candidate.Blocked == model.ReasonSettingsInvalid })

	k.Hooks.SettingsError("mod", nil)
	s.Equal(2*retry.Min, k.AdvanceToNextTimer("the second retry, backed off"))
	k.WaitReport("mod", "v2 activated after the check recovered", func(r model.Report) bool { return et.Running(r, "v2.0.0") })
}

// A candidate blocked by a verdict is re-checked on a new input: new settings run the final gate
// again; the same settings keep the memoized verdict.
func (s *gateSuite) TestBlockedCandidateIsReevaluatedOnTheNextInput() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.ApplyModule(et.Module("mod", "v1.0.0"), true)
	k.WaitReport("mod", "v1 installed", func(r model.Report) bool { return et.Running(r, "v1.0.0") })

	k.Loader.Set("mod", "v2.0.0", et.Spec{Validate: func(v addonutils.Values) error {
		if v["a"] == 1 {
			return errors.New("a must not be 1")
		}

		return nil
	}})

	v2 := et.Module("mod", "v2.0.0")
	k.ApplyModule(et.WithSettings(v2, addonutils.Values{"a": 1}), true)
	k.WaitReport("mod", "v2 blocked", func(r model.Report) bool {
		return r.Candidate.Blocked == model.ReasonSettingsInvalid && strings.Contains(r.Candidate.Message, "a must not be 1")
	})

	mark := k.Mark()
	k.ApplyModule(et.WithSettings(v2, addonutils.Values{"a": 1}), true)
	k.ApplyModule(et.WithSettings(v2, addonutils.Values{"a": 2}), true)
	k.WaitReport("mod", "v2 activated", func(r model.Report) bool { return et.Running(r, "v2.0.0") })
	beforeActivation := k.Log(mark)[:k.Index(mark, "add-node:mod@v2.0.0")]
	checks := 0
	for _, e := range beforeActivation {
		if e == "validate:mod@v2.0.0" {
			checks++
		}
	}
	s.Equal(1, checks, "checked once, for the new settings; the repeated ones kept the verdict: %v", beforeActivation)
}

// A first install is not stopped by settings: a module nobody enabled, whose settings its schema
// rejects, is activated, gets its node and the decision disabled, and never runs; its Current
// reports the rejection.
func (s *gateSuite) TestFirstInstallIsNotStoppedBySettings() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.Loader.Set("mod", "v1.0.0", et.Spec{Validate: func(addonutils.Values) error {
		return errors.New("licenseKey is required")
	}})

	k.ApplyModule(et.Module("mod", "v1.0.0"), false)

	r := k.WaitReport("mod", "activated and disabled", func(r model.Report) bool {
		return r.CurrentVersion == "v1.0.0" && r.Current.Decision != nil && !r.Current.Decision.Enabled &&
			et.False(r.Current.Configured, model.ReasonSettingsInvalid)
	})

	s.Equal(model.CandidateReport{}, r.Candidate)
	s.Equal(-1, k.Index(0, "upgrade:mod"), "a module without accepted settings must not run: %v", k.Log(0))
}
