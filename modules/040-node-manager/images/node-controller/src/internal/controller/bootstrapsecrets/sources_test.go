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

package bootstrapsecrets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

// The clusterUUID and the two names are the fixture and the golden of the helm
// template test (template_tests/module_test.go:46,801): proof the hash mirrored
// from capi still agrees with the names helm computed for the same inputs.
const (
	helmClusterUUID     = "f49dd1c3-a63a-4565-a06c-625e35587eab"
	helmZoneASecretName = "worker-02320933"
	helmZoneBSecretName = "worker-6bdb5b0d"
)

func TestCAPISecretNamesMatchTheHelmNames(t *testing.T) {
	r := newReconciler(t)

	names, err := r.capiSecretNames(t.Context(), nodeGroup("worker"), []string{"zonea", "zoneb"}, helmClusterUUID)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{helmZoneASecretName, helmZoneBSecretName}, names)
}

// A MachineDeployment created before the per-zone naming keeps its checksum-named
// Secret forever: dataSecretName lives in spec.template, so changing it replaces
// every node of the group. Its token has to keep being refreshed all the same.
func TestCAPISecretNamesKeepTheNameAnExistingMachineDeploymentCarries(t *testing.T) {
	r := newReconciler(t, machineDeployment("myprefix-worker-02320933", "worker", "worker-legacy-checksum"))

	names, err := r.capiSecretNames(t.Context(), nodeGroup("worker"), []string{"zonea"}, helmClusterUUID)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{helmZoneASecretName, "worker-legacy-checksum"}, names)
}

// An immutable group's MachineDeployment references a NodeBootstrapConfig instead
// of a data Secret, so it contributes no name.
func TestCAPISecretNamesSkipAMachineDeploymentWithoutDataSecret(t *testing.T) {
	r := newReconciler(t, machineDeployment("myprefix-worker-02320933", "worker", ""))

	names, err := r.capiSecretNames(t.Context(), nodeGroup("worker"), []string{"zonea"}, helmClusterUUID)

	require.NoError(t, err)
	assert.Equal(t, []string{helmZoneASecretName}, names)
}

func newReconciler(t *testing.T, objs ...*unstructured.Unstructured) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, deckhousev1.AddToScheme(scheme))

	builder := fake.NewClientBuilder().WithScheme(scheme)
	for _, obj := range objs {
		builder = builder.WithObjects(obj)
	}
	r := &Reconciler{}
	r.Client = builder.Build()
	return r
}

func nodeGroup(name string) *deckhousev1.NodeGroup {
	return &deckhousev1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

// machineDeployment builds a CAPI MachineDeployment; an empty dataSecretName
// stands for an immutable group, whose bootstrap is a configRef instead.
func machineDeployment(name, ngName, dataSecretName string) *unstructured.Unstructured {
	bootstrap := map[string]any{"configRef": map[string]any{"name": ngName}}
	if dataSecretName != "" {
		bootstrap = map[string]any{"dataSecretName": dataSecretName}
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cluster.x-k8s.io/v1beta2",
		"kind":       "MachineDeployment",
		"metadata": map[string]any{
			"name":      name,
			"namespace": nodecommon.MachineNamespace,
			"labels":    map[string]any{ngcommon.MachineDeploymentNodeGroupLabel: ngName},
		},
		"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"bootstrap": bootstrap}}},
	}}
}
