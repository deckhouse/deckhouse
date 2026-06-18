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

package checker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"d8.io/upmeter/pkg/kubernetes"
)

func Test_virtualImageManifest(t *testing.T) {
	manifest := virtualImageManifest(
		"agent-01",
		"test-ns",
		VirtualizationCreationProbeName,
		"alpine-3-23-bios-base",
		"https://example.com/alpine.qcow2",
	)

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	assert.NoError(t, err)

	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "alpine-3-23-bios-base", metadata["name"])
	assert.Equal(t, "test-ns", metadata["namespace"])
	labels := metadata["labels"].(map[string]interface{})
	assert.Equal(t, VirtualizationGroupName, labels["upmeter-group"])
	assert.Equal(t, VirtualizationCreationProbeName, labels["upmeter-probe"])

	spec := obj["spec"].(map[string]interface{})
	dataSource := spec["dataSource"].(map[string]interface{})
	http := dataSource["http"].(map[string]interface{})
	assert.Equal(t, "HTTP", dataSource["type"])
	assert.Equal(t, "https://example.com/alpine.qcow2", http["url"])
}

func Test_virtualDiskManifest(t *testing.T) {
	manifest := virtualDiskManifest("agent-01", "test-ns", VirtualizationCreationProbeName, "probe-disk", "upmeter-probe")

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	assert.NoError(t, err)

	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "probe-disk", metadata["name"])
	assert.Equal(t, "test-ns", metadata["namespace"])
	labels := metadata["labels"].(map[string]interface{})
	assert.Equal(t, VirtualizationGroupName, labels["upmeter-group"])
	assert.Equal(t, VirtualizationCreationProbeName, labels["upmeter-probe"])

	spec := obj["spec"].(map[string]interface{})
	dataSource := spec["dataSource"].(map[string]interface{})
	objectRef := dataSource["objectRef"].(map[string]interface{})
	assert.Equal(t, "VirtualImage", objectRef["kind"])
	assert.Equal(t, "upmeter-probe", objectRef["name"])
}

func Test_virtualMachineManifest(t *testing.T) {
	manifest := virtualMachineManifest("agent-01", "test-ns", VirtualizationCreationProbeName, "probe-vm", "probe-disk")

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	assert.NoError(t, err)

	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "probe-vm", metadata["name"])
	labels := metadata["labels"].(map[string]interface{})
	assert.Equal(t, VirtualizationGroupName, labels["upmeter-group"])
	assert.Equal(t, VirtualizationCreationProbeName, labels["upmeter-probe"])

	spec := obj["spec"].(map[string]interface{})
	assert.NotContains(t, spec, "virtualMachineClassName")
	assert.Equal(t, "AlwaysOn", spec["runPolicy"])

	cpu := spec["cpu"].(map[string]interface{})
	assert.EqualValues(t, 1, cpu["cores"])

	memory := spec["memory"].(map[string]interface{})
	assert.Equal(t, "256Mi", memory["size"])

	blockDeviceRefs := spec["blockDeviceRefs"].([]interface{})
	assert.Len(t, blockDeviceRefs, 1)
	ref := blockDeviceRefs[0].(map[string]interface{})
	assert.Equal(t, "VirtualDisk", ref["kind"])
	assert.Equal(t, "probe-disk", ref["name"])
}

func Test_virtualMachineOperationManifest(t *testing.T) {
	manifest := virtualMachineOperationManifest("agent-01", "test-ns", VirtualizationLifecycleProbeName, "probe-vm-evict", "probe-vm")

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	assert.NoError(t, err)

	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "probe-vm-evict", metadata["name"])
	assert.Equal(t, "test-ns", metadata["namespace"])
	labels := metadata["labels"].(map[string]interface{})
	assert.Equal(t, VirtualizationGroupName, labels["upmeter-group"])
	assert.Equal(t, VirtualizationLifecycleProbeName, labels["upmeter-probe"])

	spec := obj["spec"].(map[string]interface{})
	assert.Equal(t, "probe-vm", spec["virtualMachineName"])
	assert.Equal(t, "Evict", spec["type"])
}

