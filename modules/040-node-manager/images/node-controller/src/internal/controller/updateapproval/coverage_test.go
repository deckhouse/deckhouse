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

package updateapproval

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	ua "github.com/deckhouse/node-controller/internal/controller/updateapproval/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func TestSecretToAllNodeGroups_ReturnsRequestPerNodeGroup(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1.AddToScheme(scheme); err != nil {
		t.Fatalf("add v1 scheme: %v", err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(
		&v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}},
		&v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "master"}},
	).Build()
	r := &Reconciler{Base: register.Base{Client: cl}}

	reqs := r.secretToAllNodeGroups(context.Background(), &corev1.Secret{})
	if len(reqs) != 2 {
		t.Fatalf("expected 2 reconcile requests, got %d", len(reqs))
	}
	names := map[string]bool{}
	for _, req := range reqs {
		names[req.Name] = true
	}
	if !names["worker"] || !names["master"] {
		t.Fatalf("expected requests for worker and master, got %+v", names)
	}
}

// guards against accidental reuse: ensure the secret predicate helper is wired to the
// configuration-checksums secret name so the secret watch matches the right object.
func TestChecksumSecretConstantsMatch(t *testing.T) {
	if ua.ConfigurationChecksumsSecretName != nodecommon.ConfigurationChecksumsSecretName {
		t.Fatal("checksum secret name re-export drifted from internal/common")
	}
	if ua.MachineNamespace != nodecommon.MachineNamespace {
		t.Fatal("machine namespace re-export drifted from internal/common")
	}
}
