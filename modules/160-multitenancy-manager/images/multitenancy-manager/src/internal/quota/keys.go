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

// Package quota holds what the ProjectQuota controller and webhooks share: which keys a project quota
// accepts, how the module enforcement and the mode of a quota map to the label of a namespace, and how
// the usage of the namespaces adds up to the status of a ProjectQuota.
package quota

import (
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"controller/internal/quota/evaluator"
)

const (
	// ProjectNamespacesAlias is the one alias a quota accepts. The controller expands it into
	// ProjectNamespacesKey when it writes the ResourceQuota objects and leaves the object as written,
	// so a GitOps tool does not see a difference to sync.
	ProjectNamespacesAlias corev1.ResourceName = "projectNamespaces"
	// ProjectNamespacesKey counts the ProjectNamespace objects of the project. They live in the main
	// namespace only, so the usage of the project is that of its main namespace.
	ProjectNamespacesKey corev1.ResourceName = "count/projectnamespaces.deckhouse.io"

	// ReservedPrefix is kept for the project-level keys that no ResourceQuota counts.
	ReservedPrefix = "projectquota.deckhouse.io/"

	// VirtualMachineGroup and VirtualMachineResource are the objects the vm family counts.
	VirtualMachineGroup    = "virtualization.deckhouse.io"
	VirtualMachineResource = "virtualmachines"
)

// standardKeys are the keys without a slash that a ResourceQuota counts in a namespace. resourcequotas
// is not among them: d8-project-quota takes one of it in every namespace by itself.
var standardKeys = []corev1.ResourceName{
	corev1.ResourceCPU,
	corev1.ResourceMemory,
	corev1.ResourceEphemeralStorage,
	corev1.ResourcePods,
	corev1.ResourceRequestsCPU,
	corev1.ResourceRequestsMemory,
	corev1.ResourceRequestsEphemeralStorage,
	corev1.ResourceRequestsStorage,
	corev1.ResourceLimitsCPU,
	corev1.ResourceLimitsMemory,
	corev1.ResourceLimitsEphemeralStorage,
	corev1.ResourceServices,
	corev1.ResourceServicesLoadBalancers,
	corev1.ResourceServicesNodePorts,
	corev1.ResourcePersistentVolumeClaims,
	corev1.ResourceConfigMaps,
	corev1.ResourceSecrets,
	corev1.ResourceReplicationControllers,
}

// futureFamilies are the parts of the keys of the project-level families, which no ResourceQuota
// counts. The keys of a family the quota counts (evaluator.IsVMKey) are accepted; the others are
// refused rather than taken for extended resources.
var futureFamilies = []string{
	".virtualization.deckhouse.io/",
	".loadbalancerclass.network.deckhouse.io/",
	".loadbalancerpool.network.deckhouse.io/",
	"loadbalancers.network.deckhouse.io/",
}

// Served says whether and how the API server serves the resource a count/ key names.
type Served int

const (
	ServedUnknown Served = iota
	// ServedNamespaced is a namespaced resource the resource quota controller can count.
	ServedNamespaced
	// ServedClusterScoped is a cluster-scoped resource, which no ResourceQuota counts.
	ServedClusterScoped
	// ServedNotCountable is a namespaced resource without one of the verbs the resource quota
	// controller needs: create, list, watch and delete.
	ServedNotCountable
	// ServedResourceMissing is a resource the group does not have, a typo.
	ServedResourceMissing
	// ServedGroupNotServed is a group the API server does not serve, the one of a module that is not
	// enabled yet.
	ServedGroupNotServed
)

// Discovery tells how the API server serves a resource.
type Discovery interface {
	Lookup(group, resource string) (Served, error)
}

// KeyCheck is the outcome of checking the keys of a quota.
type KeyCheck struct {
	// Denied are the reasons to refuse the quota.
	Denied []string
	// Warnings are kept keys that would be refused now, and keys of groups that are not served yet.
	Warnings []string
	// NotServed are the count/ keys of groups that are not served yet; nothing counts them.
	NotServed []string
}

