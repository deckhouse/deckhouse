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

// Package resourcerequests validates the autoscaling ranges in spec.resourceRequests of an
// Application. The rules relate several fields, and the webhook checks them rather than the
// CRD schema so that each error names the workload and says how to fix it.
package resourcerequests

import (
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

// replicaKinds are the kinds that have replicas for a HorizontalPodAutoscaler to scale.
var replicaKinds = []string{"Deployment", "StatefulSet", "ReplicaSet"}

// boundedResources are the resources a VerticalPodAutoscaler sets, the only ones a range takes.
var boundedResources = []string{"cpu", "memory"}

// Validate returns the problems of the requests, a human-readable message each, in the order of
// the requests; none when the requests are valid.
func Validate(requests []v1alpha1.ApplicationResourceRequest) []string {
	var problems []string

	for _, request := range requests {
		workload := fmt.Sprintf("%s '%s'", request.Kind, request.Name)

		for _, problem := range replicasProblems(request) {
			problems = append(problems, workload+": "+problem)
		}

		for _, container := range request.Containers {
			for _, problem := range resourcesProblems(container.Resources) {
				problems = append(problems, fmt.Sprintf("%s, container '%s': %s", workload, container.Name, problem))
			}
		}
	}

	return problems
}

// replicasProblems checks the replicas range of the request: both bounds, in order, on a kind a
// HorizontalPodAutoscaler can scale, and in place of a fixed number of replicas.
func replicasProblems(request v1alpha1.ApplicationResourceRequest) []string {
	if request.MinReplicas == nil && request.MaxReplicas == nil {
		return nil
	}

	var problems []string

	if !slices.Contains(replicaKinds, request.Kind) {
		problems = append(problems, fmt.Sprintf("minReplicas and maxReplicas apply only to %s, as only those kinds have replicas", strings.Join(replicaKinds, ", ")))
	}

	if request.Replicas != nil {
		problems = append(problems, "replicas cannot be set together with minReplicas and maxReplicas. Set replicas to fix the number of replicas, or minReplicas and maxReplicas to let HorizontalPodAutoscaler choose it within the range")
	}

	if request.MinReplicas == nil || request.MaxReplicas == nil {
		problems = append(problems, fmt.Sprintf("set both minReplicas and maxReplicas: a range needs both bounds, but only %s is set", setBound(request.MinReplicas != nil, "minReplicas", "maxReplicas")))
		return problems
	}

	if *request.MinReplicas > *request.MaxReplicas {
		problems = append(problems, fmt.Sprintf("minReplicas (%d) is greater than maxReplicas (%d)", *request.MinReplicas, *request.MaxReplicas))
	}

	return problems
}

// resourcesProblems checks the range of a container: both bounds of the same resources, in order,
// and in place of fixed requests and limits of those resources.
func resourcesProblems(resources v1alpha1.ApplicationContainerResources) []string {
	if resources.Min == nil && resources.Max == nil {
		return nil
	}

	var problems []string

	if fixed := fixedResources(resources); len(fixed) > 0 {
		problems = append(problems, fmt.Sprintf("%s cannot be set together with min and max, as VerticalPodAutoscaler sets the requests within the range. Remove them, or replace min and max with requests and limits to fix the resources",
			strings.Join(fixed, " and ")))
	}

	if resources.Min == nil || resources.Max == nil {
		return append(problems, fmt.Sprintf("set both min and max: a range needs both bounds, but only %s is set", setBound(resources.Min != nil, "min", "max")))
	}

	lower, upper := bounds(resources.Min), bounds(resources.Max)

	if len(lower) == 0 && len(upper) == 0 {
		return append(problems, "min and max must set cpu, memory or both")
	}

	for _, resourceName := range boundedResources {
		low, hasLow := lower[resourceName]
		high, hasHigh := upper[resourceName]

		if hasLow != hasHigh {
			problems = append(problems, fmt.Sprintf("set %s in both min and max: a range needs both bounds, but %s is only in %s", resourceName, resourceName, setBound(hasLow, "min", "max")))
			continue
		}

		if hasLow && low.Cmp(high) > 0 {
			problems = append(problems, fmt.Sprintf("min %s (%s) is greater than max %s (%s)", resourceName, low.String(), resourceName, high.String()))
		}
	}

	return problems
}

// fixedResources names the cpu and memory requests and limits of the container, as in "cpu request".
func fixedResources(resources v1alpha1.ApplicationContainerResources) []string {
	var fixed []string

	for _, resourceName := range boundedResources {
		if _, ok := resources.Requests[corev1.ResourceName(resourceName)]; ok {
			fixed = append(fixed, resourceName+" request")
		}

		if _, ok := resources.Limits[corev1.ResourceName(resourceName)]; ok {
			fixed = append(fixed, resourceName+" limit")
		}
	}

	return fixed
}

// bounds returns the resources a bound sets, by name.
func bounds(bound *v1alpha1.ApplicationContainerResourceBounds) map[string]resource.Quantity {
	set := make(map[string]resource.Quantity)

	if bound.CPU != nil {
		set["cpu"] = *bound.CPU
	}

	if bound.Memory != nil {
		set["memory"] = *bound.Memory
	}

	return set
}

// setBound names the bound that is set of a pair, lower when isLower.
func setBound(isLower bool, lower, upper string) string {
	if isLower {
		return lower
	}

	return upper
}
