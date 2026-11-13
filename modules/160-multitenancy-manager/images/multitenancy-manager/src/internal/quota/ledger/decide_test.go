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
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/edition"
)

var testNow = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

func rl(pairs ...string) corev1.ResourceList {
	out := corev1.ResourceList{}
	for i := 0; i+1 < len(pairs); i += 2 {
		out[corev1.ResourceName(pairs[i])] = resource.MustParse(pairs[i+1])
	}
	return out
}

func rq(namespace string, rv uint64, used corev1.ResourceList) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1alpha3.ProjectQuotaName, UID: uidOf(namespace), ResourceVersion: strconv.FormatUint(rv, 10)},
		Status:     corev1.ResourceQuotaStatus{Hard: rl("pods", "100", "requests.cpu", "100"), Used: used},
	}
}

func ns(name, project string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{v1alpha3.ResourceLabelProject: project}}}
}

func pq(name string, hard corev1.ResourceList) *v1alpha1.ProjectQuota {
	return &v1alpha1.ProjectQuota{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: v1alpha1.ProjectQuotaSpec{Hard: hard}}
}

// state is a project p with the limits and two namespaces, a and b, with the usage.
func state(hard, usedA, usedB corev1.ResourceList) *State {
	s := NewState()
	s.SetProjectQuota(pq("p", hard))
	s.SetNamespace(ns("a", "p"))
	s.SetNamespace(ns("b", "p"))
	s.SetQuota(rq("a", 10, usedA))
	s.SetQuota(rq("b", 20, usedB))
	return s
}

// uidOf is the uid of the d8-project-quota of the namespace in the tests.
func uidOf(namespace string) types.UID {
	return types.UID("quota-" + namespace)
}

func write(namespace string, rvOld uint64, oldUsed, newUsed corev1.ResourceList) Input {
	return Input{Namespace: namespace, Name: v1alpha3.ProjectQuotaName, User: APIServerUser, OldUID: uidOf(namespace), OldRV: rvOld, OldUsed: oldUsed,
		NewUsed: newUsed}
}

func always() bool { return true }

func TestDecideTable(t *testing.T) {
	hard := rl("pods", "10")
	tests := []struct {
		name    string
		in      Input
		allowed bool
		code    int32
		result  string
		retry   bool
	}{
		{name: "within the limit", in: write("a", 10, rl("pods", "4"), rl("pods", "5")), allowed: true, result: ResultAllowed},
		{name: "over the limit of the project", in: write("a", 10, rl("pods", "4"), rl("pods", "6")), code: http.StatusForbidden, result: ResultExceeded},
		{name: "a decrease", in: write("a", 10, rl("pods", "4"), rl("pods", "3")), allowed: true, result: ResultAllowed},
		{name: "another user", in: func() Input { i := write("a", 10, rl("pods", "4"), rl("pods", "5")); i.User = "admin"; return i }(),
			code: http.StatusForbidden, result: ResultUnexpected},
		{name: "another quota", in: func() Input { i := write("a", 10, rl("pods", "4"), rl("pods", "5")); i.Name = "own"; return i }(),
			code: http.StatusForbidden, result: ResultUnexpected},
		{name: "a dry run", in: func() Input { i := write("a", 10, rl("pods", "4"), rl("pods", "50")); i.DryRun = true; return i }(),
			allowed: true, result: ResultDryRun},
		{name: "an unknown namespace", in: write("c", 10, nil, rl("pods", "1")), code: http.StatusServiceUnavailable, result: ResultUnknownNamespace, retry: true},
		{name: "a version the informer has seen replaced", in: write("a", 9, rl("pods", "4"), rl("pods", "50")), allowed: true, result: ResultStale},
		{name: "a version the informer has not seen", in: write("a", 11, rl("pods", "4"), rl("pods", "5")), code: http.StatusServiceUnavailable,
			result: ResultLagging, retry: true},
		{name: "a usage the informer does not show", in: write("a", 10, rl("pods", "1"), rl("pods", "2")), code: http.StatusForbidden, result: ResultInconsistent},
		{name: "a usage above the own limit of the quota", in: write("a", 10, rl("pods", "4"), rl("pods", "101")), code: http.StatusForbidden,
			result: ResultInconsistent},
		{name: "another quota object", in: func() Input { i := write("a", 10, rl("pods", "4"), rl("pods", "5")); i.OldUID = "guessed"; return i }(),
			code: http.StatusForbidden, result: ResultInconsistent, retry: true},
		{name: "a key the project does not limit", in: write("a", 10, rl("pods", "4"), rl("pods", "4", "secrets", "100")), allowed: true, result: ResultAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := state(hard, rl("pods", "4"), rl("pods", "5"))
			d := s.Decide(tt.in, testNow, always)
			assert.Equal(t, tt.allowed, d.Allowed, d.Message)
			assert.Equal(t, tt.code, d.Code)
			assert.Equal(t, tt.result, d.Result)
			assert.Equal(t, tt.retry, d.Retry)
		})
	}
}

