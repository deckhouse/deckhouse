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

package enginetest_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	et "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/enginetest"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
)

// installSuite covers scenarios A: first installs and boot.
type installSuite struct {
	suite.Suite
}

func TestInstall(t *testing.T) {
	suite.Run(t, new(installSuite))
}

// A1: a downloaded module: requirements → attach → load → final gate → activation → first run.
func (s *installSuite) TestA1FirstInstallOfADownloadedModule() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	mark := k.Mark()
	k.ApplyModule(et.Module("mod", "v1.0.0"), true)

	r := k.WaitReport("mod", "installed", func(r model.Report) bool { return et.Running(r, "v1.0.0") })
	s.True(r.Current.Decision != nil && r.Current.Decision.Enabled, "decision reported")
	s.True(et.True(r.Current.Configured))

	order := []string{
		"attach:mod@v1.0.0", "load:mod@v1.0.0", "add-node:mod@v1.0.0", "enable-hooks:mod",
		"hook:mod:onStartup", "hook:mod:beforeHelm", "upgrade:mod", "upgraded:mod@v1.0.0", "hook:mod:afterHelm",
	}
	s.inOrder(k, mark, order)
	s.Equal([]string{"mod@v1.0.0"}, k.Disk.Attached())
	s.Contains(k.Global.EnabledModules(), "mod", "Global published the enabled module")
	// The facts of the run reach the scheduler after the Report shows it.
	k.Wait("ready", k.Engine.Ready)
}

func (s *installSuite) inOrder(k *et.Kit, mark int, entries []string) {
	s.T().Helper()

	last := -1
	for _, e := range entries {
		i := k.Index(mark, e)
		s.Require().GreaterOrEqual(i, 0, "missing %s in %v", e, k.Log(mark))
		s.Greater(i, last, "%s out of order in %v", e, k.Log(mark))
		last = i
	}
}
