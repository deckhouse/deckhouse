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

package hooks_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	et "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/enginetest"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
)

// hooksSuite covers scenarios J at the engine level: the hook bindings as Current drives them.
type hooksSuite struct {
	suite.Suite
}

func TestHooks(t *testing.T) {
	suite.Run(t, new(hooksSuite))
}

// J1: OnStartup runs once per enable: a run retried after a BeforeHelm failure does not run it
// again; a disable and a re-enable do.
func (s *hooksSuite) TestJ1OnStartupOncePerEnable() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	mark := k.Mark()
	k.Faults.FailTimes("run:mod:beforeHelm", 1, errors.New("boom"))
	k.ApplyModule(et.Module("mod", "v1.0.0"), true)

	r := k.WaitReport("mod", "the run fails at beforeHelm", func(r model.Report) bool {
		return et.False(r.Current.HooksProcessed, model.ReasonHookFailed)
	})
	s.Unknown(r.Current.ManifestsApplied, "apply not reached on the first revision")

	k.AdvanceToNextTimer("the run's retry")
	k.WaitReport("mod", "the retry succeeds", func(r model.Report) bool { return et.Running(r, "v1.0.0") })
	s.Equal(1, k.Count(mark, "hook:mod:onStartup"), "OnStartup once for one enable: %v", k.Log(mark))
	s.Equal(1, k.Count(mark, "enable-hooks:mod"), "the hooks start once for one enable")

	mark = k.Mark()
	k.ApplyModule(et.Module("mod", "v1.0.0"), false)
	k.WaitReport("mod", "disabled", func(r model.Report) bool { return et.False(r.Current.Installed, model.ReasonDisabled) })
	s.False(k.Hooks.Running("mod"), "the disable stopped the hooks")

	k.ApplyModule(et.Module("mod", "v1.0.0"), true)
	k.WaitReport("mod", "re-enabled", func(r model.Report) bool { return et.True(r.Current.Installed) })
	k.WaitLog(mark, "upgraded:mod@v1.0.0")
	s.Equal(1, k.Count(mark, "hook:mod:onStartup"), "a re-enable runs OnStartup again: %v", k.Log(mark))
}

// J2: the disable stops the hooks after the deletion hooks and the release delete.
func (s *hooksSuite) TestJ2DisableStopsHooksLast() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.ApplyModule(et.Module("mod", "v1.0.0"), true)
	k.WaitReport("mod", "installed", func(r model.Report) bool { return et.Running(r, "v1.0.0") })

	mark := k.Mark()
	k.ApplyModule(et.Module("mod", "v1.0.0"), false)
	k.WaitReport("mod", "disabled", func(r model.Report) bool { return et.False(r.Current.Installed, model.ReasonDisabled) })

	log := k.Log(mark)
	order := []string{"remove-monitor:mod", "hook:mod:beforeDeleteHelm", "delete:mod", "hook:mod:afterDeleteHelm", "disable:mod"}

	last := -1
	for _, e := range order {
		i := k.Index(mark, e)
		s.Require().GreaterOrEqual(i, 0, "missing %s in %v", e, log)
		s.Greater(i, last, "%s out of order in %v", e, log)
		last = i
	}
}

// J3: a runtime hook failure is an observation, not a lifecycle input: the package stays
// Installed and converged, and the bindings' disable clears it.
func (s *hooksSuite) TestJ3RuntimeHookFailureIsObservedOnly() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.ApplyModule(et.Module("mod", "v1.0.0"), true)
	k.WaitReport("mod", "installed", func(r model.Report) bool { return et.Running(r, "v1.0.0") })
	// The facts of the run reach the scheduler after the Report shows it.
	k.Wait("converged", k.Engine.Converged)

	before := et.Dump(k.Report("mod"))
	k.Hooks.RuntimeFailure("mod", "watcher", "boom")
	s.Equal("watcher: boom", k.Observed.HookFailure("mod"))
	s.Equal(before, et.Dump(k.Report("mod")), "a runtime hook failure changes no Report")
	s.True(k.Engine.Converged())

	k.ApplyModule(et.Module("mod", "v1.0.0"), false)
	k.WaitReport("mod", "disabled", func(r model.Report) bool { return et.False(r.Current.Installed, model.ReasonDisabled) })
	s.Empty(k.Observed.HookFailure("mod"), "stopping the hooks resets the observed failure")
}

func (s *hooksSuite) Unknown(c model.Condition, msg string) {
	s.T().Helper()
	s.True(et.Unknown(c), msg)
}