// A request made up by someone who cannot read the quota names a wrong uid. It gets the same answer
// whatever version and usage it names, so the answers do not lead to them, and nothing is recorded.
func TestDecideAnswersAMadeUpRequestAlike(t *testing.T) {
	s := state(rl("pods", "10"), rl("pods", "0"), rl("pods", "0"))
	var answers []Decision
	for _, rv := range []uint64{1, 9, 10, 11, 1000} {
		for _, oldUsed := range []corev1.ResourceList{rl("pods", "0"), rl("pods", "3")} {
			in := write("a", rv, oldUsed, rl("pods", "6"))
			in.OldUID = "guessed"
			answers = append(answers, s.Decide(in, testNow, always))
		}
	}
	for _, d := range answers {
		assert.Equal(t, answers[0], d)
	}
	assert.Equal(t, ResultInconsistent, answers[0].Result)
	assert.Equal(t, 0, s.PendingCount())
}

func TestDecideRefusalText(t *testing.T) {
	s := state(rl("pods", "10", "requests.cpu", "2"), rl("pods", "4", "requests.cpu", "1"), rl("pods", "5", "requests.cpu", "1"))
	d := s.Decide(write("a", 10, rl("pods", "4", "requests.cpu", "1"), rl("pods", "6", "requests.cpu", "1500m")), testNow, always)
	require.False(t, d.Allowed)
	assert.Equal(t, "exceeded quota: d8-project-quota, project p: requested: pods=2,requests.cpu=500m, used: pods=9,requests.cpu=2, limited: pods=10,requests.cpu=2 (2 namespaces)",
		d.Message)
}

func TestDecideWithoutAProject(t *testing.T) {
	s := NewState()
	s.SetQuota(rq("a", 10, rl("pods", "1")))
	d := s.Decide(write("a", 10, rl("pods", "1"), rl("pods", "2")), testNow, always)
	assert.Equal(t, ResultNoProject, d.Result)

	s.SetNamespace(ns("a", "p"))
	d = s.Decide(write("a", 10, rl("pods", "1"), rl("pods", "2")), testNow, always)
	assert.Equal(t, ResultNoProjectQuota, d.Result)
	assert.Contains(t, d.Message, "project quota ledger unavailable (")
	assert.Contains(t, d.Message, "), retry")
}

// Two writes of the same version with different usage, as two API servers race for one namespace, are
// both pending, and the larger counts until the informer shows which went in; a write of the
// neighbour namespace sees both.
func TestPendingIsASet(t *testing.T) {
	s := state(rl("pods", "100"), rl("pods", "2"), rl("pods", "2"))
	require.True(t, s.Decide(write("a", 10, rl("pods", "2"), rl("pods", "3")), testNow, always).Allowed)
	require.True(t, s.Decide(write("a", 10, rl("pods", "2"), rl("pods", "5")), testNow, always).Allowed)
	assert.Equal(t, 2, s.PendingCount())

	// a counts as 5: b may not grow to 96, which it could if a counted as 3
	assert.Equal(t, ResultExceeded, s.Decide(write("b", 20, rl("pods", "2"), rl("pods", "96")), testNow, always).Result)
	assert.True(t, s.Decide(write("b", 20, rl("pods", "2"), rl("pods", "3")), testNow, always).Allowed)

	// the informer shows the version that replaced 10: both writes of a are settled, and the usage it
	// shows counts
	dropped := s.SetQuota(rq("a", 11, rl("pods", "3")))
	assert.Equal(t, 2, dropped)
	assert.Equal(t, 1, s.PendingCount(), "the pending write of b stays")
}