func Test_blankVirtualDiskManifest(t *testing.T) {
	manifest := blankVirtualDiskManifest("agent-01", "test-ns", VirtualizationLifecycleProbeName, "probe-extra-disk", "50Mi")

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	assert.NoError(t, err)

	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "probe-extra-disk", metadata["name"])
	assert.Equal(t, "test-ns", metadata["namespace"])
	labels := metadata["labels"].(map[string]interface{})
	assert.Equal(t, VirtualizationGroupName, labels["upmeter-group"])
	assert.Equal(t, VirtualizationLifecycleProbeName, labels["upmeter-probe"])

	spec := obj["spec"].(map[string]interface{})
	pvc := spec["persistentVolumeClaim"].(map[string]interface{})
	assert.Equal(t, "50Mi", pvc["size"])
}

func Test_virtualMachineBlockDeviceAttachmentManifest(t *testing.T) {
	manifest := virtualMachineBlockDeviceAttachmentManifest(
		"agent-01",
		"test-ns",
		VirtualizationLifecycleProbeName,
		"probe-extra-disk-attachment",
		"probe-vm",
		"probe-extra-disk",
	)

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	assert.NoError(t, err)

	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "probe-extra-disk-attachment", metadata["name"])
	assert.Equal(t, "test-ns", metadata["namespace"])
	labels := metadata["labels"].(map[string]interface{})
	assert.Equal(t, VirtualizationGroupName, labels["upmeter-group"])
	assert.Equal(t, VirtualizationLifecycleProbeName, labels["upmeter-probe"])

	spec := obj["spec"].(map[string]interface{})
	assert.Equal(t, "probe-vm", spec["virtualMachineName"])
	blockDeviceRef := spec["blockDeviceRef"].(map[string]interface{})
	assert.Equal(t, "VirtualDisk", blockDeviceRef["kind"])
	assert.Equal(t, "probe-extra-disk", blockDeviceRef["name"])
}

func Test_unstructuredNestedString(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"phase": "Running",
		},
	}

	assert.Equal(t, "Running", unstructuredNestedString(obj, "status", "phase"))
	assert.Equal(t, "", unstructuredNestedString(obj, "status", "missing"))
}

func Test_unstructuredNestedStringSlice(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"availableNodes": []interface{}{"node-a", "node-b"},
		},
	}

	assert.Equal(t, []string{"node-a", "node-b"}, unstructuredNestedStringSlice(obj, "status", "availableNodes"))
	assert.Nil(t, unstructuredNestedStringSlice(obj, "status", "missing"))
}

func Test_unstructuredConditionStatus(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{
					"type":   "VirtualMachineClassReady",
					"status": "True",
				},
				map[string]interface{}{
					"type":   "AgentReady",
					"status": "False",
				},
			},
		},
	}

	assert.Equal(t, "False", unstructuredConditionStatus(obj, "AgentReady"))
	assert.Equal(t, "True", unstructuredConditionStatus(obj, "VirtualMachineClassReady"))
	assert.Equal(t, "", unstructuredConditionStatus(obj, "Missing"))
}

func Test_virtualMachineHasAttachedDisk(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"blockDeviceRefs": []interface{}{
				map[string]interface{}{
					"kind":     "VirtualDisk",
					"name":     "root",
					"attached": true,
				},
				map[string]interface{}{
					"kind":     "VirtualDisk",
					"name":     "extra",
					"attached": false,
				},
			},
		},
	}

	assert.True(t, virtualMachineHasAttachedDisk(obj, "root"))
	assert.False(t, virtualMachineHasAttachedDisk(obj, "extra"))
	assert.False(t, virtualMachineHasAttachedDisk(obj, "missing"))
}

func Test_resizeVirtualDisk(t *testing.T) {
	ctx := context.Background()
	access := kubernetes.FakeAccessor()
	checker := testVirtualMachineLifecycleChecker(access)

	createDynamicObject(t, access, virtualDiskGVR, "test-ns", map[string]interface{}{
		"apiVersion": "virtualization.deckhouse.io/v1alpha2",
		"kind":       "VirtualDisk",
		"metadata": map[string]interface{}{
			"name":      virtualizationExtraDiskName,
			"namespace": "test-ns",
		},
		"spec": map[string]interface{}{
			"persistentVolumeClaim": map[string]interface{}{
				"size": "50Mi",
			},
		},
	})

	err := checker.resizeVirtualDisk(ctx, virtualizationExtraDiskName, "100Mi")
	assert.NoError(t, err)

	obj, err := access.Kubernetes().Dynamic().
		Resource(virtualDiskGVR).
		Namespace("test-ns").
		Get(ctx, virtualizationExtraDiskName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "100Mi", unstructuredNestedString(obj.Object, "spec", "persistentVolumeClaim", "size"))
}

