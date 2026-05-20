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

package common

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
)

func TestMappingToNodeGroup(t *testing.T) {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:   "n1",
		Labels: map[string]string{NodeGroupLabel: "ng-a"},
	}}
	reqs := NodeToNodeGroup(context.Background(), node)
	if len(reqs) != 1 || reqs[0].Name != "ng-a" {
		t.Fatalf("unexpected node mapping: %#v", reqs)
	}

	machine := &mcmv1alpha1.Machine{ObjectMeta: metav1.ObjectMeta{
		Labels: map[string]string{"node-group": "ng-b"},
	}}
	reqs = MachineToNodeGroup(context.Background(), machine)
	if len(reqs) != 1 || reqs[0].Name != "ng-b" {
		t.Fatalf("unexpected machine mapping: %#v", reqs)
	}
}