// A pending entry is not added for a version the informer has seen replaced: its write gets a
// conflict from the storage.
func TestNoPendingForAReplacedVersion(t *testing.T) {
	s := state(rl("pods", "10"), rl("pods", "2"), rl("pods", "2"))
	s.SetQuota(rq("a", 12, rl("pods", "2")))
	d := s.Decide(write("a", 10, rl("pods", "2"), rl("pods", "9")), testNow, always)
	assert.Equal(t, ResultStale, d.Result)
	assert.Zero(t, s.PendingCount())
}

func TestInflightLimit(t *testing.T) {
	s := state(rl("pods", "100"), rl("pods", "10"), rl("pods", "10"))
	require.True(t, s.Decide(write("a", 10, rl("pods", "10"), rl("pods", "15")), testNow, always).Allowed, "the first unconfirmed approval is not limited")

	d := s.Decide(write("b", 20, rl("pods", "10"), rl("pods", "16")), testNow, always)
	assert.Equal(t, ResultBusyInflight, d.Result, "5 + 6 unconfirmed is above 10%% of 100")
	assert.True(t, d.Retry)

	assert.True(t, s.Decide(write("b", 20, rl("pods", "10"), rl("pods", "15")), testNow, always).Allowed, "5 + 5 is within 10%%")
	assert.InDelta(t, 0.1, s.InflightRatioMax(), 0.0001)
}

// An answer the deadline has given already records nothing.
func TestDecideAfterTheDeadline(t *testing.T) {
	s := state(rl("pods", "10"), rl("pods", "2"), rl("pods", "2"))
	d := s.Decide(write("a", 10, rl("pods", "2"), rl("pods", "3")), testNow, func() bool { return false })
	assert.Equal(t, ResultBusy, d.Result)
	assert.Zero(t, s.PendingCount())
}

func TestResolve(t *testing.T) {
	s := state(rl("pods", "10"), rl("pods", "2"), rl("pods", "2"))
	require.True(t, s.Decide(write("a", 10, rl("pods", "2"), rl("pods", "8")), testNow, always).Allowed)
	assert.Len(t, s.Aged(testNow), 1)
	assert.Empty(t, s.Aged(testNow.Add(-time.Second)))

	// a quorum read returns the same version: the write did not go in
	assert.Equal(t, 1, s.Resolve(rq("a", 10, rl("pods", "2")), 10, testNow, true))
	assert.True(t, s.Decide(write("a", 10, rl("pods", "2"), rl("pods", "8")), testNow, always).Allowed, "the room is back")

	// a newer version: the write went in, and its usage counts until the informer catches up
	assert.Equal(t, 1, s.Resolve(rq("a", 13, rl("pods", "8")), 10, testNow, true))
	assert.Equal(t, ResultExceeded, s.Decide(write("b", 20, rl("pods", "2"), rl("pods", "3")), testNow, always).Result)
}

// The settle of an old approval leaves an approval of the same version made since: its write may still
// land, and without it the namespace would drop out of the sum of the project.
func TestResolveSettlesOnlyTheOldApprovals(t *testing.T) {
	s := state(rl("pods", "10"), rl("pods", "2"), rl("pods", "2"))
	require.True(t, s.Decide(write("a", 10, rl("pods", "2"), rl("pods", "3")), testNow.Add(-16*time.Second), always).Allowed)
	require.True(t, s.Decide(write("a", 10, rl("pods", "2"), rl("pods", "3")), testNow, always).Allowed)
	require.Equal(t, 2, s.PendingCount())

	assert.Equal(t, 1, s.Resolve(rq("a", 10, rl("pods", "2")), 10, testNow.Add(-15*time.Second), true))
	assert.Equal(t, 1, s.PendingCount(), "the fresh approval stays")
}

// A namespace that moves to another project takes its usage along.
func TestNamespaceMoves(t *testing.T) {
	s := state(rl("pods", "10"), rl("pods", "6"), rl("pods", "2"))
	s.SetProjectQuota(pq("q", rl("pods", "10")))
	s.SetNamespace(ns("c", "q"))
	s.SetQuota(rq("c", 30, rl("pods", "1")))

	assert.Equal(t, ResultExceeded, s.Decide(write("c", 30, rl("pods", "1"), rl("pods", "11")), testNow, always).Result)
	s.SetNamespace(ns("a", "q"))
	assert.Equal(t, ResultExceeded, s.Decide(write("c", 30, rl("pods", "1"), rl("pods", "5")), testNow, always).Result, "a brought 6 to q")
	assert.True(t, s.Decide(write("b", 20, rl("pods", "2"), rl("pods", "10")), testNow, always).Allowed, "and took them from p")

	s.DeleteNamespace("a")
	assert.True(t, s.Decide(write("c", 30, rl("pods", "1"), rl("pods", "9")), testNow, always).Allowed)
}

