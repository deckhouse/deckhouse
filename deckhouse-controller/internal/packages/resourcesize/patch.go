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

// Package resourcesize translates the per-workload resource footprint declared
// in a package CR (spec.resourceRequests) - replicas, container resources and
// the disk sizes of volume claim templates - into nelm render patches, which
// nelm applies to the rendered manifests both when it renders the chart and when
// it installs the release.
//
// It knows nothing of the CR types: callers hand it plain requests.
package resourcesize

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/werf/nelm/v2/pkg/resource/spec"
)

// Request pins the resource footprint of one workload a package deploys. Kind
// and Name address the workload; both are required and together they are unique,
// which the CR schema enforces with a keyed list.
//
// The field added for disks is omitted when empty, so a request that does not
// use it encodes as before and the settings checksum built from it does not
// change.
type Request struct {
	// Kind of the workload, one of the kinds in podSpecPaths.
	Kind string `json:"kind"`
	// Name of the workload as the chart renders it.
	Name string `json:"name"`
	// Replicas to run, nil to keep what the chart renders. Only meaningful for
	// the kinds in replicaKinds.
	Replicas *int32 `json:"replicas,omitempty"`
	// Containers to resize, addressed by name.
	Containers []Container `json:"containers"`
	// VolumeClaimTemplates to resize, addressed by name. Only meaningful for a
	// StatefulSet.
	VolumeClaimTemplates []VolumeClaimTemplate `json:"volumeClaimTemplates,omitempty"`
}

// VolumeClaimTemplate pins the storage one volume claim template of a
// StatefulSet requests, a quantity in string form already validated by the CR
// schema. It sizes only the claims the StatefulSet creates from now on: the
// claims it has already created are not in the release.
type VolumeClaimTemplate struct {
	Name    string `json:"name"`
	Storage string `json:"storage"`
}

// Container pins the compute resources of one container inside a workload.
//
// Keys are Kubernetes resource names (cpu, memory, ephemeral-storage) and values
// are quantities in string form, already validated by the CR schema. The maps are
// merged key by key into whatever the chart rendered, so setting only memory
// leaves the chart's cpu alone; there is no way to unset a resource the chart
// declares.
type Container struct {
	Name     string            `json:"name"`
	Requests map[string]string `json:"requests,omitempty"`
	Limits   map[string]string `json:"limits,omitempty"`
}

// podSpecPaths maps a workload kind to the jq path of the pod spec inside it. A
// kind absent from this map is not a workload this package can resize.
var podSpecPaths = map[string]string{
	"Pod":         ".spec",
	"Deployment":  ".spec.template.spec",
	"StatefulSet": ".spec.template.spec",
	"DaemonSet":   ".spec.template.spec",
	"ReplicaSet":  ".spec.template.spec",
	"Job":         ".spec.template.spec",
	"CronJob":     ".spec.jobTemplate.spec.template.spec",
}

// replicaKinds are the kinds that carry spec.replicas. The CR schema rejects
// replicas on any other kind; this guard keeps the patch honest if a request
// reaches it from somewhere else.
var replicaKinds = map[string]struct{}{
	"Deployment":  {},
	"StatefulSet": {},
	"ReplicaSet":  {},
}

const (
	// annotationScaleDisabled on a rendered workload forbids changing its replicas.
	annotationScaleDisabled = "packages.deckhouse.io/scale-disabled"
	// annotationScaleDownDisabled on a rendered workload forbids lowering its replicas.
	annotationScaleDownDisabled = "packages.deckhouse.io/scale-down-disabled"
)

// scalingErrorKey marks the jq error value of a forbidden replicas change, so ScalingError tells
// it from any other failure of a render patch.
const scalingErrorKey = "scalingError"

// replicasProgram sets spec.replicas to %[1]d, failing the render with a scaling error when the
// chart forbids the change by annotation. The chart's value, 1 when unset as in Kubernetes, is the
// baseline; asking for that same value is not scaling.
const replicasProgram = `((.metadata.annotations // {}) as $annotations | (.spec.replicas // 1) as $current |
if $annotations[%[2]q] == "true" and %[1]d != $current
then error({%[4]q: "\(.kind) '\(.metadata.name)' does not allow its replicas to change"})
elif $annotations[%[3]q] == "true" and %[1]d < $current
then error({%[4]q: "\(.kind) '\(.metadata.name)' does not allow its replicas to go below \($current)"})
else .spec.replicas = %[1]d end)`

