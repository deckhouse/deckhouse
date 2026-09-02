// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package admission

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
	"github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/internal/testprovider"
)

const (
	decodeTestModuleName = "cloud-provider-test"
	decodeTestKind       = "TestInstanceClass"
)

func decodeTestModuleConfigObject(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(moduleConfigGVK)
	if name != "" {
		obj.SetName(name)
	}
	obj.Object["spec"] = map[string]any{
		"enabled":  true,
		"version":  int64(2),
		"settings": map[string]any{"provider": map[string]any{"disabled": false}},
	}

	return obj
}

func decodeTestNodeGroupObject(name string, nodeType cpapi.NodeType) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(nodeGroupListGVK.GroupVersion().WithKind("NodeGroup"))
	obj.SetName(name)
	obj.Object["spec"] = map[string]any{
		"nodeType": string(nodeType),
		"cloudInstances": map[string]any{
			"classReference": map[string]any{"kind": decodeTestKind, "name": "worker-class"},
		},
	}

	return obj
}

func decodeTestInstanceClassObject(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(nodeGroupListGVK.GroupVersion().WithKind(decodeTestKind))
	obj.SetName(name)
	obj.Object["spec"] = map[string]any{"etcdDisk": map[string]any{"size": "10Gi"}}

	return obj
}

func TestDecodeModuleConfigObject(t *testing.T) {
	t.Parallel()

	moduleConfig, err := DecodeModuleConfigObject[*testprovider.Settings](
		decodeTestModuleName, decodeTestModuleConfigObject(decodeTestModuleName),
	)
	if err != nil {
		t.Fatalf("DecodeModuleConfigObject() error = %v", err)
	}
	if moduleConfig == nil {
		t.Fatal("DecodeModuleConfigObject() = nil, want decoded ModuleConfig")
	}
	if moduleConfig.Name != decodeTestModuleName {
		t.Fatalf("DecodeModuleConfigObject() name = %q, want %q", moduleConfig.Name, decodeTestModuleName)
	}
	if moduleConfig.Spec.Version != 2 || moduleConfig.Spec.Enabled == nil || !*moduleConfig.Spec.Enabled {
		t.Fatalf("DecodeModuleConfigObject() spec = %#v, want enabled version 2", moduleConfig.Spec)
	}
	if moduleConfig.Spec.Settings == nil {
		t.Fatal("DecodeModuleConfigObject() settings = nil, want decoded settings")
	}
}

// An admission object without metadata.name still has to be attributed to the module: rules
// report the ModuleConfig by name, and the reviewed object is the module's own config.
func TestDecodeModuleConfigObjectFallsBackToModuleName(t *testing.T) {
	t.Parallel()

	moduleConfig, err := DecodeModuleConfigObject[*testprovider.Settings](
		decodeTestModuleName, decodeTestModuleConfigObject(""),
	)
	if err != nil {
		t.Fatalf("DecodeModuleConfigObject() error = %v", err)
	}
	if moduleConfig.Name != decodeTestModuleName {
		t.Fatalf("DecodeModuleConfigObject() name = %q, want the module name", moduleConfig.Name)
	}
}

func TestDecodeModuleConfigObjectRejectsBrokenSpec(t *testing.T) {
	t.Parallel()

	broken := decodeTestModuleConfigObject(decodeTestModuleName)
	broken.Object["spec"] = "invalid"

	if _, err := DecodeModuleConfigObject[*testprovider.Settings](decodeTestModuleName, broken); err == nil ||
		!strings.Contains(err.Error(), "decode ModuleConfig") {
		t.Fatalf("DecodeModuleConfigObject() error = %v, want decode error", err)
	}
}

func TestDecodeNodeGroupObject(t *testing.T) {
	t.Parallel()

	nodeGroup, err := DecodeNodeGroupObject(decodeTestNodeGroupObject("worker", cpapi.NodeTypeCloudPermanent))
	if err != nil {
		t.Fatalf("DecodeNodeGroupObject() error = %v", err)
	}
	if nodeGroup.Name != "worker" || nodeGroup.Spec.NodeType != cpapi.NodeTypeCloudPermanent {
		t.Fatalf("DecodeNodeGroupObject() = %#v, want CloudPermanent worker", nodeGroup)
	}
	if nodeGroup.Spec.CloudInstances == nil || nodeGroup.Spec.CloudInstances.ClassReference == nil {
		t.Fatalf("DecodeNodeGroupObject() cloudInstances = %#v, want a class reference", nodeGroup.Spec.CloudInstances)
	}
	if nodeGroup.Spec.CloudInstances.ClassReference.Name != "worker-class" {
		t.Fatalf("DecodeNodeGroupObject() class reference = %#v", nodeGroup.Spec.CloudInstances.ClassReference)
	}
}

