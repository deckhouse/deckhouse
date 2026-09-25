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

// Package resourcerequests overlays the per-workload resource footprint declared
// in a package CR (spec.resourceRequests) onto the manifests a chart rendered.
//
// It is a leaf: it knows neither the CR types nor nelm. Callers hand it plain
// unstructured resources, which it mutates in place.
package resourcerequests

import (
	"fmt"
	"maps"
	"reflect"
	"slices"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Request pins the resource footprint of one workload a package deploys. Kind
// and Name address the workload; both are required and together they are unique,
// which the CR schema enforces with a keyed list.
type Request struct {
	// Kind of the workload, one of the kinds in podSpecPaths.
	Kind string
	// Name of the workload as the chart renders it.
	Name string
	// Replicas to run, nil to keep what the chart renders. Only meaningful for
	// the kinds in replicaKinds.
	Replicas *int32
	// Containers to resize, addressed by name.
	Containers []Container
}

// Container pins the compute resources of one container inside a workload.
//
// Keys are Kubernetes resource names (cpu, memory, ephemeral-storage) and values
// are quantities in string form, already validated by the CR schema. The maps are
// merged key by key into whatever the chart rendered, so setting only memory
// leaves the chart's cpu alone; there is no way to unset a resource the chart
// declares.
type Container struct {
	Name     string
	Requests map[string]string
	Limits   map[string]string
}

// podSpecPaths maps a workload kind to the path of the pod spec inside it. A kind
// absent from this map is not a workload this package can resize.
var podSpecPaths = map[string][]string{
	"Pod":         {"spec"},
	"Deployment":  {"spec", "template", "spec"},
	"StatefulSet": {"spec", "template", "spec"},
	"DaemonSet":   {"spec", "template", "spec"},
	"ReplicaSet":  {"spec", "template", "spec"},
	"Job":         {"spec", "template", "spec"},
	"CronJob":     {"spec", "jobTemplate", "spec", "template", "spec"},
}

// replicaKinds are the kinds that carry spec.replicas. The CR schema rejects
// replicas on any other kind; this guard keeps the overlay honest if a request
// reaches it from somewhere else.
var replicaKinds = map[string]struct{}{
	"Deployment":  {},
	"StatefulSet": {},
	"ReplicaSet":  {},
}

// containerFields are the container lists a request may address. A container name
// is unique across both lists within one pod spec, so a request naming an init
// container resizes it without having to say which list it is in.
var containerFields = []string{"containers", "initContainers"}

// Equal reports whether two request sets describe the same footprint. It is
// order-sensitive: reordering the list in the CR reads as a change here, which
// costs one reschedule but no apply, because the manifests it renders are
// identical and the release checksum does not move.
func Equal(a, b []Request) bool {
	return reflect.DeepEqual(a, b)
}

// Apply overlays the requests onto the rendered resources, mutating them in place.
//
// It returns the requests that matched no rendered workload. That is not an
// error — a chart may render a workload conditionally, so a request can be
// legitimately dormant — but it is worth surfacing, since a typo in kind or name
// looks exactly the same from here.
func Apply(resources []*unstructured.Unstructured, requests []Request) ([]Request, error) {
	if len(requests) == 0 {
		return nil, nil
	}

	matched := make(map[int]struct{}, len(requests))

	for _, resource := range resources {
		if resource == nil {
			continue
		}

		for i, request := range requests {
			if resource.GetKind() != request.Kind || resource.GetName() != request.Name {
				continue
			}

			if err := applyOne(resource, request); err != nil {
				return nil, fmt.Errorf("resize %s '%s': %w", request.Kind, request.Name, err)
			}

			matched[i] = struct{}{}
		}
	}

	var unmatched []Request
	for i, request := range requests {
		if _, ok := matched[i]; !ok {
			unmatched = append(unmatched, request)
		}
	}

	return unmatched, nil
}

// applyOne overlays a single request onto the workload it addresses.
func applyOne(resource *unstructured.Unstructured, request Request) error {
	path, ok := podSpecPaths[request.Kind]
	if !ok {
		return fmt.Errorf("unsupported workload kind")
	}

	if request.Replicas != nil {
		if _, ok := replicaKinds[request.Kind]; !ok {
			return fmt.Errorf("kind does not have replicas")
		}

		if err := unstructured.SetNestedField(resource.Object, int64(*request.Replicas), "spec", "replicas"); err != nil {
			return fmt.Errorf("set replicas: %w", err)
		}
	}

	if len(request.Containers) == 0 {
		return nil
	}

	podSpec, found, err := unstructured.NestedMap(resource.Object, path...)
	if err != nil {
		return fmt.Errorf("read pod spec: %w", err)
	}

	if !found {
		return nil
	}

	for _, container := range request.Containers {
		if err = resizeContainer(podSpec, container); err != nil {
			return fmt.Errorf("resize container '%s': %w", container.Name, err)
		}
	}

	if err = unstructured.SetNestedMap(resource.Object, podSpec, path...); err != nil {
		return fmt.Errorf("write pod spec: %w", err)
	}

	return nil
}

// resizeContainer merges the container's resources into every list of podSpec
// that holds a container of that name. A container the workload does not have is
// ignored, as the CR field documents.
func resizeContainer(podSpec map[string]any, container Container) error {
	if len(container.Requests) == 0 && len(container.Limits) == 0 {
		return nil
	}

	for _, field := range containerFields {
		entries, found, err := unstructured.NestedSlice(podSpec, field)
		if err != nil {
			return fmt.Errorf("read %s: %w", field, err)
		}

		if !found {
			continue
		}

		changed := false

		for _, entry := range entries {
			spec, ok := entry.(map[string]any)
			if !ok {
				continue
			}

			name, ok := spec["name"].(string)
			if !ok || name != container.Name {
				continue
			}

			mergeResources(spec, container)

			changed = true
		}

		if !changed {
			continue
		}

		if err = unstructured.SetNestedSlice(podSpec, entries, field); err != nil {
			return fmt.Errorf("write %s: %w", field, err)
		}
	}

	return nil
}

// mergeResources merges the requested quantities into the container's resources,
// creating the maps the container is missing.
func mergeResources(containerSpec map[string]any, container Container) {
	resources, ok := containerSpec["resources"].(map[string]any)
	if !ok {
		resources = make(map[string]any, 2)
	}

	mergeQuantities(resources, "requests", container.Requests)
	mergeQuantities(resources, "limits", container.Limits)

	containerSpec["resources"] = resources
}

// mergeQuantities merges quantities into one resource list (requests or limits).
func mergeQuantities(resources map[string]any, field string, quantities map[string]string) {
	if len(quantities) == 0 {
		return
	}

	list, ok := resources[field].(map[string]any)
	if !ok {
		list = make(map[string]any, len(quantities))
	}

	for _, name := range slices.Sorted(maps.Keys(quantities)) {
		list[name] = quantities[name]
	}

	resources[field] = list
}
