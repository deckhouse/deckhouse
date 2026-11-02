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
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"controller/apis/deckhouse.io/v1alpha3"
)

// A family of objects is counted by the ledger itself, from an informer of its objects: no
// ResourceQuota counts its keys. The webhook of the family is called on the object, not on the status
// of a quota, so an approval is pending under the name of the object until the informer shows the
// version it made.

// FamilyVM is the family of the virtual machines.
const FamilyVM = "vm"

// objectEntry is an object of a family as the ledger knows it.
type objectEntry struct {
	namespace string
	// rv and used are the version and the usage of the object the informer shows; rv is 0 for an
	// object only an approval knows.
	rv   uint64
	used corev1.ResourceList
	// pending are the approvals the informer has not shown yet. Two writes of one version may both be
	// approved, one of them conflicting later, so every approval is kept until a version covers it.
	pending []pendingObject
	// eff is what the object adds to the sum of its project per key of the family, and project the
	// project whose sum it is part of.
	eff     corev1.ResourceList
	project string
}

// pendingObject is an approved write of an object: the version it replaces, 0 for a create, the
// usage it writes and when it was approved.
type pendingObject struct {
	rvOld uint64
	used  corev1.ResourceList
	at    time.Time
}

// ObjectInput is a write of an object of a family the webhook is asked about.
type ObjectInput struct {
	Family    string
	Namespace string
	Name      string
	DryRun    bool
	// OldRV and OldUsed are the version and the usage of the object the write replaces, zero for a
	// create.
	OldRV   uint64
	OldUsed corev1.ResourceList
	// NewUsed is the usage the write puts.
	NewUsed corev1.ResourceList
}

func objectKey(namespace, name string) string { return namespace + "/" + name }

func (s *State) familyLocked(family string) map[string]*objectEntry {
	objects, ok := s.objects[family]
	if !ok {
		objects = map[string]*objectEntry{}
		s.objects[family] = objects
	}
	return objects
}

// SetObject records an object of a family the informer shows. A version not newer than the one
// recorded is ignored; the approval the version covers is dropped.
func (s *State) SetObject(family, namespace, name string, rv uint64, used corev1.ResourceList) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.changedLocked()
	objects := s.familyLocked(family)
	o, ok := objects[objectKey(namespace, name)]
	if !ok {
		o = &objectEntry{namespace: namespace}
		objects[objectKey(namespace, name)] = o
	} else if rv <= o.rv {
		return
	}
	o.rv, o.used = rv, used.DeepCopy()
	o.pending = slices.DeleteFunc(o.pending, func(p pendingObject) bool { return p.rvOld < rv })
	s.seeVersionLocked(rv)
	s.recomputeObjectLocked(family, o)
}

// seeVersionLocked keeps the newest version the ledger has seen.
func (s *State) seeVersionLocked(rv uint64) {
	if rv > s.newestRV {
		s.newestRV = rv
	}
}

// ObjectUsed returns the usage the informer last showed for the object, and whether the ledger knows
// the object.
func (s *State) ObjectUsed(family, namespace, name string) (corev1.ResourceList, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.objects[family][objectKey(namespace, name)]
	if !ok {
		return nil, false
	}
	return o.used.DeepCopy(), true
}

// DeleteObject forgets an object the informer shows deleted, and its approval.
func (s *State) DeleteObject(family, namespace, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.changedLocked()
	objects := s.familyLocked(family)
	o, ok := objects[objectKey(namespace, name)]
	if !ok {
		return
	}
	s.subtractObjectLocked(family, o)
	delete(objects, objectKey(namespace, name))
	if project, ok := s.namespaces[namespace]; ok {
		s.released[project] = true
	}
}

