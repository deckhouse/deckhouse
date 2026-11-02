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

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"controller/internal/quota/evaluator"
)

// gpuMachine is a running virtual machine of the a namespace with the GPUs of the spec, at a
// generation the virtualization module has processed up to observed, with the changes that wait for a
// restart.
func gpuMachine(rv string, gpus int, generation, observed int64, awaiting ...any) *unstructured.Unstructured {
	list := make([]any, 0, gpus)
	for range gpus {
		list = append(list, map[string]any{"gpuClassName": "h100"})
	}
	vm := &unstructured.Unstructured{Object: map[string]any{
		"spec":   map[string]any{"cpu": map[string]any{"cores": int64(1), "coreFraction": "100%"}, "gpus": list},
		"status": map[string]any{"observedGeneration": observed, "restartAwaitingChanges": awaiting},
	}}
	vm.SetGroupVersionKind(virtualMachineGVK)
	vm.SetNamespace("a")
	vm.SetName("vm")
	vm.SetResourceVersion(rv)
	vm.SetGeneration(generation)
	return vm
}

// The GPUs of a running machine show nowhere but in its spec and in the changes that wait for a
// restart, which the virtualization module writes after the spec: a GPU taken out of the spec keeps
// its place until the module has said that the change waits for a restart, and frees it only once the
// machine runs without it.
func TestVirtualMachineKeepsItsGPUsUntilTheChangeIsObserved(t *testing.T) {
	r := &runner{state: vmState(), log: logr.Discard()}
	gpus := func() int64 {
		used, _ := r.state.ObjectUsed(FamilyVM, "a", "vm")
		value := used[evaluator.VMGPUs]
		return value.Value()
	}

	r.onVirtualMachine(gpuMachine("10", 1, 1, 1))
	assert.Equal(t, int64(1), gpus())

	r.onVirtualMachine(gpuMachine("11", 0, 2, 1))
	assert.Equal(t, int64(1), gpus(), "the spec without the GPU is not processed yet")

	r.onVirtualMachine(gpuMachine("12", 0, 2, 2, map[string]any{"operation": "remove", "path": "gpus",
		"currentValue": []any{map[string]any{"gpuClassName": "h100"}}}))
	assert.Equal(t, int64(1), gpus(), "the removal waits for a restart")

	r.onVirtualMachine(gpuMachine("13", 0, 2, 2))
	assert.Zero(t, gpus(), "restarted without the GPU")
}

// The quorum list of catch-up takes only the metadata of the virtual machines: the state covers such a
// list as it covers the whole machines, and the delete of a machine the informer showed removes the
// listed metadata too.
func TestCoversTheMetadataOfVirtualMachines(t *testing.T) {
	r := &runner{state: vmState(), log: logr.Discard()}
	listed := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Namespace: "a", Name: "vm", ResourceVersion: "10"}}
	listed.SetGroupVersionKind(virtualMachineGVK)
	assert.False(t, r.state.Covers([]metav1.Object{listed}), "the informer has not shown the machine yet")

	r.onVirtualMachine(gpuMachine("10", 0, 1, 1))
	assert.True(t, r.state.Covers([]metav1.Object{listed}))

	r.deleted.record(gpuMachine("11", 0, 1, 1))
	assert.Empty(t, r.deleted.remaining([]metav1.Object{listed}), "the delete of the machine covers its metadata")
}
