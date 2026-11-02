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
	"fmt"
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// The keys of the vm family: the virtual machines of a project by their specification.
const (
	// VMCores is the sum of spec.cpu.cores.
	VMCores corev1.ResourceName = "virtualmachines.virtualization.deckhouse.io/cores"
	// VMCPU is the guaranteed CPU: the sum of cores × coreFraction, a coreFraction left to the
	// autoscaler (Auto) or to the class counting as 100 %.
	VMCPU corev1.ResourceName = "virtualmachines.virtualization.deckhouse.io/cpu"
	// VMMemory is the sum of spec.memory.size.
	VMMemory corev1.ResourceName = "virtualmachines.virtualization.deckhouse.io/memory"
	// VMGPUs is the number of GPUs of spec.gpus.
	VMGPUs corev1.ResourceName = "virtualmachines.virtualization.deckhouse.io/gpus"

	// gpuClassSuffix ends the key of the GPUs of one GPUClass: <class>.gpuclass.virtualization.deckhouse.io/gpus.
	gpuClassSuffix = ".gpuclass.virtualization.deckhouse.io/gpus"

	runPolicyAlwaysOff = "AlwaysOff"
)

// VMKeys are the keys of the vm family other than the ones of a GPUClass.
var VMKeys = []corev1.ResourceName{VMCores, VMCPU, VMMemory, VMGPUs}

// IsVMKey reports whether the key belongs to the vm family.
func IsVMKey(key corev1.ResourceName) bool {
	name := string(key)
	for _, k := range VMKeys {
		if key == k {
			return true
		}
	}
	class, ok := strings.CutSuffix(name, gpuClassSuffix)
	return ok && class != "" && !strings.ContainsAny(class, "/")
}

// GPUClassKey is the key of the GPUs of the GPUClass.
func GPUClassKey(class string) corev1.ResourceName {
	return corev1.ResourceName(class + gpuClassSuffix)
}

// VirtualMachine is the family of the virtual machines of the virtualization module. Every virtual
// machine takes its place in the quota but one with runPolicy: AlwaysOff, which is kept switched off
// on purpose: a machine stopped by hand may be started at any moment, and the quota must not refuse
// that start.
type VirtualMachine struct{}

// Family is "vm".
func (VirtualMachine) Family() string { return "vm" }

// Usage is what the virtual machine adds to the project. The object is read as unstructured: the
// module does not depend on the API of the virtualization module.
//
// A machine keeps what it runs with until a restart applies a smaller spec: the usage is, key by key,
// the most of the spec, of the spec as it applies now (the current values of the changes that wait for
// a restart) and of the resources the machine runs with (status.resources). Without that, making the
// spec of a running machine smaller would free its quota for another machine while it still runs on
// the old size.
func (VirtualMachine) Usage(object runtime.Object) (corev1.ResourceList, error) {
	vm, ok := object.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("the vm family evaluates unstructured objects, got %T", object)
	}
	runPolicy, _, _ := unstructured.NestedString(vm.Object, "spec", "runPolicy")
	if runPolicy == runPolicyAlwaysOff {
		return corev1.ResourceList{}, nil
	}
	spec, _, _ := unstructured.NestedMap(vm.Object, "spec")
	usage, err := specUsage(spec)
	if err != nil {
		return nil, err
	}
	// the status is written by the virtualization module: a value it cannot be read with adds nothing
	if applied, ok := appliedSpec(vm.Object, spec); ok {
		if appliedUsage, err := specUsage(applied); err == nil {
			addMax(usage, appliedUsage)
		}
	}
	if running, err := statusUsage(vm.Object); err == nil {
		addMax(usage, running)
	}
	return usage, nil
}

// specUsage is what a spec of a virtual machine takes.
func specUsage(spec map[string]any) (corev1.ResourceList, error) {
	usage := corev1.ResourceList{}
	cores, _, err := unstructured.NestedFieldNoCopy(spec, "cpu", "cores")
	if err != nil {
		return nil, fmt.Errorf("read spec.cpu.cores: %w", err)
	}
	coreCount, err := wholeNumber(cores)
	if err != nil {
		return nil, fmt.Errorf("read spec.cpu.cores: %w", err)
	}
	fraction, _, _ := unstructured.NestedString(spec, "cpu", "coreFraction")
	percent, err := corePercent(fraction)
	if err != nil {
		return nil, err
	}
	usage[VMCores] = *resource.NewQuantity(coreCount, resource.DecimalSI)
	usage[VMCPU] = *resource.NewMilliQuantity(coreCount*10*percent, resource.DecimalSI)

	if size, found, _ := unstructured.NestedString(spec, "memory", "size"); found {
		memory, err := resource.ParseQuantity(size)
		if err != nil {
			return nil, fmt.Errorf("read spec.memory.size: %w", err)
		}
		usage[VMMemory] = memory
	}

	gpus, _, _ := unstructured.NestedSlice(spec, "gpus")
	if len(gpus) > 0 {
		usage[VMGPUs] = *resource.NewQuantity(int64(len(gpus)), resource.DecimalSI)
		for _, gpu := range gpus {
			entry, _ := gpu.(map[string]any)
			class, _ := entry["gpuClassName"].(string)
			if class == "" {
				continue
			}
			key := GPUClassKey(class)
			count := usage[key]
			count.Add(*resource.NewQuantity(1, resource.DecimalSI))
			usage[key] = count
		}
	}
	return usage, nil
}

