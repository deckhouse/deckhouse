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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"controller/internal/quota/evaluator"
)

const lbAddresses = string(evaluator.LBAddresses)

// lbState is a project p with namespaces a and b and a quota of 2 external addresses.
func lbState() *State {
	s := NewState()
	s.SetProjectQuota(pq("p", rl(lbAddresses, "2")))
	s.SetNamespace(ns("a", "p"))
	s.SetNamespace(ns("b", "p"))
	return s
}

func lbWrite(namespace, name, addresses string) ObjectInput {
	return ObjectInput{Family: FamilyLB, Namespace: namespace, Name: name, OldUsed: corev1.ResourceList{}, NewUsed: rl(lbAddresses, addresses)}
}

// Every load balancer counts its own addresses, also two of one namespace that share an address under
// one key: the ledger does not see the key.
func TestDecideObjectCountsEveryLoadBalancer(t *testing.T) {
	s := lbState()
	s.SetObject(FamilyLB, "a", "web", 10, rl(lbAddresses, "1"))
	assert.True(t, s.DecideObject(lbWrite("a", "web-udp", "1"), testNow, always).Allowed)
	assert.Equal(t, 1, s.ObjectPendingCount(), "the second Service takes an address of its own")
	assert.False(t, s.DecideObject(lbWrite("b", "dns", "1"), testNow, always).Allowed, "2 of 2 addresses are taken")
	s.SetObject(FamilyLB, "a", "web-udp", 11, rl(lbAddresses, "1"))

	s.DeleteObject(FamilyLB, "a", "web")
	assert.True(t, s.DecideObject(lbWrite("b", "dns", "1"), testNow, always).Allowed)
}

// A change of the classes of the cluster moves the usage of a Service without a new version of it.
func TestRefreshObject(t *testing.T) {
	services := func(class string) corev1.ResourceName { return evaluator.LBClassServices(class) }
	s := NewState()
	s.SetProjectQuota(pq("p", rl(string(services("l2-new")), "1")))
	s.SetNamespace(ns("a", "p"))
	s.SetObject(FamilyLB, "a", "web", 10, corev1.ResourceList{services("l2-old"): rl("x", "1")["x"]})
	assert.True(t, s.DecideObject(ObjectInput{Family: FamilyLB, Namespace: "a", Name: "dns", OldUsed: corev1.ResourceList{},
		NewUsed: corev1.ResourceList{services("l2-new"): rl("x", "1")["x"]}}, testNow, always).Allowed, "web is of another class")

	s = NewState()
	s.SetProjectQuota(pq("p", rl(string(services("l2-new")), "1")))
	s.SetNamespace(ns("a", "p"))
	s.SetObject(FamilyLB, "a", "web", 10, corev1.ResourceList{services("l2-old"): rl("x", "1")["x"]})
	s.RefreshObject(FamilyLB, "a", "web", corev1.ResourceList{services("l2-new"): rl("x", "1")["x"]})
	assert.False(t, s.DecideObject(ObjectInput{Family: FamilyLB, Namespace: "a", Name: "dns", OldUsed: corev1.ResourceList{},
		NewUsed: corev1.ResourceList{services("l2-new"): rl("x", "1")["x"]}}, testNow, always).Allowed, "web moved to the new default class")
}
