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

package reconcile_test

import (
	"errors"
	"time"

	"github.com/stretchr/testify/suite"

	et "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/enginetest"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
)

// quietPeriod is how long a test watches for something that must not happen after the engine
// converged; the engine reacts within milliseconds, retries back off from 1 s.
const quietPeriod = 400 * time.Millisecond

var errBoom = errors.New("boom")

// base is the shared part of the scenario suites.
type base struct {
	suite.Suite
}

// installed waits until name runs version with its checkpoint written.
func (s *base) installed(k *et.Kit, name, version string) model.Report {
	s.T().Helper()

	return k.WaitReport(name, "running "+version, func(r model.Report) bool {
		return et.Running(r, version)
	})
}

// converged waits until the engine is converged (Global synchronized, every module converged);
// the kit wakes on every Publish, where convergence changes.
func (s *base) converged(k *et.Kit) {
	s.T().Helper()

	k.Wait("the engine to converge", k.Engine.Converged)
}

// upgrades counts the successful upgrades of name since mark.
func upgrades(k *et.Kit, mark int, name, version string) int {
	return k.Count(mark, "upgraded:"+name+"@"+version)
}

// exactly waits for n upgrades of name since mark, for convergence, then checks that no further
// upgrade follows within quietPeriod.
func (s *base) exactly(k *et.Kit, mark int, name, version string, n int) {
	s.T().Helper()

	k.Wait("upgrades of "+name, func() bool { return upgrades(k, mark, name, version) >= n })
	s.converged(k)
	s.False(k.WaitFor(quietPeriod, func() bool { return upgrades(k, mark, name, version) > n }),
		"%s upgraded more than %d times: %v", name, n, k.Log(mark))
	s.Equal(n, upgrades(k, mark, name, version), "upgrades of %s: %v", name, k.Log(mark))
}

// inOrder asserts that entries appear after mark in this order.
func (s *base) inOrder(k *et.Kit, mark int, entries ...string) {
	s.T().Helper()

	log := k.Log(mark)
	last := -1

	for _, e := range entries {
		i := -1

		for j := last + 1; j < len(log); j++ {
			if log[j] == e {
				i = j
				break
			}
		}

		s.Require().GreaterOrEqual(i, 0, "%s missing after position %d in %v", e, last, log)
		last = i
	}
}