// restartFields are the fields of the spec the quota counts. status.restartAwaitingChanges names a
// change by its path in the spec: a whole field such as cpu or gpus, with the field as its current
// value, or one in it such as cpu.cores or memory.size, with a scalar.
var restartFields = []string{"cpu", "memory", "gpus"}

// appliedSpec is the spec with the changes that wait for a restart taken back to their current values,
// and whether there is any such change.
func appliedSpec(vm, spec map[string]any) (map[string]any, bool) {
	changes, _, _ := unstructured.NestedSlice(vm, "status", "restartAwaitingChanges")
	applied := runtime.DeepCopyJSON(spec)
	changed := false
	for _, change := range changes {
		entry, _ := change.(map[string]any)
		path, _ := entry["path"].(string)
		fields := strings.Split(path, ".")
		if !slices.Contains(restartFields, fields[0]) {
			continue
		}
		changed = true
		current, ok := entry["currentValue"]
		if !ok {
			unstructured.RemoveNestedField(applied, fields...)
			continue
		}
		if err := unstructured.SetNestedField(applied, runtime.DeepCopyJSONValue(current), fields...); err != nil {
			continue
		}
	}
	return applied, changed
}

// statusUsage is what the machine runs with: its cores, their fraction and its memory in
// status.resources. The virtualization module fills them from the running machine, and from the spec
// while the machine is stopped.
func statusUsage(vm map[string]any) (corev1.ResourceList, error) {
	resources, found, _ := unstructured.NestedMap(vm, "status", "resources")
	if !found {
		return corev1.ResourceList{}, nil
	}
	cpu, _, _ := unstructured.NestedMap(resources, "cpu")
	memory, _, _ := unstructured.NestedMap(resources, "memory")
	usage, err := specUsage(map[string]any{"cpu": cpu, "memory": memory})
	if err != nil {
		return nil, err
	}
	// without the fraction the guaranteed CPU is not known, and counting whole cores would be too much
	if fraction, _, _ := unstructured.NestedString(cpu, "coreFraction"); fraction == "" {
		delete(usage, VMCPU)
	}
	return usage, nil
}

// addMax raises every key of usage to its value in other.
func addMax(usage, other corev1.ResourceList) {
	for key, value := range other {
		if current, ok := usage[key]; !ok || value.Cmp(current) > 0 {
			usage[key] = value.DeepCopy()
		}
	}
}

// TrimVirtualMachine keeps of a virtual machine only what the vm family counts it by, so that an
// informer of every virtual machine of the cluster does not hold their whole status.
func TrimVirtualMachine(object any) (any, error) {
	vm, ok := object.(*unstructured.Unstructured)
	if !ok {
		return object, nil
	}
	trimmed := &unstructured.Unstructured{Object: map[string]any{}}
	trimmed.SetAPIVersion(vm.GetAPIVersion())
	trimmed.SetKind(vm.GetKind())
	trimmed.SetName(vm.GetName())
	trimmed.SetNamespace(vm.GetNamespace())
	trimmed.SetUID(vm.GetUID())
	trimmed.SetResourceVersion(vm.GetResourceVersion())
	trimmed.SetDeletionTimestamp(vm.GetDeletionTimestamp())
	trimmed.SetGeneration(vm.GetGeneration())
	for _, path := range [][]string{
		{"spec", "cpu"}, {"spec", "memory"}, {"spec", "gpus"}, {"spec", "runPolicy"},
		{"status", "resources"}, {"status", "restartAwaitingChanges"}, {"status", "observedGeneration"},
	} {
		if value, found, _ := unstructured.NestedFieldNoCopy(vm.Object, path...); found {
			if err := unstructured.SetNestedField(trimmed.Object, runtime.DeepCopyJSONValue(value), path...); err != nil {
				return nil, fmt.Errorf("keep %s of a virtual machine: %w", strings.Join(path, "."), err)
			}
		}
	}
	return trimmed, nil
}

// corePercent is the guaranteed share of a core in percent: N% as it says, Auto or nothing as 100,
// the most the autoscaler or the class can give.
func corePercent(fraction string) (int64, error) {
	if fraction == "" || fraction == "Auto" {
		return 100, nil
	}
	digits, ok := strings.CutSuffix(fraction, "%")
	if !ok {
		return 0, fmt.Errorf("read spec.cpu.coreFraction %q: not a percentage", fraction)
	}
	percent, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || percent < 1 || percent > 100 {
		return 0, fmt.Errorf("read spec.cpu.coreFraction %q: not a percentage from 1 to 100", fraction)
	}
	return percent, nil
}

// wholeNumber reads a JSON number that is whole.
func wholeNumber(value any) (int64, error) {
	switch v := value.(type) {
	case nil:
		return 0, nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		if v != float64(int64(v)) {
			return 0, fmt.Errorf("%v is not a whole number", v)
		}
		return int64(v), nil
	}
	return 0, fmt.Errorf("%v is not a number", value)
}

// SpecUnobserved reports whether the virtualization module has not processed the latest spec of the
// virtual machine yet: until it has, status.restartAwaitingChanges does not show what a change of the
// spec waits for, and the usage of the new spec may be below what the machine runs with.
func SpecUnobserved(vm *unstructured.Unstructured) bool {
	observed, _, _ := unstructured.NestedFieldNoCopy(vm.Object, "status", "observedGeneration")
	generation, err := wholeNumber(observed)
	return err != nil || generation < vm.GetGeneration()
}