// CheckKeys checks the keys of hard. A key that old has already is kept with a warning when it would
// be refused now, so that an object accepted by an earlier release stays writable; only a key the
// write adds is checked strictly. A nil discovery skips the checks of count/ keys.
func CheckKeys(hard, old corev1.ResourceList, discovery Discovery) (KeyCheck, error) {
	var check KeyCheck
	if _, alias := hard[ProjectNamespacesAlias]; alias {
		if _, canonical := hard[ProjectNamespacesKey]; canonical {
			check.Denied = append(check.Denied, fmt.Sprintf("%s and %s are the same limit; set one of them", ProjectNamespacesAlias, ProjectNamespacesKey))
		}
	}

	keys := make([]corev1.ResourceName, 0, len(hard))
	for key := range hard {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	for _, key := range keys {
		reason, served, err := keyRefusal(key, discovery)
		if err != nil {
			return KeyCheck{}, err
		}
		if served == ServedGroupNotServed {
			check.NotServed = append(check.NotServed, string(key))
			check.Warnings = append(check.Warnings, fmt.Sprintf("%s: the API server does not serve this group yet, so nothing counts the key until the module that provides it is enabled", key))
			continue
		}
		if reason == "" {
			continue
		}
		if _, kept := old[key]; kept {
			check.Warnings = append(check.Warnings, fmt.Sprintf("%s: %s; the key is kept because the object has it already", key, reason))
			continue
		}
		check.Denied = append(check.Denied, fmt.Sprintf("%s: %s", key, reason))
	}
	if warning := vmAndPodKeys(hard); warning != "" {
		check.Warnings = append(check.Warnings, warning)
	}
	return check, nil
}

// vmAndPodKeys warns about a quota that limits virtual machines by their specification and pods by
// their requests or limits at once: the pod of a virtual machine counts in the keys of the pods too,
// twice during a migration, so a project with virtual machines reaches such a limit sooner than its
// virtual machines say.
func vmAndPodKeys(hard corev1.ResourceList) string {
	var vm, pod bool
	for key := range hard {
		name := string(key)
		switch {
		case evaluator.IsVMKey(key):
			vm = true
		case key == corev1.ResourceCPU || key == corev1.ResourceMemory || strings.HasPrefix(name, "requests.") || strings.HasPrefix(name, "limits."):
			pod = true
		}
	}
	if !vm || !pod {
		return ""
	}
	return "the quota limits virtual machines by their specification and pods by their requests or limits: the pod of a virtual machine counts in " +
		"the keys of pods too, twice while the machine migrates, so the project reaches those limits sooner than its virtual machines say"
}

// VMHard returns the limits of a canonical quota of the vm family.
func VMHard(canonical corev1.ResourceList) corev1.ResourceList {
	out := corev1.ResourceList{}
	for key, value := range canonical {
		if evaluator.IsVMKey(key) {
			out[key] = value.DeepCopy()
		}
	}
	return out
}

// keyRefusal returns why the key is refused, empty when it is accepted, and how its resource is
// served for a count/ key.
func keyRefusal(key corev1.ResourceName, discovery Discovery) (string, Served, error) {
	name := string(key)
	switch {
	case key == ProjectNamespacesAlias:
		return "", ServedUnknown, nil
	case name == "resourcequotas" || name == "count/resourcequotas":
		return "the d8-project-quota ResourceQuota takes one in every namespace of the project by itself; this key cannot be limited", ServedUnknown, nil
	case strings.HasPrefix(name, ReservedPrefix):
		return "the " + ReservedPrefix + " prefix is reserved and not implemented yet", ServedUnknown, nil
	case strings.HasPrefix(name, "count/"):
		return countKeyRefusal(strings.TrimPrefix(name, "count/"), discovery)
	case evaluator.IsVMKey(key):
		if discovery == nil {
			return "", ServedUnknown, nil
		}
		served, err := discovery.Lookup(VirtualMachineGroup, VirtualMachineResource)
		if err != nil {
			return "", ServedUnknown, fmt.Errorf("look up the virtual machines: %w", err)
		}
		return "", served, nil
	case isFutureFamily(name):
		return "the project quota does not count this family of keys yet", ServedUnknown, nil
	case slices.Contains(standardKeys, key),
		strings.HasPrefix(name, corev1.ResourceRequestsHugePagesPrefix),
		isExtendedRequest(name),
		isStorageClassKey(name),
		isDeviceClassKey(name):
		return "", ServedUnknown, nil
	}
	return "a ResourceQuota does not count this key; see the list of accepted keys in the documentation of ProjectQuota", ServedUnknown, nil
}

func countKeyRefusal(resource string, discovery Discovery) (string, Served, error) {
	if resource == "" {
		return "the key names no resource", ServedUnknown, nil
	}
	if discovery == nil {
		return "", ServedUnknown, nil
	}
	name, group, _ := strings.Cut(resource, ".")
	served, err := discovery.Lookup(group, name)
	if err != nil {
		return "", ServedUnknown, fmt.Errorf("look up the %q resource: %w", resource, err)
	}
	switch served {
	case ServedClusterScoped:
		return "the resource is cluster-scoped, and a ResourceQuota counts only namespaced objects", served, nil
	case ServedNotCountable:
		return "the resource lacks one of the verbs create, list, watch and delete, so the resource quota controller does not count it", served, nil
	case ServedResourceMissing:
		return "the API server serves the group but no such resource in it; check the spelling (the resource is plural and lowercase)", served, nil
	}
	return "", served, nil
}

func isFutureFamily(name string) bool {
	return slices.ContainsFunc(futureFamilies, func(part string) bool { return strings.Contains(name, part) })
}

// isExtendedRequest reports a request of an extended resource, such as requests.nvidia.com/gpu.
func isExtendedRequest(name string) bool {
	return strings.HasPrefix(name, corev1.DefaultResourceRequestsPrefix) && strings.Contains(name, "/")
}

// isStorageClassKey reports <class>.storageclass.storage.k8s.io/requests.storage and
// <class>.storageclass.storage.k8s.io/persistentvolumeclaims.
func isStorageClassKey(name string) bool {
	class, limit, ok := strings.Cut(name, ".storageclass.storage.k8s.io/")
	return ok && class != "" && (limit == string(corev1.ResourceRequestsStorage) || limit == string(corev1.ResourcePersistentVolumeClaims))
}

// isDeviceClassKey reports <class>.deviceclass.resource.k8s.io/devices.
func isDeviceClassKey(name string) bool {
	class, limit, ok := strings.Cut(name, ".deviceclass.resource.k8s.io/")
	return ok && class != "" && limit == "devices"
}

// Canonical returns hard with the alias expanded.
func Canonical(hard corev1.ResourceList) corev1.ResourceList {
	out := make(corev1.ResourceList, len(hard))
	for key, value := range hard {
		if key == ProjectNamespacesAlias {
			key = ProjectNamespacesKey
		}
		out[key] = value.DeepCopy()
	}
	return out
}

// ResourceQuotaHard returns the limits of a canonical quota that a ResourceQuota counts: everything
// but the project-level keys, which never go into one.
func ResourceQuotaHard(canonical corev1.ResourceList) corev1.ResourceList {
	out := make(corev1.ResourceList, len(canonical))
	for key, value := range canonical {
		name := string(key)
		if strings.HasPrefix(name, ReservedPrefix) || !strings.HasPrefix(name, "count/") && isFutureFamily(name) {
			continue
		}
		out[key] = value.DeepCopy()
	}
	return out
}
