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

package evaluator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func vm(spec map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "virtualization.deckhouse.io/v1alpha2", "kind": "VirtualMachine",
		"metadata": map[string]any{"name": "vm", "namespace": "team"}, "spec": spec,
	}}
}

func TestVirtualMachineUsage(t *testing.T) {
	q := resource.MustParse
	tests := []struct {
		name string
		spec map[string]any
		want corev1.ResourceList
	}{
		{name: "cores, a share of them and memory", spec: map[string]any{"cpu": map[string]any{"cores": int64(4), "coreFraction": "25%"}, "memory": map[string]any{"size": "8Gi"}},
			want: corev1.ResourceList{VMCores: q("4"), VMCPU: q("1"), VMMemory: q("8Gi")}},
		{name: "Auto counts as the whole cores", spec: map[string]any{"cpu": map[string]any{"cores": int64(2), "coreFraction": "Auto"}, "memory": map[string]any{"size": "1Gi"}},
			want: corev1.ResourceList{VMCores: q("2"), VMCPU: q("2"), VMMemory: q("1Gi")}},
		{name: "a share left to the class counts as the whole cores", spec: map[string]any{"cpu": map[string]any{"cores": float64(3)}},
			want: corev1.ResourceList{VMCores: q("3"), VMCPU: q("3")}},
		{name: "GPUs in all and by class", spec: map[string]any{"cpu": map[string]any{"cores": int64(1), "coreFraction": "100%"},
			"gpus": []any{map[string]any{"gpuClassName": "h100"}, map[string]any{"gpuClassName": "h100"}, map[string]any{"gpuClassName": "a10"}}},
			want: corev1.ResourceList{VMCores: q("1"), VMCPU: q("1"), VMGPUs: q("3"), GPUClassKey("h100"): q("2"), GPUClassKey("a10"): q("1")}},
		{name: "a machine kept switched off takes nothing", spec: map[string]any{"runPolicy": "AlwaysOff", "cpu": map[string]any{"cores": int64(8)}, "memory": map[string]any{"size": "8Gi"}},
			want: corev1.ResourceList{}},
		{name: "a machine stopped by hand keeps its place", spec: map[string]any{"runPolicy": "Manual", "cpu": map[string]any{"cores": int64(1), "coreFraction": "10%"}},
			want: corev1.ResourceList{VMCores: q("1"), VMCPU: q("100m")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VirtualMachine{}.Usage(vm(tt.spec))
			require.NoError(t, err)
			require.Len(t, got, len(tt.want))
			for key, value := range tt.want {
				have := got[key]
				assert.Zero(t, value.Cmp(have), "%s: want %s, got %s", key, value.String(), have.String())
			}
		})
	}
}

// A smaller spec frees nothing while the machine runs on the bigger one: the usage is the most of the
// spec, of the spec as it applies until a restart, and of what the machine runs with.
func TestVirtualMachineUsageUntilARestart(t *testing.T) {
	q := resource.MustParse
	small := map[string]any{"cpu": map[string]any{"cores": int64(2), "coreFraction": "100%"}, "memory": map[string]any{"size": "1Gi"}}
	tests := []struct {
		name   string
		spec   map[string]any
		status map[string]any
		want   corev1.ResourceList
	}{
		{name: "running on the old size", spec: small,
			status: map[string]any{"resources": map[string]any{"cpu": map[string]any{"cores": int64(8), "coreFraction": "100%"}, "memory": map[string]any{"size": "8Gi"}}},
			want:   corev1.ResourceList{VMCores: q("8"), VMCPU: q("8"), VMMemory: q("8Gi")}},
		{name: "changes waiting for a restart, as the virtualization module writes them", spec: small,
			status: map[string]any{"restartAwaitingChanges": []any{
				map[string]any{"operation": "replace", "path": "cpu.cores", "currentValue": float64(8), "desiredValue": float64(2)},
				map[string]any{"operation": "replace", "path": "memory.size", "currentValue": "8Gi", "desiredValue": "1Gi"},
				map[string]any{"operation": "remove", "path": "gpus", "currentValue": []any{map[string]any{"gpuClassName": "h100"}}},
				map[string]any{"operation": "replace", "path": "runPolicy", "currentValue": "AlwaysOn"},
			}},
			want: corev1.ResourceList{VMCores: q("8"), VMCPU: q("8"), VMMemory: q("8Gi"), VMGPUs: q("1"), GPUClassKey("h100"): q("1")}},
		{name: "cores and their share changed together", spec: small,
			status: map[string]any{"restartAwaitingChanges": []any{
				map[string]any{"operation": "replace", "path": "cpu", "currentValue": map[string]any{"cores": float64(8), "coreFraction": "50%"}, "desiredValue": small["cpu"]},
			}},
			want: corev1.ResourceList{VMCores: q("8"), VMCPU: q("4"), VMMemory: q("1Gi")}},
		{name: "a share of the cores changed alone", spec: map[string]any{"cpu": map[string]any{"cores": int64(4), "coreFraction": "25%"}},
			status: map[string]any{"restartAwaitingChanges": []any{
				map[string]any{"operation": "replace", "path": "cpu.coreFraction", "currentValue": "100%", "desiredValue": "25%"},
			}},
			want: corev1.ResourceList{VMCores: q("4"), VMCPU: q("4")}},
		{name: "a bigger spec counts at once", spec: map[string]any{"cpu": map[string]any{"cores": int64(16), "coreFraction": "100%"}, "memory": map[string]any{"size": "16Gi"}},
			status: map[string]any{"resources": map[string]any{"cpu": map[string]any{"cores": int64(8), "coreFraction": "100%"}, "memory": map[string]any{"size": "8Gi"}}},
			want:   corev1.ResourceList{VMCores: q("16"), VMCPU: q("16"), VMMemory: q("16Gi")}},
		{name: "a status without the fraction does not count the CPU", spec: small,
			status: map[string]any{"resources": map[string]any{"cpu": map[string]any{"cores": int64(4)}}},
			want:   corev1.ResourceList{VMCores: q("4"), VMCPU: q("2"), VMMemory: q("1Gi")}},
		{name: "a status that cannot be read adds nothing", spec: small,
			status: map[string]any{"resources": map[string]any{"memory": map[string]any{"size": "lots"}}, "restartAwaitingChanges": []any{"broken"}},
			want:   corev1.ResourceList{VMCores: q("2"), VMCPU: q("2"), VMMemory: q("1Gi")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			object := vm(tt.spec)
			object.Object["status"] = tt.status
			got, err := VirtualMachine{}.Usage(object)
			require.NoError(t, err)
			require.Len(t, got, len(tt.want))
			for key, value := range tt.want {
				have := got[key]
				assert.Zero(t, value.Cmp(have), "%s: want %s, got %s", key, value.String(), have.String())
			}
		})
	}
}

