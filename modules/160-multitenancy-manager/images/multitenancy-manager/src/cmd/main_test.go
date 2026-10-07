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

package main

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// The startup wait lists pods and ReplicaSets through the uncached API reader of the manager, which
// uses this scheme. A kind missing from it fails every check, and the wait holds each start of the
// controllers for its whole timeout.
func TestNewScheme_KnowsWhatTheStartupWaitReads(t *testing.T) {
	scheme, err := newScheme()
	if err != nil {
		t.Fatalf("build the scheme: %v", err)
	}
	for _, list := range []client.ObjectList{&corev1.PodList{}, &appsv1.ReplicaSetList{}} {
		if _, err := apiutil.GVKForObject(list, scheme); err != nil {
			t.Errorf("the scheme cannot read %T: %v", list, err)
		}
	}
}