func TestDecodeNodeGroupObjectRejectsBrokenSpec(t *testing.T) {
	t.Parallel()

	broken := decodeTestNodeGroupObject("worker", cpapi.NodeTypeCloudPermanent)
	broken.Object["spec"] = "invalid"

	if _, err := DecodeNodeGroupObject(broken); err == nil || !strings.Contains(err.Error(), "decode NodeGroup") {
		t.Fatalf("DecodeNodeGroupObject() error = %v, want decode error", err)
	}
}

func TestDecodeInstanceClassObject(t *testing.T) {
	t.Parallel()

	instanceClass, err := DecodeInstanceClassObject[*testprovider.InstanceClass](decodeTestInstanceClassObject("worker-class"))
	if err != nil {
		t.Fatalf("DecodeInstanceClassObject() error = %v", err)
	}
	if instanceClass.GetName() != "worker-class" {
		t.Fatalf("DecodeInstanceClassObject() name = %q, want worker-class", instanceClass.GetName())
	}
	if instanceClass.GetEtcdDisk() == nil {
		t.Fatal("DecodeInstanceClassObject() GetEtcdDisk() = nil, want the decoded etcd disk")
	}
	if instanceClass.GroupVersionKind().Kind != decodeTestKind {
		t.Fatalf("DecodeInstanceClassObject() kind = %q, want %q", instanceClass.GroupVersionKind().Kind, decodeTestKind)
	}
}

func TestDecodeInstanceClassObjectRejectsBrokenSpec(t *testing.T) {
	t.Parallel()

	broken := decodeTestInstanceClassObject("worker-class")
	broken.Object["spec"] = "invalid"

	if _, err := DecodeInstanceClassObject[*testprovider.InstanceClass](broken); err == nil ||
		!strings.Contains(err.Error(), "decode instance class") {
		t.Fatalf("DecodeInstanceClassObject() error = %v, want decode error", err)
	}
}

// Typed objects arrive from the credential Secret surface, unstructured ones from every
// CRD surface, so both shapes have to decode.
func TestRuntimeObjectToMapAcceptsTypedAndUnstructured(t *testing.T) {
	t.Parallel()

	unstructuredObj := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "x"},
	}}
	got, err := runtimeObjectToMap(unstructuredObj)
	if err != nil {
		t.Fatalf("runtimeObjectToMap(unstructured) error = %v", err)
	}
	if got["metadata"] == nil {
		t.Fatalf("runtimeObjectToMap(unstructured) = %#v, want metadata", got)
	}

	typedObj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-credentials", Namespace: "d8-cloud-provider-test"},
		Type:       cpapi.CredentialsSecretType,
	}
	got, err = runtimeObjectToMap(typedObj)
	if err != nil {
		t.Fatalf("runtimeObjectToMap(typed) error = %v", err)
	}
	metadata, ok := got["metadata"].(map[string]any)
	if !ok || metadata["name"] != "d8-credentials" {
		t.Fatalf("runtimeObjectToMap(typed) = %#v, want the marshalled Secret", got)
	}
}

// A nil object decodes to an absent resource rather than an error: the builder steps decide
// whether an object is expected at all, so a webhook can hand over a missing old object without
// having to special-case it. Nothing reaches the state as a result — an absent InstanceClass and
// ModuleConfig stay nil, and a zero NodeGroup is dropped by the CloudPermanent filter.
func TestDecodeNilObjectYieldsAbsentResource(t *testing.T) {
	t.Parallel()

	var typedNil runtime.Object
	objMap, err := runtimeObjectToMap(typedNil)
	if err != nil || objMap != nil {
		t.Fatalf("runtimeObjectToMap(nil) = %#v, err = %v, want nil map without error", objMap, err)
	}

	moduleConfig, err := DecodeModuleConfigObject[*testprovider.Settings](decodeTestModuleName, nil)
	if err != nil || moduleConfig != nil {
		t.Fatalf("DecodeModuleConfigObject(nil) = %#v, err = %v, want absent ModuleConfig", moduleConfig, err)
	}

	instanceClass, err := DecodeInstanceClassObject[*testprovider.InstanceClass](nil)
	if err != nil {
		t.Fatalf("DecodeInstanceClassObject(nil) error = %v", err)
	}
	if !cpvalapi.IsResourceAbsent(instanceClass) {
		t.Fatalf("DecodeInstanceClassObject(nil) = %#v, want an absent InstanceClass", instanceClass)
	}

	nodeGroup, err := DecodeNodeGroupObject(nil)
	if err != nil {
		t.Fatalf("DecodeNodeGroupObject(nil) error = %v", err)
	}
	if nodeGroup.Spec.NodeType == cpapi.NodeTypeCloudPermanent {
		t.Fatalf("DecodeNodeGroupObject(nil) = %#v, want a zero NodeGroup", nodeGroup)
	}
}
