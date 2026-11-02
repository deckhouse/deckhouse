/*
Copyright 2026 Flant JSC

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

package ledger

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"controller/internal/quota/evaluator"
)

const cores = "virtualmachines.virtualization.deckhouse.io/cores"

// vmState is a project p with namespaces a and b and a quota of 8 cores of virtual machines.
func vmState() *State {
	s := NewState()
	s.SetProjectQuota(pq("p", rl(cores, "8", "pods", "100")))
	s.SetNamespace(ns("a", "p"))
	s.SetNamespace(ns("b", "p"))
	return s
}

func vmWrite(namespace, name string, oldRV uint64, oldCores, newCores string) ObjectInput {
	in := ObjectInput{Family: FamilyVM, Namespace: namespace, Name: name, OldRV: oldRV, NewUsed: rl(cores, newCores)}
	if oldCores != "" {
		in.OldUsed = rl(cores, oldCores)
	}
	return in
}

func TestDecideObjectSumsTheProject(t *testing.T) {
	s := vmState()
	s.SetObject(FamilyVM, "a", "one", 10, rl(cores, "4"))
	s.SetObject(FamilyVM, "b", "two", 11, rl(cores, "2"))

	d := s.DecideObject(vmWrite("a", "three", 0, "", "2"), testNow, always)
	require.True(t, d.Allowed, d.Message)
	assert.Equal(t, 1, s.ObjectPendingCount())

	d = s.DecideObject(vmWrite("b", "four", 0, "", "1"), testNow, always)
	require.False(t, d.Allowed, "the approval of three counts until the informer shows it")
	assert.Equal(t, int32(http.StatusForbidden), d.Code)
	assert.Equal(t, "exceeded quota: d8-project-quota, project p: requested: "+cores+"=1, used: "+cores+"=8, limited: "+cores+"=8 (2 namespaces)", d.Message)

	s.SetObject(FamilyVM, "a", "three", 12, rl(cores, "2"))
	assert.Zero(t, s.ObjectPendingCount(), "the informer shows the approved object")
	s.DeleteObject(FamilyVM, "a", "one")
	d = s.DecideObject(vmWrite("b", "four", 0, "", "1"), testNow, always)
	assert.True(t, d.Allowed, d.Message)
}

func TestDecideObjectOnAnUpdate(t *testing.T) {
	s := vmState()
	s.SetObject(FamilyVM, "a", "one", 10, rl(cores, "6"))

	d := s.DecideObject(vmWrite("a", "one", 10, "6", "4"), testNow, always)
	assert.True(t, d.Allowed)
	assert.Zero(t, s.ObjectPendingCount(), "a write that takes nothing more is not recorded")

	assert.True(t, s.DecideObject(vmWrite("a", "one", 10, "6", "8"), testNow, always).Allowed, "the difference counts, not the whole machine")
	assert.False(t, s.DecideObject(vmWrite("b", "two", 0, "", "1"), testNow, always).Allowed)
}

func TestDecideObjectDryRunIsNotRecorded(t *testing.T) {
	s := vmState()
	in := vmWrite("a", "one", 0, "", "8")
	in.DryRun = true
	assert.Equal(t, ResultDryRun, s.DecideObject(in, testNow, always).Result)
	assert.Zero(t, s.ObjectPendingCount())
	in = vmWrite("a", "one", 0, "", "9")
	in.DryRun = true
	assert.False(t, s.DecideObject(in, testNow, always).Allowed, "a dry run over the quota is refused like the real write")
}

func TestDecideObjectWithoutTheKeysOfTheFamily(t *testing.T) {
	s := NewState()
	s.SetProjectQuota(pq("p", rl("pods", "1")))
	s.SetNamespace(ns("a", "p"))
	assert.True(t, s.DecideObject(vmWrite("a", "one", 0, "", "100"), testNow, always).Allowed)
	assert.Equal(t, ResultNoProject, s.DecideObject(vmWrite("c", "one", 0, "", "1"), testNow, always).Result)
}

// An approval the informer does not show is settled by a quorum read: an object that is not there
// gives its place back.
func TestResolveObject(t *testing.T) {
	s := vmState()
	require.True(t, s.DecideObject(vmWrite("a", "one", 0, "", "8"), testNow, always).Allowed)
	require.Equal(t, []AgedObject{{Family: FamilyVM, Namespace: "a", Name: "one"}}, s.AgedObjects(testNow))

	s.ResolveObject(FamilyVM, "a", "one", testNow, false, 0, nil)
	assert.Zero(t, s.ObjectPendingCount())
	assert.True(t, s.DecideObject(vmWrite("b", "two", 0, "", "8"), testNow, always).Allowed)
}

// A namespace that leaves the project takes its objects out of the sum, and a changed quota is
// applied to the objects there are.
func TestObjectsFollowTheirNamespaceAndTheQuota(t *testing.T) {
	s := vmState()
	s.SetObject(FamilyVM, "a", "one", 10, rl(cores, "8"))
	assert.False(t, s.DecideObject(vmWrite("b", "two", 0, "", "1"), testNow, always).Allowed)

	s.DeleteNamespace("a")
	assert.True(t, s.DecideObject(vmWrite("b", "two", 0, "", "1"), testNow, always).Allowed)

	s.SetObject(FamilyVM, "b", "two", 11, rl(cores, "1"))
	s.SetNamespace(ns("a", "p"))
	s.SetProjectQuota(pq("p", rl(cores, "20")))
	assert.True(t, s.DecideObject(vmWrite("b", "three", 0, "", "10"), testNow, always).Allowed)
	assert.False(t, s.DecideObject(vmWrite("b", "four", 0, "", "2"), testNow, always).Allowed, "8 + 1 + 10 + 2 > 20")
}

// Approvals the informer has not shown yet may take at most a tenth of a limit.
func TestDecideObjectBoundsTheUnconfirmedApprovals(t *testing.T) {
	s := NewState()
	s.SetProjectQuota(pq("p", rl(cores, "100")))
	s.SetNamespace(ns("a", "p"))
	require.True(t, s.DecideObject(vmWrite("a", "one", 0, "", "8"), testNow, always).Allowed)
	d := s.DecideObject(vmWrite("a", "two", 0, "", "4"), testNow, always)
	assert.Equal(t, ResultBusyInflight, d.Result)
	assert.True(t, d.Retry)
}

func TestObjectVersionsOnlyGoForward(t *testing.T) {
	s := vmState()
	s.SetObject(FamilyVM, "a", "one", 10, rl(cores, "4"))
	s.SetObject(FamilyVM, "a", "one", 9, rl(cores, "1"))
	assert.False(t, s.DecideObject(vmWrite("b", "two", 0, "", "5"), testNow, always).Allowed)
}

// Two writes of one version of an object may both be approved, the second one on another key; the
// first one still counts until a version covers it, so a third write cannot take its place.
func TestDecideObjectKeepsEveryApprovalOfAnObject(t *testing.T) {
	memory := string(evaluator.VMMemory)
	s := NewState()
	s.SetProjectQuota(pq("p", rl(cores, "10", memory, "1000Gi")))
	s.SetNamespace(ns("a", "p"))
	s.SetObject(FamilyVM, "a", "x", 100, rl(cores, "2", memory, "1Gi"))

	grow := ObjectInput{Family: FamilyVM, Namespace: "a", Name: "x", OldRV: 100, OldUsed: rl(cores, "2", memory, "1Gi"), NewUsed: rl(cores, "8", memory, "1Gi")}
	require.True(t, s.DecideObject(grow, testNow, always).Allowed)
	more := ObjectInput{Family: FamilyVM, Namespace: "a", Name: "x", OldRV: 100, OldUsed: rl(cores, "2", memory, "1Gi"), NewUsed: rl(cores, "2", memory, "16Gi")}
	require.True(t, s.DecideObject(more, testNow, always).Allowed)
	assert.Equal(t, 2, s.ObjectPendingCount())

	assert.False(t, s.DecideObject(vmWrite("a", "y", 0, "", "8"), testNow, always).Allowed, "the 8 cores of the first approval still count")
	sum := s.projects["p"].familySum[FamilyVM][evaluator.VMCores]
	assert.Equal(t, int64(8), sum.Value())

	s.SetObject(FamilyVM, "a", "x", 101, rl(cores, "8", memory, "1Gi"))
	assert.Zero(t, s.ObjectPendingCount(), "the version the writes replaced is gone")
}

// A write whose old object has a version far above any the informers showed was not sent by an API
// server, and an informer would never clear its approval.
func TestDecideObjectRefusesAnOldVersionFromNowhere(t *testing.T) {
	s := vmState()
	s.SetObject(FamilyVM, "a", "one", 10, rl(cores, "2"))
	d := s.DecideObject(vmWrite("a", "one", 1<<63, "2", "4"), testNow, always)
	assert.False(t, d.Allowed)
	assert.Equal(t, ResultBadRequest, d.Result)
	assert.Zero(t, s.ObjectPendingCount())
	assert.True(t, s.DecideObject(vmWrite("a", "one", 12, "2", "4"), testNow, always).Allowed, "an informer that lags a little")
}

// Without settling by age only an approval a quorum read shows written goes: a create not seen yet
// stays counted.
func TestResolveObjectWithoutSettleKeepsAWriteNotLanded(t *testing.T) {
	s := vmState()
	require.True(t, s.DecideObject(vmWrite("a", "one", 0, "", "8"), testNow, always).Allowed)
	s.ResolveObject(FamilyVM, "a", "one", time.Time{}, false, 0, nil)
	assert.Equal(t, 1, s.ObjectPendingCount())
	assert.False(t, s.DecideObject(vmWrite("b", "two", 0, "", "1"), testNow, always).Allowed)

	s.ResolveObject(FamilyVM, "a", "one", time.Time{}, true, 5, rl(cores, "8"))
	assert.Zero(t, s.ObjectPendingCount(), "the read shows it written")
}

// With a small limit of virtual machines the unconfirmed approvals leave room for one machine at a
// time; parallel creates wait for the ones before them to be confirmed instead of being answered
// busy, so the ones that fit are admitted and the others refused over the quota.
func TestDecideObjectWaitsForRoom(t *testing.T) {
	s := NewState()
	s.SetProjectQuota(pq("p", rl(cores, "3")))
	s.SetNamespace(ns("a", "p"))
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for rv := uint64(100); ; rv++ {
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
			}
			s.mu.Lock()
			type written struct {
				namespace, name string
				used            corev1.ResourceList
			}
			var shown []written
			for key, o := range s.objects[FamilyVM] {
				if len(o.pending) > 0 {
					namespace, name, _ := strings.Cut(key, "/")
					shown = append(shown, written{namespace, name, o.pending[len(o.pending)-1].used})
				}
			}
			s.mu.Unlock()
			for _, w := range shown {
				s.SetObject(FamilyVM, w.namespace, w.name, rv, w.used)
			}
		}
	}()

	results := make(chan Decision, 10)
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			in := vmWrite("a", fmt.Sprintf("vm-%d", i), 0, "", "1")
			results <- decideWithRoom(context.Background(), s, "a", serving, nil, nil, func() Decision {
				return decideWithin(func(claim func() bool) Decision { return s.DecideObject(in, time.Now(), claim) })
			})
		}()
	}
	wg.Wait()
	close(results)
	admitted, exceeded := 0, 0
	for d := range results {
		switch {
		case d.Allowed:
			admitted++
		case d.Result == ResultExceeded:
			exceeded++
		}
	}
	assert.Equal(t, 3, admitted)
	assert.Equal(t, 7, exceeded, "the rest are refused over the quota, not answered busy")
}
