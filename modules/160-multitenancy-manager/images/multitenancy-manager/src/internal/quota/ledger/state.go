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

// Package ledger keeps the sum of a project quota in memory and answers the webhook on the status
// writes of the d8-project-quota ResourceQuota objects.
//
// The quota plugin of the API server writes the status of every ResourceQuota of a namespace before
// it admits an object there, and an object is admitted only when that write succeeds. The webhook
// lets a write of d8-project-quota through only when the usage of the whole project, with the write,
// stays within the ProjectQuota. The ledger holds, for every namespace of a quota project, the usage
// the informer last saw and the writes it approved that the informer has not shown yet (pending); the
// usage of a namespace is the larger of the two, and the sum of a project is kept by deltas, so a
// decision does no I/O and costs O(keys).
//
// Only the holder of the ledger lease answers, and only while it has sent a successful renewal less
// than the fence ago: a new holder takes the lease no earlier than the lease duration after it saw
// the last renewal and starts from a quorum list, so an approval of the old holder that it cannot
// see yet is one whose write took longer than lease - fence - webhook timeout.
package ledger

import (
	"slices"
	"strconv"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/quota"
)

// pendingEntry is a write the ledger approved and the informer has not shown yet: the version of the
// quota the write replaces, the usage it writes and when it was approved.
type pendingEntry struct {
	rvOld uint64
	used  corev1.ResourceList
	at    time.Time
}

// namespaceQuota is the d8-project-quota of a namespace as the ledger knows it.
type namespaceQuota struct {
	uid     types.UID
	rv      uint64
	used    corev1.ResourceList
	hard    corev1.ResourceList
	pending []pendingEntry
	// eff is the usage the namespace adds to the sum of its project, per key of the project.
	eff corev1.ResourceList
	// project is the project whose sum eff is part of, empty when it is part of none.
	project string
}

type projectState struct {
	hard corev1.ResourceList
	sum  corev1.ResourceList
	// familyHard and familySum are the limits and the sum of the families of objects, by family.
	familyHard map[string]corev1.ResourceList
	familySum  map[string]corev1.ResourceList
}

// State is the memory of the ledger. The zero value is not usable; use NewState.
type State struct {
	mu sync.Mutex
	// quotas are the d8-project-quota objects by namespace.
	quotas map[string]*namespaceQuota
	// namespaces map a namespace to its project.
	namespaces map[string]string
	// members are the namespaces of a project.
	members map[string]map[string]struct{}
	// projects are the projects with a ProjectQuota.
	projects map[string]*projectState
	// refused are the namespaces whose write was refused over the quota of their project, by when.
	refused map[string]time.Time
	// released are the projects whose usage went down since the last wake-up, by when it was woken.
	released map[string]bool
	woken    map[string]time.Time
	// changed is closed and made anew whenever the state changes, so that a write waiting for the
	// approvals of its project to be confirmed wakes up; it has a lock of its own, so that taking it
	// never waits for a decision. waiting counts those writes per project.
	signalMu sync.Mutex
	changed  chan struct{}
	waiting  map[string]int
	// objects are the objects of the families of objects, by family and namespace/name.
	objects map[string]map[string]*objectEntry
	// newestRV is the newest version of an object the informers showed.
	newestRV uint64
}

// NewState returns an empty ledger memory.
func NewState() *State {
	return &State{
		quotas:     map[string]*namespaceQuota{},
		namespaces: map[string]string{},
		members:    map[string]map[string]struct{}{},
		projects:   map[string]*projectState{},
		refused:    map[string]time.Time{},
		released:   map[string]bool{},
		woken:      map[string]time.Time{},
		objects:    map[string]map[string]*objectEntry{},
	}
}