// DecideObject answers a write of an object of a family. A write that adds nothing to a key the project
// limits is let through without being recorded; a dry run is decided but not recorded.
func (s *State) DecideObject(in ObjectInput, now time.Time, claim func() bool) Decision {
	s.mu.Lock()
	defer s.mu.Unlock()

	projectName, ok := s.namespaces[in.Namespace]
	if !ok {
		return unavailable(ResultNoProject, "the namespace belongs to no project", true)
	}
	p, ok := s.projects[projectName]
	if !ok {
		return unavailable(ResultNoProjectQuota, "the project has no ProjectQuota", true)
	}
	hard := p.familyHard[in.Family]
	if len(hard) == 0 {
		return allow(ResultAllowed)
	}
	// The version a write replaces is one the API server has stored, so it is never far above the newest
	// version the informers showed. A version the informers will not reach in years cannot be cleared
	// by them; it comes only in a request that was not sent by an API server.
	if in.OldRV > s.newestRV+maxVersionLead {
		return Decision{Code: http.StatusBadRequest, Result: ResultBadRequest,
			Message: fmt.Sprintf("the old object has the version %d, far above the newest version the ledger has seen", in.OldRV)}
	}
	o := s.objects[in.Family][objectKey(in.Namespace, in.Name)]

	var keys []corev1.ResourceName
	base := make(corev1.ResourceList, len(hard))
	for key := range hard {
		// what the object takes now: the larger of the ledger's view and of the object the write
		// replaces
		current := in.OldUsed[key].DeepCopy()
		if o != nil {
			if known := o.eff[key]; known.Cmp(current) > 0 {
				current = known.DeepCopy()
			}
		}
		base[key] = current
		if written := in.NewUsed[key]; written.Cmp(current) > 0 {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return allow(ResultAllowed)
	}
	slices.Sort(keys)

	var over []corev1.ResourceName
	for _, key := range keys {
		projected := in.NewUsed[key].DeepCopy()
		projected.Add(p.familySum[in.Family][key])
		projected.Sub(base[key])
		if limit := hard[key]; projected.Cmp(limit) > 0 {
			over = append(over, key)
		}
	}
	if len(over) > 0 {
		return Decision{Code: http.StatusForbidden, Result: ResultExceeded, Message: s.objectExceededLocked(projectName, p, in, base, over)}
	}

	for _, key := range keys {
		inflight := s.objectInflightLocked(in.Family, projectName, key)
		if inflight.Sign() == 0 {
			continue
		}
		increase := in.NewUsed[key].DeepCopy()
		increase.Sub(base[key])
		inflight.Add(increase)
		if limit := hard[key]; inflight.AsApproximateFloat64() > inflightShare*limit.AsApproximateFloat64() {
			return unavailable(ResultBusyInflight, "too many approvals of the project are not confirmed yet", true)
		}
	}

	if in.DryRun {
		return allow(ResultDryRun)
	}
	if !claim() {
		return unavailable(ResultBusy, "busy", false)
	}
	if o == nil {
		o = &objectEntry{namespace: in.Namespace}
		s.familyLocked(in.Family)[objectKey(in.Namespace, in.Name)] = o
	}
	o.pending = append(o.pending, pendingObject{rvOld: in.OldRV, used: in.NewUsed.DeepCopy(), at: now})
	s.recomputeObjectLocked(in.Family, o)
	return allow(ResultAllowed)
}

// objectExceededMessage is the refusal of a write that takes the project over its quota, in the form
// of the refusals of the quota plugin.
func (s *State) objectExceededLocked(projectName string, p *projectState, in ObjectInput, base corev1.ResourceList, over []corev1.ResourceName) string {
	requested := make([]string, 0, len(over))
	used := make([]string, 0, len(over))
	limited := make([]string, 0, len(over))
	for _, key := range over {
		delta := in.NewUsed[key].DeepCopy()
		delta.Sub(base[key])
		current := p.familySum[in.Family][key].DeepCopy()
		limit := p.familyHard[in.Family][key]
		requested = append(requested, fmt.Sprintf("%s=%s", key, delta.String()))
		used = append(used, fmt.Sprintf("%s=%s", key, current.String()))
		limited = append(limited, fmt.Sprintf("%s=%s", key, limit.String()))
	}
	return fmt.Sprintf("exceeded quota: %s, project %s: requested: %s, used: %s, limited: %s (%d namespaces)",
		v1alpha3.ProjectQuotaName, projectName, strings.Join(requested, ","), strings.Join(used, ","), strings.Join(limited, ","),
		len(s.members[projectName]))
}

// objectInflightLocked is the part of the sum of a project on the key that the approvals of the
// family put above what the informer shows.
func (s *State) objectInflightLocked(family, project string, key corev1.ResourceName) resource.Quantity {
	var total resource.Quantity
	for _, o := range s.objects[family] {
		if o.project != project || len(o.pending) == 0 {
			continue
		}
		over := o.eff[key].DeepCopy()
		over.Sub(o.used[key])
		if over.Sign() > 0 {
			total.Add(over)
		}
	}
	return total
}

// AgedObject is an approval of an object the informer has not confirmed in time.
type AgedObject struct {
	Family, Namespace, Name string
}

// AgedObjects returns the approvals of objects made at or before the time.
func (s *State) AgedObjects(before time.Time) []AgedObject {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []AgedObject
	for family, objects := range s.objects {
		for key, o := range objects {
			if slices.ContainsFunc(o.pending, func(p pendingObject) bool { return !p.at.After(before) }) {
				namespace, name, _ := strings.Cut(key, "/")
				out = append(out, AgedObject{Family: family, Namespace: namespace, Name: name})
			}
		}
	}
	return out
}

// ResolveObject settles the approvals of an object made at or before the time with a quorum read taken
// after it: found tells whether the object is there, and rv and used are its version and usage. A
// write approved that long ago has landed or failed by the read, so the read is taken as the object
// until the informer catches up.
func (s *State) ResolveObject(family, namespace, name string, before time.Time, found bool, rv uint64, used corev1.ResourceList) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.changedLocked()
	objects := s.familyLocked(family)
	o, ok := objects[objectKey(namespace, name)]
	if !ok {
		return
	}
	o.pending = slices.DeleteFunc(o.pending, func(p pendingObject) bool { return !p.at.After(before) || found && p.rvOld < rv })
	switch {
	case !found && o.rv == 0 && len(o.pending) == 0:
		// a create that never landed
		s.subtractObjectLocked(family, o)
		delete(objects, objectKey(namespace, name))
		return
	case found && rv >= o.rv:
		o.rv, o.used = rv, used.DeepCopy()
		s.seeVersionLocked(rv)
	}
	s.recomputeObjectLocked(family, o)
}