// In CE an additional namespace left from another edition is not one of the project, as the controller
// sees it: its usage is not in the sum the main namespace is checked against, which is the sum the
// status of the ProjectQuota shows, and a quorum list with it is covered. In the other editions it
// counts. The test switches the edition of the binary, so it does not run in parallel.
func TestAdditionalNamespaceByEdition(t *testing.T) {
	build := func() *State {
		s := NewState()
		s.SetProjectQuota(pq("p", rl("pods", "10")))
		s.SetNamespace(ns("p", "p"))
		s.SetNamespace(ns("p-db", "p"))
		s.SetQuota(rq("p", 10, rl("pods", "6")))
		s.SetQuota(rq("p-db", 20, rl("pods", "4")))
		return s
	}
	grow := write("p", 10, rl("pods", "6"), rl("pods", "9"))

	t.Run("a commercial edition counts it", func(t *testing.T) {
		defer edition.SetForTest("EE")()
		assert.Equal(t, ResultExceeded, build().Decide(grow, testNow, always).Result)
	})

	t.Run("CE does not", func(t *testing.T) {
		defer edition.SetForTest(edition.CE)()
		s := build()
		assert.True(t, s.Decide(grow, testNow, always).Allowed)
		assert.True(t, s.Covers([]metav1.Object{ns("p", "p"), ns("p-db", "p"), pq("p", nil)}))
	})
}

// The limits of a project change the sum it is checked against.
func TestProjectQuotaChange(t *testing.T) {
	s := state(rl("pods", "10"), rl("pods", "6"), rl("pods", "2"))
	s.SetProjectQuota(pq("p", rl("pods", "8")))
	assert.Equal(t, ResultExceeded, s.Decide(write("b", 20, rl("pods", "2"), rl("pods", "3")), testNow, always).Result)
	s.SetProjectQuota(pq("p", rl("projectNamespaces", "3", "pods", "20")))
	assert.True(t, s.Decide(write("b", 20, rl("pods", "2"), rl("pods", "3")), testNow, always).Allowed)
	s.DeleteProjectQuota("p")
	assert.Equal(t, ResultNoProjectQuota, s.Decide(write("b", 20, rl("pods", "2"), rl("pods", "3")), testNow, always).Result)
}