// ParseResourceVersion reads a resourceVersion as an integer, as the etcd storage of the API server
// writes it. A version that is not one reads as 0.
func ParseResourceVersion(rv string) uint64 {
	v, err := strconv.ParseUint(rv, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// SetQuota records the d8-project-quota of a namespace the informer shows. A version not newer than
// the one recorded is ignored. The pending writes the new version covers are dropped.
func (s *State) SetQuota(rq *corev1.ResourceQuota) int {
	if rq.Name != v1alpha3.ProjectQuotaName {
		return 0
	}
	rv := ParseResourceVersion(rq.ResourceVersion)
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.changedLocked()
	q, ok := s.quotas[rq.Namespace]
	if !ok {
		q = &namespaceQuota{}
		s.quotas[rq.Namespace] = q
	} else if rv <= q.rv {
		return 0
	}
	q.uid, q.rv = rq.UID, rv
	s.seeVersionLocked(rv)
	if decreased(q.used, rq.Status.Used) {
		if project, ok := s.namespaces[rq.Namespace]; ok {
			s.released[project] = true
		}
	}
	q.used = rq.Status.Used.DeepCopy()
	q.hard = rq.Status.Hard.DeepCopy()
	before := len(q.pending)
	q.pending = slices.DeleteFunc(q.pending, func(p pendingEntry) bool { return p.rvOld < rv })
	s.recomputeLocked(rq.Namespace)
	return before - len(q.pending)
}

// DeleteQuota forgets the d8-project-quota of a namespace.
func (s *State) DeleteQuota(namespace string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.changedLocked()
	q, ok := s.quotas[namespace]
	if !ok {
		return
	}
	s.subtractLocked(q)
	delete(s.quotas, namespace)
}

// SetNamespace records the project of a namespace from its labels; a namespace without the project
// label belongs to none.
func (s *State) SetNamespace(ns *corev1.Namespace) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.changedLocked()
	project := ns.Labels[v1alpha3.ResourceLabelProject]
	previous, known := s.namespaces[ns.Name]
	if known && previous == project {
		return
	}
	if known {
		delete(s.members[previous], ns.Name)
	}
	if project == "" {
		delete(s.namespaces, ns.Name)
	} else {
		s.namespaces[ns.Name] = project
		if s.members[project] == nil {
			s.members[project] = map[string]struct{}{}
		}
		s.members[project][ns.Name] = struct{}{}
	}
	s.recomputeLocked(ns.Name)
	s.recomputeObjectsInLocked(ns.Name)
}

// DeleteNamespace forgets a namespace.
func (s *State) DeleteNamespace(name string) {
	s.SetNamespace(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}})
}

// SetProjectQuota records the limits of a project.
func (s *State) SetProjectQuota(pq *v1alpha1.ProjectQuota) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.changedLocked()
	p, ok := s.projects[pq.Name]
	if !ok {
		p = &projectState{}
		s.projects[pq.Name] = p
	}
	canonical := quota.Canonical(pq.Spec.Hard)
	p.hard = quota.ResourceQuotaHard(canonical)
	s.rebuildLocked(pq.Name)
	p.familyHard = map[string]corev1.ResourceList{FamilyVM: quota.VMHard(canonical)}
	s.rebuildFamiliesLocked(pq.Name)
}

// DeleteProjectQuota forgets the limits of a project.
func (s *State) DeleteProjectQuota(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.changedLocked()
	delete(s.projects, name)
	for ns := range s.members[name] {
		if q, ok := s.quotas[ns]; ok {
			q.eff, q.project = nil, ""
		}
	}
	for _, objects := range s.objects {
		for _, o := range objects {
			if o.project == name {
				o.eff, o.project = nil, ""
			}
		}
	}
}

// rebuildLocked computes the sum of a project anew.
func (s *State) rebuildLocked(project string) {
	p, ok := s.projects[project]
	if !ok {
		return
	}
	p.sum = make(corev1.ResourceList, len(p.hard))
	for key := range p.hard {
		p.sum[key] = resource.Quantity{}
	}
	for ns := range s.members[project] {
		q, ok := s.quotas[ns]
		if !ok {
			continue
		}
		q.project = project
		q.eff = effective(q, p.hard)
		for key, value := range q.eff {
			sum := p.sum[key]
			sum.Add(value)
			p.sum[key] = sum
		}
	}
}

// recomputeLocked moves the namespace's part of the sum of its project to what it is now.
func (s *State) recomputeLocked(namespace string) {
	q, ok := s.quotas[namespace]
	if !ok {
		return
	}
	s.subtractLocked(q)
	project := s.namespaces[namespace]
	p, ok := s.projects[project]
	if !ok {
		return
	}
	q.project = project
	q.eff = effective(q, p.hard)
	for key, value := range q.eff {
		sum := p.sum[key]
		sum.Add(value)
		p.sum[key] = sum
	}
}

