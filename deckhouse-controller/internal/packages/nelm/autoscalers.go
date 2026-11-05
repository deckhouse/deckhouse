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

package nelm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/resourcesize"
)

// vpaOff is the update mode of a VerticalPodAutoscaler, and the mode of a container
// policy, that leaves the resources alone.
const vpaOff = "Off"

// vpaResources are the resources a VerticalPodAutoscaler sets, all of them unless a
// container policy narrows them.
var vpaResources = []string{"cpu", "memory"}

// autoscaledKinds are the workload kinds a resource request addresses, read for the
// containers a VerticalPodAutoscaler of theirs sizes.
var autoscaledKinds = map[string]struct{}{
	"Deployment":  {},
	"StatefulSet": {},
	"DaemonSet":   {},
	"ReplicaSet":  {},
	"Job":         {},
	"CronJob":     {},
	"Pod":         {},
}

// targetRef is the workload an autoscaler targets, by kind and name as the patches of
// resourcesize address it.
type targetRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// renderedHPA is the part of a rendered HorizontalPodAutoscaler that tells what it scales.
type renderedHPA struct {
	Spec struct {
		ScaleTargetRef targetRef `json:"scaleTargetRef"`
	} `json:"spec"`
}

// renderedVPA is the part of a rendered VerticalPodAutoscaler that tells what it sizes.
type renderedVPA struct {
	Spec struct {
		TargetRef    targetRef `json:"targetRef"`
		UpdatePolicy struct {
			UpdateMode string `json:"updateMode"`
		} `json:"updatePolicy"`
		ResourcePolicy struct {
			ContainerPolicies []vpaContainerPolicy `json:"containerPolicies"`
		} `json:"resourcePolicy"`
	} `json:"spec"`
}

type vpaContainerPolicy struct {
	ContainerName       string   `json:"containerName"`
	Mode                string   `json:"mode"`
	ControlledResources []string `json:"controlledResources"`
}

// namedContainers are the containers of a pod spec, by name.
type namedContainers struct {
	Containers []struct {
		Name string `json:"name"`
	} `json:"containers"`
}

// renderedWorkload holds the regular containers of a rendered workload, wherever its
// kind keeps the pod spec: a Pod in spec, a CronJob in spec.jobTemplate, the rest in
// spec.template. Init containers are left out, as VPA does not size them.
type renderedWorkload struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		namedContainers
		Template struct {
			Spec namedContainers `json:"spec"`
		} `json:"template"`
		JobTemplate struct {
			Spec struct {
				Template struct {
					Spec namedContainers `json:"spec"`
				} `json:"template"`
			} `json:"spec"`
		} `json:"jobTemplate"`
	} `json:"spec"`
}

// containers returns the names of the workload's regular containers.
func (w *renderedWorkload) containers(kind string) []string {
	spec := w.Spec.Template.Spec

	switch kind {
	case "Pod":
		spec = w.Spec.namedContainers
	case "CronJob":
		spec = w.Spec.JobTemplate.Spec.Template.Spec
	}

	names := make([]string, 0, len(spec.Containers))
	for _, container := range spec.Containers {
		names = append(names, container.Name)
	}

	return names
}

// readAutoscaled reads what the autoscalers of a rendered multi-document YAML manifest
// control: the replicas of the workload a HorizontalPodAutoscaler scales, and the cpu
// and memory of the containers of the workload a VerticalPodAutoscaler sizes.
func readAutoscaled(renderedManifests string) (resourcesize.Autoscaled, error) {
	var (
		autoscaled resourcesize.Autoscaled
		vpas       []*renderedVPA
	)

	containers := make(map[targetRef][]string)

	dec := yaml.NewYAMLOrJSONDecoder(strings.NewReader(renderedManifests), 4096)

	for {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			if !errors.Is(err, io.EOF) {
				return resourcesize.Autoscaled{}, fmt.Errorf("decode rendered manifests: %w", err)
			}

			break
		}

		meta := new(typeMeta)
		if len(raw) == 0 || json.Unmarshal(raw, meta) != nil {
			continue
		}

		group, _, _ := strings.Cut(meta.APIVersion, "/")

		switch {
		case group == "autoscaling" && meta.Kind == "HorizontalPodAutoscaler":
			hpa := new(renderedHPA)
			if err := json.Unmarshal(raw, hpa); err != nil {
				return resourcesize.Autoscaled{}, fmt.Errorf("decode HorizontalPodAutoscaler: %w", err)
			}

			autoscaled.Replicas = append(autoscaled.Replicas, resourcesize.AutoscaledReplicas{
				Kind: hpa.Spec.ScaleTargetRef.Kind,
				Name: hpa.Spec.ScaleTargetRef.Name,
			})

		case group == "autoscaling.k8s.io" && meta.Kind == "VerticalPodAutoscaler":
			vpa := new(renderedVPA)
			if err := json.Unmarshal(raw, vpa); err != nil {
				return resourcesize.Autoscaled{}, fmt.Errorf("decode VerticalPodAutoscaler: %w", err)
			}

			vpas = append(vpas, vpa)

		default:
			if _, ok := autoscaledKinds[meta.Kind]; !ok {
				continue
			}

			workload := new(renderedWorkload)
			if err := json.Unmarshal(raw, workload); err != nil {
				return resourcesize.Autoscaled{}, fmt.Errorf("decode %s: %w", meta.Kind, err)
			}

			containers[targetRef{Kind: meta.Kind, Name: workload.Metadata.Name}] = workload.containers(meta.Kind)
		}
	}

	// A VerticalPodAutoscaler may come before its workload, so it is resolved once all is read.
	for _, vpa := range vpas {
		autoscaled.Resources = append(autoscaled.Resources, vpa.sized(containers[vpa.Spec.TargetRef])...)
	}

	return autoscaled, nil
}

// sized returns the containers whose resources the VerticalPodAutoscaler sets, out of the
// regular containers of its target: none in update mode Off, and none whose policy is Off.
func (v *renderedVPA) sized(containers []string) []resourcesize.AutoscaledResources {
	if v.Spec.UpdatePolicy.UpdateMode == vpaOff {
		return nil
	}

	var sized []resourcesize.AutoscaledResources

	for _, container := range containers {
		policy := v.policy(container)
		if policy.Mode == vpaOff {
			continue
		}

		resources := vpaResources
		if len(policy.ControlledResources) > 0 {
			resources = slices.DeleteFunc(slices.Clone(policy.ControlledResources), func(resourceName string) bool {
				return !slices.Contains(vpaResources, resourceName)
			})
		}

		sized = append(sized, resourcesize.AutoscaledResources{
			Kind:      v.Spec.TargetRef.Kind,
			Name:      v.Spec.TargetRef.Name,
			Container: container,
			Resources: resources,
		})
	}

	return sized
}

// policy returns the policy the VerticalPodAutoscaler applies to the container, as VPA picks
// it: the container's own, else the "*" one, else none, which sizes all resources.
func (v *renderedVPA) policy(container string) vpaContainerPolicy {
	var fallback vpaContainerPolicy

	for _, policy := range v.Spec.ResourcePolicy.ContainerPolicies {
		if policy.ContainerName == container {
			return policy
		}

		if policy.ContainerName == "*" {
			fallback = policy
		}
	}

	return fallback
}
