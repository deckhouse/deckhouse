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

package capi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
)

// TestBuildStaticMachineTemplate_LabelSelector mirrors the helm
// node_group_static_or_hybrid_machine_template define for a NodeGroup whose
// staticInstances declares a labelSelector: name = ng.Name, the two-arg module
// labels on both metadata blocks (heritage/module/node-group), and the selector
// copied into spec.template.spec.labelSelector.
func TestBuildStaticMachineTemplate_LabelSelector(t *testing.T) {
	ng := &deckhousev1.NodeGroup{}
	ng.Name = "worker-static"
	ng.Spec.StaticInstances = &deckhousev1.StaticInstancesSpec{
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"role": "worker"},
		},
	}

	smt, err := buildStaticMachineTemplate(ng)
	require.NoError(t, err)

	assert.Equal(t, "infrastructure.cluster.x-k8s.io/v1alpha1", smt.Object["apiVersion"])
	assert.Equal(t, "StaticMachineTemplate", smt.Object["kind"])

	meta := smt.Object["metadata"].(map[string]interface{})
	assert.Equal(t, "worker-static", meta["name"], "name = ng.Name")
	assert.Equal(t, common.MachineNamespace, meta["namespace"])

	labels := meta["labels"].(map[string]interface{})
	assert.Equal(t, "deckhouse", labels["heritage"])
	assert.Equal(t, "node-manager", labels["module"])
	assert.Equal(t, "worker-static", labels["node-group"], "two-arg module labels add node-group")

	tmpl := smt.Object["spec"].(map[string]interface{})["template"].(map[string]interface{})
	tmeta := tmpl["metadata"].(map[string]interface{})
	assert.Equal(t, labels, tmeta["labels"], "template.metadata carries the same module labels")

	ls, found, err := unstructured.NestedStringMap(smt.Object, "spec", "template", "spec", "labelSelector", "matchLabels")
	require.NoError(t, err)
	require.True(t, found, "labelSelector.matchLabels present when set")
	assert.Equal(t, map[string]string{"role": "worker"}, ls)
}

// TestBuildStaticMachineTemplate_NoLabelSelector mirrors the define's else branch:
// no labelSelector key, an empty spec.template.spec (spec: {}).
func TestBuildStaticMachineTemplate_NoLabelSelector(t *testing.T) {
	ng := &deckhousev1.NodeGroup{}
	ng.Name = "worker-hybrid"
	ng.Spec.StaticInstances = &deckhousev1.StaticInstancesSpec{}

	smt, err := buildStaticMachineTemplate(ng)
	require.NoError(t, err)

	spec := smt.Object["spec"].(map[string]interface{})["template"].(map[string]interface{})["spec"].(map[string]interface{})
	assert.Empty(t, spec, "spec.template.spec is empty when no labelSelector")
	_, found := spec["labelSelector"]
	assert.False(t, found, "no labelSelector key when unset")
}