// subtractLocked takes the namespace's part out of the sum of the project it was part of.
func (s *State) subtractLocked(q *namespaceQuota) {
	if p, ok := s.projects[q.project]; ok {
		for key, value := range q.eff {
			sum := p.sum[key]
			sum.Sub(value)
			p.sum[key] = sum
		}
	}
	q.eff, q.project = nil, ""
}

// effective is the usage of the namespace per key of the limits: the larger of what the informer
// shows and of what a pending write puts there.
func effective(q *namespaceQuota, hard corev1.ResourceList) corev1.ResourceList {
	eff := make(corev1.ResourceList, len(hard))
	for key := range hard {
		value := q.used[key]
		for _, p := range q.pending {
			if v, ok := p.used[key]; ok && v.Cmp(value) > 0 {
				value = v
			}
		}
		eff[key] = value.DeepCopy()
	}
	return eff
}

// PendingCount is the number of pending writes.
func (s *State) PendingCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, q := range s.quotas {
		n += len(q.pending)
	}
	return n
}

// AgedPending is a pending write the informer has not confirmed in time.
type AgedPending struct {
	Namespace string
	RVOld     uint64
}

// Aged returns the pending writes approved at or before the time.
func (s *State) Aged(before time.Time) []AgedPending {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []AgedPending
	for ns, q := range s.quotas {
		for _, p := range q.pending {
			if !p.at.After(before) {
				out = append(out, AgedPending{Namespace: ns, RVOld: p.rvOld})
			}
		}
	}
	return out
}

// Resolve settles the pending writes of a namespace older than the version a quorum read returned:
// a newer version means the write went in or was overtaken, and the entry goes; the same version
// means it did not go in or still runs, and the entry goes only with settle, when it is old enough to
// have failed: approved at or before the time. An approval of the same version made since may still
// land and stays. The read itself is taken as the usage until the informer catches up. It returns how
// many entries it dropped.
func (s *State) Resolve(rq *corev1.ResourceQuota, rvOld uint64, approvedBefore time.Time, settle bool) int {
	rv := ParseResourceVersion(rq.ResourceVersion)
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.changedLocked()
	q, ok := s.quotas[rq.Namespace]
	if !ok {
		return 0
	}
	before := len(q.pending)
	if settle {
		q.pending = slices.DeleteFunc(q.pending, func(p pendingEntry) bool { return p.rvOld <= rvOld && !p.at.After(approvedBefore) })
	}
	if rv > q.rv {
		q.rv = rv
		q.used = rq.Status.Used.DeepCopy()
		q.hard = rq.Status.Hard.DeepCopy()
		q.pending = slices.DeleteFunc(q.pending, func(p pendingEntry) bool { return p.rvOld < rv })
	}
	s.recomputeLocked(rq.Namespace)
	return before - len(q.pending)
}

// InflightRatioMax is the largest share of a limit that the unconfirmed approvals of a project take.
func (s *State) InflightRatioMax() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	worst := 0.0
	for name, p := range s.projects {
		for key, limit := range p.hard {
			l := limit.AsApproximateFloat64()
			if l <= 0 {
				continue
			}
			inflight := s.inflightLocked(name, key)
			if r := inflight.AsApproximateFloat64() / l; r > worst {
				worst = r
			}
		}
	}
	return worst
}

// inflightLocked is the part of the sum of a project on the key that the informer has not confirmed:
// over the namespaces, what the pending writes put above the usage the informer shows.
func (s *State) inflightLocked(project string, key corev1.ResourceName) resource.Quantity {
	var total resource.Quantity
	for ns := range s.members[project] {
		q, ok := s.quotas[ns]
		if !ok || len(q.pending) == 0 {
			continue
		}
		over := q.eff[key].DeepCopy()
		over.Sub(q.used[key])
		if over.Sign() > 0 {
			total.Add(over)
		}
	}
	return total
}

