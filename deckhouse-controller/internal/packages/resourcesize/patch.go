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
// The footprint goes into the workload and into the autoscalers of the release
// that target it: a HorizontalPodAutoscaler or a VerticalPodAutoscaler is pinned
// to the requested values, so it keeps them instead of overriding them. A
// requested range goes into the autoscalers alone, which keep scaling the
// workload within it; a range no autoscaler of the release applies falls back
// to fixed values in the workload instead (FallbackPatches).
//
// It knows nothing of the CR types: callers hand it plain requests.
package resourcesize

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/werf/nelm/v2/pkg/resource/spec"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Request pins or bounds the resource footprint of one workload a package
// deploys. Kind and Name address the workload; both are required and together
// they are unique, which the CR schema enforces with a keyed list.
//
// The fields added for disks and ranges are omitted when empty, so a request
// that does not use them encodes as before and the settings checksum built from
// it does not change.
type Request struct {
	// Kind of the workload, one of the kinds in podSpecPaths.
	Kind string `json:"kind"`
	// Name of the workload as the chart renders it.
	Name string `json:"name"`
	// Replicas to run, nil to keep what the chart renders. Only meaningful for
	// the kinds in replicaKinds.
	Replicas *int32 `json:"replicas,omitempty"`
	// MinReplicas and MaxReplicas bound the replicas a HorizontalPodAutoscaler
	// of the workload picks, nil to keep the autoscaler's own bound. They
	// exclude Replicas and leave the workload's replicas as the chart renders
	// them, unless no autoscaler scales the workload: it then runs MinReplicas.
	// Only meaningful for the kinds in replicaKinds.
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`
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
//
// Min and Max bound the cpu and memory requests a VerticalPodAutoscaler of the
// workload picks for the container, keyed and valued the same way. They reach
// the autoscaler alone, unless none sizes the container: Min then becomes its
// request and Max its limit. They exclude cpu and memory in Requests and Limits.
type Container struct {
	Name     string            `json:"name"`
	Requests map[string]string `json:"requests,omitempty"`
	Limits   map[string]string `json:"limits,omitempty"`
	Min      map[string]string `json:"min,omitempty"`
	Max      map[string]string `json:"max,omitempty"`
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
	// annotationScaleDisabled on a rendered workload or HorizontalPodAutoscaler forbids
	// changing the workload's replicas.
	annotationScaleDisabled = "packages.deckhouse.io/scale-disabled"
	// annotationScaleDownDisabled on a rendered workload or HorizontalPodAutoscaler forbids
	// lowering the workload's replicas.
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

// hpaProgram runs %[5]s on every HorizontalPodAutoscaler that targets the workload of kind %[1]s
// and name %[2]s, to keep the workload's replicas between %[3]s and %[4]s, jq expressions that may
// read the autoscaler's own minimum as $min. Like replicasProgram, it fails the render with a
// scaling error when the autoscaler forbids that by annotation. The autoscaler's bounds,
// minReplicas 1 when unset as in Kubernetes, are the baseline; asking for the bounds it already
// holds is not scaling.
const hpaProgram = `if .spec.scaleTargetRef.kind == %[1]s and .spec.scaleTargetRef.name == %[2]s
then ((.metadata.annotations // {}) as $annotations | (.spec.minReplicas // 1) as $min |
(%[3]s) as $lower | (%[4]s) as $upper |
"\(.kind) '\(.metadata.name)' does not allow the replicas of \(.spec.scaleTargetRef.kind) '\(.spec.scaleTargetRef.name)'" as $forbidden |
if $annotations[%[6]q] == "true" and ($lower != $min or $upper != .spec.maxReplicas)
then error({%[8]q: "\($forbidden) to change"})
elif $annotations[%[7]q] == "true" and $lower < $min
then error({%[8]q: "\($forbidden) to go below \($min)"})
else %[5]s end)
else . end`

// hpaBounds is what a HorizontalPodAutoscaler is to hold, as jq for hpaProgram: the lower and the
// upper bound of its workload's replicas and the steps that set them.
type hpaBounds struct {
	lower string
	upper string
	set   string
}

// vpaProgram runs the steps %[3]s on every VerticalPodAutoscaler that targets the workload of
// kind %[1]s and name %[2]s.
const vpaProgram = `if .spec.targetRef.kind == %[1]s and .spec.targetRef.name == %[2]s then (%[3]s) else . end`

// vpaPolicyProgram runs the steps %[2]s on the policy of container %[1]s in a VerticalPodAutoscaler.
// A container without a policy of its own follows the "*" one, if there is one, so it gets a copy
// of that to change, which leaves the other containers following "*" as they were.
const vpaPolicyProgram = `.spec.resourcePolicy.containerPolicies |= ((. // []) as $policies |
if any($policies[]; .containerName == %[1]s) then $policies
else $policies + [(($policies | map(select(.containerName == "*")) | first) // {}) + {containerName: %[1]s}] end |
map(if .containerName == %[1]s then (%[2]s) else . end))`

// vpaScales holds the only resources VPA controls, the others stay in the workload alone, each
// with the finest scale VPA accepts in a bound: a whole number of milli CPUs or of bytes.
var vpaScales = map[string]resource.Scale{"cpu": resource.Milli, "memory": 0}

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

// Patches translates the requests into nelm render patches: for each request, one
// for its workload when the request changes it, and one for each kind of
// autoscaler the request pins or bounds.
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

		// A request that only gives ranges leaves the workload as the chart renders it.
		if program != "" {
			patches = append(patches, workloadPatch(request, program))
		}

		autoscalers, err := autoscalerPatches(request)
		if err != nil {
			return nil, fmt.Errorf("patch autoscalers of %s '%s': %w", request.Kind, request.Name, err)
		}

		patches = append(patches, autoscalers...)
	}

	return patches, nil
}

// workloadPatch runs program on the workload the request addresses.
func workloadPatch(request Request, program string) spec.Patch {
	return spec.Patch{
		Match: spec.ResourceMatcher{Kinds: []string{request.Kind}, Names: []string{request.Name}},
		Type:  spec.PatchTypeJQ,
		Patch: program,
	}
}

// autoscalerPatches builds the patches for the autoscalers targeting the request's
// workload: a HorizontalPodAutoscaler is pinned to the replicas or bounded by the
// replicas range, a VerticalPodAutoscaler is pinned to the cpu and memory requests
// or bounded by the range of each container. A HorizontalPodAutoscaler guards the
// replicas with the same annotations as a workload.
//
// The workload keeps pinned values too, so they hold when the chart renders no
// autoscaler for it; a range reaches the autoscalers alone. A matcher cannot
// address an object by what it targets, so these patches match every autoscaler
// of a kind and the programs select their targets themselves.
func autoscalerPatches(request Request) ([]spec.Patch, error) {
	kind, err := json.Marshal(request.Kind)
	if err != nil {
		return nil, fmt.Errorf("encode kind: %w", err)
	}

	name, err := json.Marshal(request.Name)
	if err != nil {
		return nil, fmt.Errorf("encode name: %w", err)
	}

	var patches []spec.Patch

	bounds, ok, err := replicaBounds(request)
	if err != nil {
		return nil, err
	}

	if ok {
		patches = append(patches, spec.Patch{
			Match: spec.ResourceMatcher{Groups: []string{"autoscaling"}, Kinds: []string{"HorizontalPodAutoscaler"}},
			Type:  spec.PatchTypeJQ,
			Patch: fmt.Sprintf(hpaProgram, kind, name, bounds.lower, bounds.upper, bounds.set, annotationScaleDisabled, annotationScaleDownDisabled, scalingErrorKey),
		})
	}

	steps, err := vpaSteps(request.Containers)
	if err != nil {
		return nil, err
	}

	if len(steps) > 0 {
		patches = append(patches, spec.Patch{
			Match: spec.ResourceMatcher{Groups: []string{"autoscaling.k8s.io"}, Kinds: []string{"VerticalPodAutoscaler"}},
			Type:  spec.PatchTypeJQ,
			Patch: fmt.Sprintf(vpaProgram, kind, name, strings.Join(steps, " | ")),
		})
	}

	return patches, nil
}

// replicaBounds returns what a HorizontalPodAutoscaler of the request's workload is
// to hold, false when the request leaves the replicas alone. Replicas pin both
// bounds; a range sets the bounds it gives and keeps the autoscaler's own for the
// rest.
//
// The CR schema rejects a range on a kind without replicas and a range next to
// replicas; the guards keep the patch honest if a request reaches it from
// somewhere else.
func replicaBounds(request Request) (hpaBounds, bool, error) {
	ranged := request.MinReplicas != nil || request.MaxReplicas != nil

	if ranged && request.Replicas != nil {
		return hpaBounds{}, false, errors.New("replicas are set together with a replicas range")
	}

	if request.Replicas != nil {
		replicas := strconv.Itoa(int(*request.Replicas))

		// A HorizontalPodAutoscaler cannot go below one replica, so zero goes to the
		// workload alone: the autoscaler does not scale a target that runs none. Its
		// guards still apply, as zero replicas change the workload all the same.
		set := "."
		if *request.Replicas > 0 {
			set = fmt.Sprintf("(.spec.minReplicas = %[1]s | .spec.maxReplicas = %[1]s)", replicas)
		}

		return hpaBounds{lower: replicas, upper: replicas, set: set}, true, nil
	}

	if !ranged {
		return hpaBounds{}, false, nil
	}

	if _, ok := replicaKinds[request.Kind]; !ok {
		return hpaBounds{}, false, errors.New("kind does not have replicas")
	}

	bounds := hpaBounds{lower: "$min", upper: ".spec.maxReplicas"}

	var steps []string

	if request.MinReplicas != nil {
		bounds.lower = strconv.Itoa(int(*request.MinReplicas))
		steps = append(steps, ".spec.minReplicas = "+bounds.lower)
	}

	if request.MaxReplicas != nil {
		bounds.upper = strconv.Itoa(int(*request.MaxReplicas))
		steps = append(steps, ".spec.maxReplicas = "+bounds.upper)
	}

	bounds.set = "(" + strings.Join(steps, " | ") + ")"

	return bounds, true, nil
}

// vpaSteps builds the jq steps that pin or bound the containers' resources in a
// VerticalPodAutoscaler, one per container that sets a resource VPA controls.
//
// A cpu or memory request becomes both minAllowed and maxAllowed, so VPA keeps it
// as it is; a range becomes the bounds it gives, within which VPA keeps picking
// the request. VPA scales a limit together with its request, keeping the ratio
// from the pod template, so a container that sets a cpu or memory limit makes VPA
// leave its limits alone. A container the workload does not have gets a policy
// VPA has no use for.
func vpaSteps(containers []Container) ([]string, error) {
	var steps []string

	for _, container := range containers {
		// The CR schema rejects cpu and memory requests and limits next to a
		// range, as VPA picks the requests within it; the guard keeps a request
		// that reached here another way from both pinning and ranging them.
		if (controlsAny(container.Min) || controlsAny(container.Max)) && (controlsAny(container.Requests) || controlsAny(container.Limits)) {
			return nil, fmt.Errorf("container '%s' sets cpu or memory requests or limits together with a range", container.Name)
		}

		minAllowed := make(map[string]string)
		maxAllowed := make(map[string]string)
		limited := false

		lists := []struct {
			name       string
			quantities map[string]string
			bounds     []map[string]string
		}{
			{"request", container.Requests, []map[string]string{minAllowed, maxAllowed}},
			{"min", container.Min, []map[string]string{minAllowed}},
			{"max", container.Max, []map[string]string{maxAllowed}},
		}

		for resourceName, scale := range vpaScales {
			for _, list := range lists {
				quantity, ok := list.quantities[resourceName]
				if !ok {
					continue
				}

				bound, err := vpaBound(quantity, scale)
				if err != nil {
					return nil, fmt.Errorf("container '%s' %s %s: %w", container.Name, resourceName, list.name, err)
				}

				for _, bounds := range list.bounds {
					bounds[resourceName] = bound
				}
			}

			if _, ok := container.Limits[resourceName]; ok {
				limited = true
			}
		}

		var changes []string

		for _, policy := range []struct {
			field  string
			bounds map[string]string
		}{
			{"minAllowed", minAllowed},
			{"maxAllowed", maxAllowed},
		} {
			if len(policy.bounds) == 0 {
				continue
			}

			literal, err := json.Marshal(policy.bounds)
			if err != nil {
				return nil, fmt.Errorf("encode container '%s' %s: %w", container.Name, policy.field, err)
			}

			changes = append(changes, fmt.Sprintf(".%s += %s", policy.field, literal))
		}

		if limited {
			changes = append(changes, `.controlledValues = "RequestsOnly"`)
		}

		if len(changes) == 0 {
			continue
		}

		name, err := json.Marshal(container.Name)
		if err != nil {
			return nil, fmt.Errorf("encode container name: %w", err)
		}

		steps = append(steps, fmt.Sprintf(vpaPolicyProgram, name, strings.Join(changes, " | ")))
	}

	return steps, nil
}

// vpaBound turns a requested quantity into a VPA bound. VPA rejects a bound finer
// than the scale of its resource, while a pod takes it, so the bound is rounded up
// to that scale: the container still gets at least what it asks for.
func vpaBound(value string, scale resource.Scale) (string, error) {
	quantity, err := resource.ParseQuantity(value)
	if err != nil {
		return "", err
	}

	quantity.RoundUp(scale)

	return quantity.String(), nil
}

// controlsAny reports whether the quantities give a resource VPA controls.
func controlsAny(quantities map[string]string) bool {
	for resourceName := range vpaScales {
		if _, ok := quantities[resourceName]; ok {
			return true
		}
	}

	return false
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