func TestVirtualMachineUsageRefusesWhatItCannotRead(t *testing.T) {
	for _, spec := range []map[string]any{
		{"cpu": map[string]any{"cores": int64(1), "coreFraction": "150%"}},
		{"cpu": map[string]any{"cores": "two"}},
		{"cpu": map[string]any{"cores": int64(1)}, "memory": map[string]any{"size": "lots"}},
	} {
		_, err := VirtualMachine{}.Usage(vm(spec))
		assert.Error(t, err, spec)
	}
}

func TestIsVMKey(t *testing.T) {
	for _, key := range []corev1.ResourceName{VMCores, VMCPU, VMMemory, VMGPUs, "h100.gpuclass.virtualization.deckhouse.io/gpus"} {
		assert.True(t, IsVMKey(key), key)
	}
	for _, key := range []corev1.ResourceName{"requests.cpu", "virtualmachines.virtualization.deckhouse.io/disks", ".gpuclass.virtualization.deckhouse.io/gpus", "count/virtualmachines.virtualization.deckhouse.io"} {
		assert.False(t, IsVMKey(key), key)
	}
}

// The informer keeps only what the vm family counts a virtual machine by, and the usage of the kept
// object is the usage of the whole one.
func TestTrimVirtualMachine(t *testing.T) {
	full := vm(map[string]any{
		"cpu": map[string]any{"cores": int64(2), "coreFraction": "50%"}, "memory": map[string]any{"size": "1Gi"},
		"gpus": []any{map[string]any{"gpuClassName": "h100"}}, "runPolicy": "Manual",
		"blockDeviceRefs": []any{map[string]any{"kind": "VirtualDisk", "name": "root"}},
	})
	full.SetResourceVersion("42")
	full.SetAnnotations(map[string]string{"big": "annotation"})
	full.Object["status"] = map[string]any{
		"resources":              map[string]any{"cpu": map[string]any{"cores": int64(4), "coreFraction": "50%"}, "memory": map[string]any{"size": "2Gi"}},
		"restartAwaitingChanges": []any{map[string]any{"path": "memory", "currentValue": map[string]any{"size": "2Gi"}}},
		"conditions":             []any{map[string]any{"type": "Running"}},
	}

	object, err := TrimVirtualMachine(full)
	require.NoError(t, err)
	trimmed, ok := object.(*unstructured.Unstructured)
	require.True(t, ok)

	assert.Equal(t, "42", trimmed.GetResourceVersion())
	assert.Equal(t, full.GroupVersionKind(), trimmed.GroupVersionKind())
	assert.Empty(t, trimmed.GetAnnotations())
	_, found, _ := unstructured.NestedFieldNoCopy(trimmed.Object, "spec", "blockDeviceRefs")
	assert.False(t, found)
	_, found, _ = unstructured.NestedFieldNoCopy(trimmed.Object, "status", "conditions")
	assert.False(t, found)

	want, err := VirtualMachine{}.Usage(full)
	require.NoError(t, err)
	got, err := VirtualMachine{}.Usage(trimmed)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// Until the virtualization module has processed the latest spec, the change a spec waits for is not
// known yet.
func TestSpecUnobserved(t *testing.T) {
	machine := vm(map[string]any{"cpu": map[string]any{"cores": int64(1)}})
	machine.SetGeneration(2)
	assert.True(t, SpecUnobserved(machine), "no status yet")
	machine.Object["status"] = map[string]any{"observedGeneration": int64(1)}
	assert.True(t, SpecUnobserved(machine))
	machine.Object["status"] = map[string]any{"observedGeneration": float64(2)}
	assert.False(t, SpecUnobserved(machine))

	object, err := TrimVirtualMachine(machine)
	require.NoError(t, err)
	trimmed, ok := object.(*unstructured.Unstructured)
	require.True(t, ok)
	assert.False(t, SpecUnobserved(trimmed), "the informer keeps the generations")
}