// ScalingError returns the message of a replicas change the chart forbids, found anywhere in err's
// chain; "" for any other error.
func ScalingError(err error) string {
	var valueErr gojq.ValueError
	if !errors.As(err, &valueErr) {
		return ""
	}

	value, ok := valueErr.Value().(map[string]any)
	if !ok {
		return ""
	}

	message, _ := value[scalingErrorKey].(string)

	return message
}

// Patches translates the requests into nelm render patches, one per request that
// changes anything.
//
// A request whose name holds a slash is left out: no Kubernetes object can be
// named that way, and nelm would read a name wrapped in slashes as a regexp,
// resizing every workload of the kind it matches.
func Patches(requests []Request) ([]spec.Patch, error) {
	patches := make([]spec.Patch, 0, len(requests))

	for _, request := range requests {
		if strings.Contains(request.Name, "/") {
			continue
		}

		program, err := jqProgram(request)
		if err != nil {
			return nil, fmt.Errorf("resize %s '%s': %w", request.Kind, request.Name, err)
		}

		if program == "" {
			continue
		}

		patches = append(patches, spec.Patch{
			Match: spec.ResourceMatcher{Kinds: []string{request.Kind}, Names: []string{request.Name}},
			Type:  spec.PatchTypeJQ,
			Patch: program,
		})
	}

	return patches, nil
}

// jqProgram builds the jq program that overlays one request onto the workload it
// addresses, or "" when the request changes nothing. User-supplied values reach
// the program only as JSON literals, which jq reads as its own literals.
func jqProgram(request Request) (string, error) {
	podSpec, ok := podSpecPaths[request.Kind]
	if !ok {
		return "", fmt.Errorf("unsupported workload kind")
	}

	// The CR schema rejects volume claim templates on any other kind, like
	// replicas below.
	if len(request.VolumeClaimTemplates) > 0 && request.Kind != "StatefulSet" {
		return "", fmt.Errorf("kind does not have volume claim templates")
	}

	var steps []string

	if request.Replicas != nil {
		if _, ok := replicaKinds[request.Kind]; !ok {
			return "", fmt.Errorf("kind does not have replicas")
		}

		steps = append(steps, fmt.Sprintf(replicasProgram, *request.Replicas, annotationScaleDisabled, annotationScaleDownDisabled, scalingErrorKey))
	}

	for _, container := range request.Containers {
		name, err := json.Marshal(container.Name)
		if err != nil {
			return "", fmt.Errorf("encode container name: %w", err)
		}

		// A container name is unique across both lists within one pod spec, so a
		// request naming an init container resizes it without having to say which
		// list it is in. []? yields nothing for an absent list, so a container the
		// workload does not have is ignored, as the CR field documents.
		resources := fmt.Sprintf("%s | (.containers[]?, .initContainers[]?) | select(.name == %s) | .resources", podSpec, name)

		lists := []struct {
			field      string
			quantities map[string]string
		}{
			{"requests", container.Requests},
			{"limits", container.Limits},
		}

		for _, list := range lists {
			// An empty list is skipped rather than merged: merging {} would still
			// create the resources the chart did not declare.
			if len(list.quantities) == 0 {
				continue
			}

			literal, err := json.Marshal(list.quantities)
			if err != nil {
				return "", fmt.Errorf("encode container '%s' %s: %w", container.Name, list.field, err)
			}

			// Object addition merges key by key, the right side winning, and treats
			// a missing list as empty.
			steps = append(steps, fmt.Sprintf("(%s.%s) += %s", resources, list.field, literal))
		}
	}

	for _, template := range request.VolumeClaimTemplates {
		// An empty size is skipped rather than written, like an empty list of a
		// container.
		if template.Storage == "" {
			continue
		}

		name, err := json.Marshal(template.Name)
		if err != nil {
			return "", fmt.Errorf("encode volume claim template name: %w", err)
		}

		storage, err := json.Marshal(template.Storage)
		if err != nil {
			return "", fmt.Errorf("encode volume claim template '%s' storage: %w", template.Name, err)
		}

		// []? yields nothing for an absent list, so a template the StatefulSet
		// does not have is ignored, as the CR field documents.
		steps = append(steps, fmt.Sprintf("(.spec.volumeClaimTemplates[]? | select(.metadata.name == %s) | .spec.resources.requests.storage) = %s", name, storage))
	}

	return strings.Join(steps, " | "), nil
}