// ObjectPendingCount is the number of approvals of objects the informer has not shown yet.
func (s *State) ObjectPendingCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, objects := range s.objects {
		for _, o := range objects {
			n += len(o.pending)
		}
	}
	return n
}

// rebuildFamiliesLocked computes the sums of the families of a project anew.
func (s *State) rebuildFamiliesLocked(project string) {
	p, ok := s.projects[project]
	if !ok {
		return
	}
	p.familySum = make(map[string]corev1.ResourceList, len(p.familyHard))
	for family := range p.familyHard {
		p.familySum[family] = corev1.ResourceList{}
	}
	for family, objects := range s.objects {
		for _, o := range objects {
			if s.namespaces[o.namespace] == project {
				o.eff, o.project = nil, ""
				s.recomputeObjectLocked(family, o)
			}
		}
	}
}

// recomputeObjectsInLocked moves the objects of a namespace to the sum of its project now.
func (s *State) recomputeObjectsInLocked(namespace string) {
	for family, objects := range s.objects {
		for _, o := range objects {
			if o.namespace == namespace {
				s.recomputeObjectLocked(family, o)
			}
		}
	}
}

// recomputeObjectLocked moves the part of an object in the sum of its project to what it is now.
func (s *State) recomputeObjectLocked(family string, o *objectEntry) {
	s.subtractObjectLocked(family, o)
	project := s.namespaces[o.namespace]
	p, ok := s.projects[project]
	if !ok {
		return
	}
	hard := p.familyHard[family]
	o.project = project
	o.eff = make(corev1.ResourceList, len(hard))
	if p.familySum == nil {
		p.familySum = map[string]corev1.ResourceList{}
	}
	sum := p.familySum[family]
	if sum == nil {
		sum = corev1.ResourceList{}
		p.familySum[family] = sum
	}
	for key := range hard {
		value := o.used[key].DeepCopy()
		for _, p := range o.pending {
			if v := p.used[key]; v.Cmp(value) > 0 {
				value = v.DeepCopy()
			}
		}
		o.eff[key] = value
		total := sum[key]
		total.Add(value)
		sum[key] = total
	}
}

// subtractObjectLocked takes the part of an object out of the sum of the project it was part of.
func (s *State) subtractObjectLocked(family string, o *objectEntry) {
	if p, ok := s.projects[o.project]; ok && p.familySum != nil {
		if sum := p.familySum[family]; sum != nil {
			for key, value := range o.eff {
				total := sum[key]
				total.Sub(value)
				sum[key] = total
			}
		}
	}
	o.eff, o.project = nil, ""
}
