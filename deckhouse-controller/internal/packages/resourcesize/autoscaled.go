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

package resourcesize

import (
	"fmt"
	"slices"
	"strings"

	"github.com/werf/nelm/v2/pkg/resource/spec"
)

// Autoscaled is what the autoscalers of a release control: the replicas a
// HorizontalPodAutoscaler picks and the container resources a VerticalPodAutoscaler
// sets. A range of a request applies only through them.
type Autoscaled struct {
	Replicas  []AutoscaledReplicas
	Resources []AutoscaledResources
}

// AutoscaledReplicas is a workload whose replicas a HorizontalPodAutoscaler picks.
type AutoscaledReplicas struct {
	Kind string
	Name string
}

// AutoscaledResources is a container of a workload whose Resources, cpu or memory, a
// VerticalPodAutoscaler sets.
type AutoscaledResources struct {
	Kind      string
	Name      string
	Container string
	Resources []string
}

// scales reports whether a HorizontalPodAutoscaler picks the replicas of the workload.
func (a Autoscaled) scales(kind, name string) bool {
	return slices.Contains(a.Replicas, AutoscaledReplicas{Kind: kind, Name: name})
}

// sizes reports whether a VerticalPodAutoscaler sets the resource of the container.
func (a Autoscaled) sizes(kind, name, container, resourceName string) bool {
	return slices.ContainsFunc(a.Resources, func(sized AutoscaledResources) bool {
		return sized.Kind == kind && sized.Name == name && sized.Container == container &&
			slices.Contains(sized.Resources, resourceName)
	})
}

// vpaResourceNames are the resources of vpaScales in a fixed order, so the fallbacks of a
// container come out the same way every render.
var vpaResourceNames = []string{"cpu", "memory"}

// Target is what an autoscaler scales: the replicas of a workload, or the cpu or memory
// of one of its containers.
type Target struct {
	Kind string
	Name string
	// Container is empty for the replicas.
	Container string
	// Resource is "replicas", cpu or memory.
	Resource string
}

// Autoscaler is the kind of autoscaler that scales the target.
func (t Target) Autoscaler() string {
	if t.Container == "" {
		return "HorizontalPodAutoscaler"
	}

	return "VerticalPodAutoscaler"
}

// Pins returns the targets the requests fix by value although an autoscaler of the
// release scales them: the patches pin the autoscaler to the value, so it stops
// scaling. Zero replicas pin nothing, as they go to the workload alone.
func Pins(requests []Request, autoscaled Autoscaled) []Target {
	var pins []Target

	for _, request := range requests {
		if request.Replicas != nil && *request.Replicas > 0 && autoscaled.scales(request.Kind, request.Name) {
			pins = append(pins, Target{Kind: request.Kind, Name: request.Name, Resource: "replicas"})
		}

		for _, container := range request.Containers {
			for _, resourceName := range vpaResourceNames {
				_, requested := container.Requests[resourceName]
				_, limited := container.Limits[resourceName]

				if !requested && !limited || !autoscaled.sizes(request.Kind, request.Name, container.Name, resourceName) {
					continue
				}

				pins = append(pins, Target{Kind: request.Kind, Name: request.Name, Container: container.Name, Resource: resourceName})
			}
		}
	}

	return pins
}

// FallbackPatches returns the targets whose range no autoscaler of the release applies,
// with the render patches that apply the ranges to their workloads as fixed values:
// the replicas as minReplicas, a resource as a request of its min and a limit of its
// max. A workload guards the replicas with its annotations as it does replicas asked
// for directly.
//
// The patches add to the ones Patches builds for the same requests, which leave the
// workload alone for a range.
func FallbackPatches(requests []Request, autoscaled Autoscaled) ([]Target, []spec.Patch, error) {
	var (
		fallbacks []Target
		patches   []spec.Patch
	)

	for _, request := range requests {
		// Like Patches, as nelm would read such a name as a regexp.
		if strings.Contains(request.Name, "/") {
			continue
		}

		fixed := Request{Kind: request.Kind, Name: request.Name}

		if request.MinReplicas != nil && !autoscaled.scales(request.Kind, request.Name) {
			fixed.Replicas = request.MinReplicas
			fallbacks = append(fallbacks, Target{Kind: request.Kind, Name: request.Name, Resource: "replicas"})
		}

		for _, container := range request.Containers {
			pinned := Container{Name: container.Name, Requests: make(map[string]string), Limits: make(map[string]string)}

			for _, resourceName := range vpaResourceNames {
				lower, hasLower := container.Min[resourceName]
				upper, hasUpper := container.Max[resourceName]

				if !hasLower && !hasUpper || autoscaled.sizes(request.Kind, request.Name, container.Name, resourceName) {
					continue
				}

				if hasLower {
					pinned.Requests[resourceName] = lower
				}

				if hasUpper {
					pinned.Limits[resourceName] = upper
				}

				fallbacks = append(fallbacks, Target{Kind: request.Kind, Name: request.Name, Container: container.Name, Resource: resourceName})
			}

			fixed.Containers = append(fixed.Containers, pinned)
		}

		program, err := jqProgram(fixed)
		if err != nil {
			return nil, nil, fmt.Errorf("fall back %s '%s': %w", request.Kind, request.Name, err)
		}

		if program == "" {
			continue
		}

		patches = append(patches, workloadPatch(fixed, program))
	}

	return fallbacks, patches, nil
}