func Test_waitVirtualDiskCapacity(t *testing.T) {
	ctx := context.Background()
	access := kubernetes.FakeAccessor()
	checker := testVirtualMachineLifecycleChecker(access)

	createDynamicObject(t, access, virtualDiskGVR, "test-ns", map[string]interface{}{
		"apiVersion": "virtualization.deckhouse.io/v1alpha2",
		"kind":       "VirtualDisk",
		"metadata": map[string]interface{}{
			"name":      virtualizationExtraDiskName,
			"namespace": "test-ns",
		},
		"status": map[string]interface{}{
			"phase":    virtualizationPhaseReady,
			"capacity": "100Mi",
		},
	})

	err := checker.waitVirtualDiskCapacity(ctx, virtualizationExtraDiskName, "100Mi")
	assert.NoError(t, err)
}

func Test_cleanupVirtualMachineLifecycleResources(t *testing.T) {
	ctx := context.Background()
	access := kubernetes.FakeAccessor()
	checker := testVirtualMachineLifecycleChecker(access)

	_, err := access.Kubernetes().CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ns"},
	}, metav1.CreateOptions{})
	assert.NoError(t, err)

	createDynamicObject(t, access, virtualMachineBlockDeviceAttachmentGVR, "test-ns", map[string]interface{}{
		"apiVersion": "virtualization.deckhouse.io/v1alpha2",
		"kind":       "VirtualMachineBlockDeviceAttachment",
		"metadata": map[string]interface{}{
			"name":      virtualizationExtraDiskAttachmentName,
			"namespace": "test-ns",
		},
	})
	createDynamicObject(t, access, virtualMachineGVR, "test-ns", map[string]interface{}{
		"apiVersion": "virtualization.deckhouse.io/v1alpha2",
		"kind":       "VirtualMachine",
		"metadata": map[string]interface{}{
			"name":      virtualizationVMName,
			"namespace": "test-ns",
		},
	})
	createDynamicObject(t, access, virtualDiskGVR, "test-ns", map[string]interface{}{
		"apiVersion": "virtualization.deckhouse.io/v1alpha2",
		"kind":       "VirtualDisk",
		"metadata": map[string]interface{}{
			"name":      virtualizationDiskName,
			"namespace": "test-ns",
		},
	})
	createDynamicObject(t, access, virtualDiskGVR, "test-ns", map[string]interface{}{
		"apiVersion": "virtualization.deckhouse.io/v1alpha2",
		"kind":       "VirtualDisk",
		"metadata": map[string]interface{}{
			"name":      virtualizationExtraDiskName,
			"namespace": "test-ns",
		},
	})
	createDynamicObject(t, access, virtualImageGVR, "test-ns", map[string]interface{}{
		"apiVersion": "virtualization.deckhouse.io/v1alpha2",
		"kind":       "VirtualImage",
		"metadata": map[string]interface{}{
			"name":      VirtualizationImageName,
			"namespace": "test-ns",
		},
	})

	err = checker.cleanup(ctx)
	assert.NoError(t, err)

	_, err = access.Kubernetes().Dynamic().
		Resource(virtualMachineBlockDeviceAttachmentGVR).
		Namespace("test-ns").
		Get(ctx, virtualizationExtraDiskAttachmentName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "VirtualMachineBlockDeviceAttachment should be deleted")
	_, err = access.Kubernetes().Dynamic().
		Resource(virtualDiskGVR).
		Namespace("test-ns").
		Get(ctx, virtualizationExtraDiskName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "extra VirtualDisk should be deleted")
}

func testVirtualMachineLifecycleChecker(access kubernetes.Access) *virtualMachineLifecycleChecker {
	return &virtualMachineLifecycleChecker{
		access:                      access,
		namespace:                   "test-ns",
		virtualImageName:            VirtualizationImageName,
		waitVirtualDiskTimeout:      time.Millisecond,
		waitVirtualMachineTimeout:   time.Millisecond,
		waitDeletionTimeout:         time.Millisecond,
		waitNamespaceDeletedTimeout: time.Millisecond,
	}
}

func createDynamicObject(
	t *testing.T,
	access kubernetes.Access,
	gvr schema.GroupVersionResource,
	namespace string,
	object map[string]interface{},
) {
	t.Helper()

	_, err := access.Kubernetes().Dynamic().
		Resource(gvr).
		Namespace(namespace).
		Create(context.Background(), &unstructured.Unstructured{Object: object}, metav1.CreateOptions{})
	assert.NoError(t, err)
}