// Covers reports whether the state has what a quorum list returned: every quota in its listed version
// or a newer one, every namespace in its project, every ProjectQuota.
func (s *State) Covers(listed []metav1.Object) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, object := range listed {
		switch o := object.(type) {
		case *corev1.ResourceQuota:
			q, ok := s.quotas[o.Namespace]
			if !ok || q.rv < ParseResourceVersion(o.ResourceVersion) {
				return false
			}
		case *corev1.Namespace:
			if project := o.Labels[v1alpha3.ResourceLabelProject]; project != "" && s.namespaces[o.Name] != project {
				return false
			}
		case *v1alpha1.ProjectQuota:
			if _, ok := s.projects[o.Name]; !ok {
				return false
			}
		case *unstructured.Unstructured:
			if o.GroupVersionKind().Kind == virtualMachineGVK.Kind {
				entry, ok := s.objects[FamilyVM][objectKey(o.GetNamespace(), o.GetName())]
				if !ok || entry.rv < ParseResourceVersion(o.GetResourceVersion()) {
					return false
				}
			}
		case *metav1.PartialObjectMetadata:
			if o.GroupVersionKind().Kind == virtualMachineGVK.Kind {
				entry, ok := s.objects[FamilyVM][objectKey(o.Namespace, o.Name)]
				if !ok || entry.rv < ParseResourceVersion(o.ResourceVersion) {
					return false
				}
			}
		}
	}
	return true
}

// decreased reports whether some usage went down.
func decreased(before, after corev1.ResourceList) bool {
	for key, value := range before {
		if now := after[key]; now.Cmp(value) < 0 {
			return true
		}
	}
	return false
}

const (
	// wakeWindow is how long after a refusal the quota of a namespace is woken when its project frees
	// some of its quota: a controller that retries on the events of the quota of its namespace, as
	// the virtualization module does, learns that it may try again.
	wakeWindow = 5 * time.Minute
	// wakeEvery bounds the wake-ups of a project.
	wakeEvery = 30 * time.Second
)

// TakeWakes returns the namespaces whose quota to wake: the ones refused in the last few minutes, of
// the projects whose usage went down, at most once in a while per project.
func (s *State) TakeWakes(now time.Time) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for namespace, at := range s.refused {
		if now.Sub(at) > wakeWindow {
			delete(s.refused, namespace)
		}
	}
	for project := range s.released {
		if now.Sub(s.woken[project]) < wakeEvery {
			continue
		}
		delete(s.released, project)
		woke := false
		for namespace := range s.members[project] {
			if _, ok := s.refused[namespace]; ok {
				out = append(out, namespace)
				woke = true
			}
		}
		if woke {
			s.woken[project] = now
		}
	}
	slices.Sort(out)
	return out
}

// Drift counts the quotas that a quorum read shows in the version the ledger holds but with another
// usage. The same version is the same object, so anything but 0 is a bug of the ledger.
func (s *State) Drift(quotas []corev1.ResourceQuota) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	drift := 0
	for i := range quotas {
		q, ok := s.quotas[quotas[i].Namespace]
		if !ok || q.rv != ParseResourceVersion(quotas[i].ResourceVersion) {
			continue
		}
		if !equality.Semantic.DeepEqual(nonZero(q.used), nonZero(quotas[i].Status.Used)) {
			drift++
		}
	}
	return drift
}

// maxWaitingPerProject bounds the writes of one project that wait for room at a time.
const maxWaitingPerProject = 16

// Changed returns a channel closed at the next change of the state.
func (s *State) Changed() <-chan struct{} {
	s.signalMu.Lock()
	defer s.signalMu.Unlock()
	if s.changed == nil {
		s.changed = make(chan struct{})
	}
	return s.changed
}

// changedLocked wakes the writes waiting for a change of the state.
func (s *State) changedLocked() {
	s.signalMu.Lock()
	defer s.signalMu.Unlock()
	if s.changed != nil {
		close(s.changed)
		s.changed = nil
	}
}

// startWaiting takes a place among the writes of the project of the namespace that wait for room; it
// refuses when the project has maxWaitingPerProject of them.
func (s *State) startWaiting(namespace string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// a namespace the informer has not shown in a project yet waits on its own, so that made-up
	// namespaces do not take the places of a new namespace of the cluster
	project, ok := s.namespaces[namespace]
	if !ok || project == "" {
		project = "\x00" + namespace
	}
	if s.waiting == nil {
		s.waiting = map[string]int{}
	}
	if s.waiting[project] >= maxWaitingPerProject {
		return project, false
	}
	s.waiting[project]++
	return project, true
}

func (s *State) stopWaiting(project string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.waiting[project]--; s.waiting[project] <= 0 {
		delete(s.waiting, project)
	}
}