// Concurrent writes over many projects never take a project over its limit.
func TestConcurrentWritesStayWithinTheLimit(t *testing.T) {
	s := NewState()
	const projects, namespaces, limit = 5, 4, 20
	for p := range projects {
		name := fmt.Sprintf("p%d", p)
		s.SetProjectQuota(pq(name, rl("pods", strconv.Itoa(limit))))
		for n := range namespaces {
			namespace := fmt.Sprintf("%s-%d", name, n)
			s.SetNamespace(ns(namespace, name))
			s.SetQuota(rq(namespace, 1, rl("pods", "0")))
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	approved := map[string]int{}
	for p := range projects {
		for n := range namespaces {
			wg.Add(1)
			go func() {
				defer wg.Done()
				namespace := fmt.Sprintf("p%d-%d", p, n)
				rv := uint64(1)
				used := 0
				for range 50 {
					d := s.Decide(write(namespace, rv, rl("pods", strconv.Itoa(used)), rl("pods", strconv.Itoa(used+1))), testNow, always)
					if !d.Allowed {
						continue
					}
					// the write goes in, and the informer shows it
					used++
					rv++
					s.SetQuota(rq(namespace, rv, rl("pods", strconv.Itoa(used))))
					mu.Lock()
					approved[fmt.Sprintf("p%d", p)]++
					mu.Unlock()
				}
			}()
		}
	}
	wg.Wait()
	for p := range projects {
		assert.Equal(t, limit, approved[fmt.Sprintf("p%d", p)])
	}
}

// A new leader answers only once the state, not only the store of the informers, has what its quorum
// list returned.
func TestCovers(t *testing.T) {
	s := state(rl("pods", "10"), rl("pods", "1"), rl("pods", "1"))
	assert.True(t, s.Covers([]metav1.Object{rq("a", 10, nil), ns("a", "p"), pq("p", nil)}))
	assert.False(t, s.Covers([]metav1.Object{rq("a", 11, nil)}), "a newer quota is not in the state yet")
	assert.False(t, s.Covers([]metav1.Object{ns("c", "p")}), "an unknown namespace")
	assert.False(t, s.Covers([]metav1.Object{pq("q", nil)}), "an unknown ProjectQuota")
}

// A namespace refused over the quota of its project is woken once the project frees some of it, at
// most every wakeEvery per project and only within wakeWindow of the refusal.
func TestWakeUps(t *testing.T) {
	s := state(rl("pods", "10"), rl("pods", "6"), rl("pods", "4"))
	require.Equal(t, ResultExceeded, s.Decide(write("b", 20, rl("pods", "4"), rl("pods", "5")), testNow, always).Result)
	assert.Empty(t, s.TakeWakes(testNow), "nothing was freed yet")

	s.SetQuota(rq("a", 11, rl("pods", "5")))
	assert.Equal(t, []string{"b"}, s.TakeWakes(testNow))
	assert.Empty(t, s.TakeWakes(testNow), "woken once")

	s.SetQuota(rq("a", 12, rl("pods", "4")))
	assert.Empty(t, s.TakeWakes(testNow.Add(time.Second)), "not again within the interval")
	assert.Equal(t, []string{"b"}, s.TakeWakes(testNow.Add(wakeEvery)))

	s.SetQuota(rq("a", 13, rl("pods", "3")))
	assert.Empty(t, s.TakeWakes(testNow.Add(wakeWindow+time.Minute)), "the refusal is too old")
}

func TestDrift(t *testing.T) {
	s := state(rl("pods", "10"), rl("pods", "6"), rl("pods", "4"))
	assert.Zero(t, s.Drift([]corev1.ResourceQuota{*rq("a", 10, rl("pods", "6")), *rq("b", 21, rl("pods", "9"))}), "a newer version is not drift")
	assert.Equal(t, 1, s.Drift([]corev1.ResourceQuota{*rq("a", 10, rl("pods", "7"))}))
}

// The quota plugin writes the keys a request leaves alone as they were, also when a lowered limit or a
// recount left them above the limit of the quota. Such a key does not make the write inconsistent;
// a write that raises a key above the limit still does.
func TestDecideAKeyAboveItsOwnLimitLeftAlone(t *testing.T) {
	hard := rl("pods", "10", "requests.cpu", "100")
	s := NewState()
	s.SetProjectQuota(pq("p", hard))
	s.SetNamespace(ns("a", "p"))
	quota := rq("a", 10, rl("pods", "2", "requests.cpu", "6"))
	quota.Status.Hard = rl("pods", "10", "requests.cpu", "4")
	s.SetQuota(quota)

	d := s.Decide(write("a", 10, rl("pods", "2", "requests.cpu", "6"), rl("pods", "3", "requests.cpu", "6")), testNow, always)
	assert.True(t, d.Allowed, d.Message)

	d = s.Decide(write("a", 10, rl("pods", "2", "requests.cpu", "6"), rl("pods", "2", "requests.cpu", "7")), testNow, always)
	assert.Equal(t, ResultInconsistent, d.Result, "the key above the limit grows")
}

// A leader that hands the lease over keeps an approval whose write a quorum read does not show yet:
// the write may still land, and a new leader would approve without it.
func TestResolveWithoutSettleKeepsAWriteNotLanded(t *testing.T) {
	s := state(rl("pods", "10"), rl("pods", "2"), rl("pods", "2"))
	require.True(t, s.Decide(write("a", 10, rl("pods", "2"), rl("pods", "8")), testNow, always).Allowed)

	assert.Zero(t, s.Resolve(rq("a", 10, rl("pods", "2")), 10, testNow, false))
	assert.Equal(t, 1, s.PendingCount(), "the same version: the write may still land")
	assert.Equal(t, ResultExceeded, s.Decide(write("b", 20, rl("pods", "2"), rl("pods", "3")), testNow, always).Result)

	assert.Equal(t, 1, s.Resolve(rq("a", 11, rl("pods", "8")), 10, testNow, false), "a newer version settles it")
}
