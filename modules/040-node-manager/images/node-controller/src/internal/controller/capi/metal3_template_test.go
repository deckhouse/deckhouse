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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/cloudprovider"
	"github.com/deckhouse/node-controller/internal/machinetemplate"
)

func TestMetal3TemplateGenerations(t *testing.T) {
	stringField := func(obj *unstructured.Unstructured, fields ...string) string {
		t.Helper()
		value, found, err := unstructured.NestedString(obj.Object, fields...)
		require.NoError(t, err)
		require.True(t, found)
		return value
	}
	raw, err := os.ReadFile("../../../../../../../../ee/modules/030-infra-provider-baremetal/capi/template.yaml")
	require.NoError(t, err)
	contract, err := machinetemplate.ParseContract(raw)
	require.NoError(t, err)
	r := fakeReconciler(t)
	r.APIReader = r.Client
	input := machineTemplateGeneration{
		ng:       &deckhousev1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "workers"}},
		template: &cloudprovider.MachineTemplate{Contract: contract},
		baseName: "workers-zone",
		gvk:      schema.GroupVersionKind{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta1", Kind: "Metal3MachineTemplate"},
		data: cloudprovider.RenderData{
			NodeGroupName: "workers",
			Zone:          "default",
			Provider: map[string]any{"images": map[string]any{
				"ubuntu": map[string]any{
					"url":          "https://images.example.test/ubuntu.raw",
					"checksum":     "https://images.example.test/ubuntu.raw.sha256",
					"checksumType": "sha256",
				},
				"other": map[string]any{
					"url":          "https://images.example.test/other.raw",
					"checksum":     "https://images.example.test/other.raw.sha256",
					"checksumType": "sha256",
				},
			}},
			InstanceClass: map[string]any{
				"imageRef": map[string]any{"kind": "BareMetalImage", "name": "ubuntu"},
				"capacity": map[string]any{"cpu": "8", "memory": "16Gi"},
			},
		},
	}

	name, err := r.ensureMachineTemplateGeneration(t.Context(), input)
	require.NoError(t, err)
	require.Equal(t, "workers-zone-gen1", name)
	first, err := r.getMachineTemplate(t.Context(), input.gvk, name)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, "baremetal-metadata-v1", stringField(first, "spec", "template", "spec", "dataTemplate", "name"))
	assert.Equal(t, "metadata", stringField(first, "spec", "template", "spec", "automatedCleaningMode"))
	_, hasFormat, err := unstructured.NestedFieldNoCopy(first.Object, "spec", "template", "spec", "image", "format")
	require.NoError(t, err)
	assert.False(t, hasFormat, "format is optional in the image resource")
	_, hasInjectedImage := input.data.InstanceClass["image"]
	assert.False(t, hasInjectedImage, "InstanceClass must remain the user's spec")
	assert.Equal(t, "{}", first.GetAnnotations()[machinetemplate.AppliedProviderConfigAnnotation])

	other := input
	other.ng = &deckhousev1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "other-workers"}}
	other.baseName = "other-workers-zone"
	other.data.InstanceClass = map[string]any{"imageRef": map[string]any{"kind": "BareMetalImage", "name": "other"}}
	otherName, err := r.ensureMachineTemplateGeneration(t.Context(), other)
	require.NoError(t, err)
	other.currentName = otherName

	input.currentName = name
	images := input.data.Provider["images"].(map[string]any)
	originalImage := images["ubuntu"]
	delete(images, "ubuntu")
	retained, err := r.ensureMachineTemplateGeneration(t.Context(), input)
	require.NoError(t, err)
	assert.Equal(t, name, retained, "removing an image does not invalidate an existing generation")
	images["ubuntu"] = map[string]any{
		"url":          "https://images.example.test/recreated.raw",
		"checksum":     "https://images.example.test/recreated.raw.sha256",
		"checksumType": "sha256",
	}
	retained, err = r.ensureMachineTemplateGeneration(t.Context(), input)
	require.NoError(t, err)
	assert.Equal(t, name, retained, "recreating an image under the same name does not trigger rollout")
	stored, err := r.getMachineTemplate(t.Context(), input.gvk, retained)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, first.Object, stored.Object, "the original provisioning image remains frozen")
	images["ubuntu"] = originalImage

	input.data.InstanceClass["capacity"] = map[string]any{"cpu": "16", "memory": "32Gi"}
	unchanged, err := r.ensureMachineTemplateGeneration(t.Context(), input)
	require.NoError(t, err)
	assert.Equal(t, name, unchanged, "autoscaler capacity does not require reprovisioning")

	input.data.Provider["images"].(map[string]any)["ubuntu-new"] = map[string]any{
		"url":          "https://images.example.test/ubuntu-new.raw",
		"checksum":     "https://images.example.test/ubuntu-new.raw.sha256",
		"checksumType": "sha256",
	}
	input.data.InstanceClass["imageRef"] = map[string]any{"kind": "BareMetalImage", "name": "ubuntu-new"}
	imageGeneration, err := r.ensureMachineTemplateGeneration(t.Context(), input)
	require.NoError(t, err)
	require.Equal(t, "workers-zone-gen2", imageGeneration)
	unchangedOther, err := r.ensureMachineTemplateGeneration(t.Context(), other)
	require.NoError(t, err)
	assert.Equal(t, otherName, unchangedOther, "switching one group to a new image must not roll other groups")
	input.currentName = imageGeneration
	input.data.InstanceClass["hostSelector"] = map[string]any{
		"matchLabels": map[string]any{"pool": "workers"},
		"matchExpressions": []any{map[string]any{
			"key": "rack", "operator": "In", "values": []any{"rack-a", "rack-b"},
		}},
	}
	input.data.InstanceClass["disableAutomatedCleaning"] = true
	input.data.InstanceClass["nodeReuse"] = true
	nextName, err := r.ensureMachineTemplateGeneration(t.Context(), input)
	require.NoError(t, err)
	require.Equal(t, "workers-zone-gen3", nextName)
	next, err := r.getMachineTemplate(t.Context(), input.gvk, nextName)
	require.NoError(t, err)
	require.NotNil(t, next)
	assert.Equal(t, "baremetal-metadata-v1", stringField(next, "spec", "template", "spec", "dataTemplate", "name"))
	assert.Equal(t, "disabled", stringField(next, "spec", "template", "spec", "automatedCleaningMode"))
	selector, _, err := unstructured.NestedMap(next.Object, "spec", "template", "spec", "hostSelector")
	require.NoError(t, err)
	assert.Equal(t, input.data.InstanceClass["hostSelector"], selector)
	reuse, _, err := unstructured.NestedBool(next.Object, "spec", "nodeReuse")
	require.NoError(t, err)
	assert.True(t, reuse)

	previous, err := r.getMachineTemplate(t.Context(), input.gvk, name)
	require.NoError(t, err)
	require.NotNil(t, previous)
	assert.Equal(t, first.Object, previous.Object, "the old generation must remain unchanged")

	input.currentName = nextName
	input.data.InstanceClass["imageRef"] = map[string]any{"kind": "BareMetalImage", "name": "missing-image"}
	_, err = r.ensureMachineTemplateGeneration(t.Context(), input)
	require.ErrorContains(t, err, "referenced BareMetalImage is unavailable")
	preserved, err := r.getMachineTemplate(t.Context(), input.gvk, nextName)
	require.NoError(t, err)
	assert.Equal(t, next.Object, preserved.Object, "an unresolved new reference must leave existing generations intact")
	input.currentName = nextName
	input.data.InstanceClass["imageRef"] = map[string]any{"kind": "BareMetalImage", "name": "other"}
	switchedName, err := r.ensureMachineTemplateGeneration(t.Context(), input)
	require.NoError(t, err)
	assert.Equal(t, "workers-zone-gen4", switchedName)
	switched, err := r.getMachineTemplate(t.Context(), input.gvk, switchedName)
	require.NoError(t, err)
	assert.Equal(t, "https://images.example.test/other.raw", stringField(switched, "spec", "template", "spec", "image", "url"))

	input.currentName = switchedName
	input.rolloutID = "audit-rollout"
	manualName, err := r.ensureMachineTemplateGeneration(t.Context(), input)
	require.NoError(t, err)
	assert.Equal(t, "workers-zone-gen5", manualName, "manual-rollout-id must still force an update")
	manual, err := r.getMachineTemplate(t.Context(), input.gvk, manualName)
	require.NoError(t, err)
	assert.Equal(t, switched.Object["spec"], manual.Object["spec"], "a manual rollout preserves provisioning parameters")
}
