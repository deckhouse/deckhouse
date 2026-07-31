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

package capi

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/machinetemplate"
)

const (
	// machineTemplateContractKey is the single file of the v2 provider contract. Its presence in
	// the provider Secret is what selects the v2 engine — see readMachineTemplateContract.
	machineTemplateContractKey = "template.yaml"

	// generationSuffix makes the counter readable in the object name. The number that decides the
	// next generation is read from the snapshot annotation, not from the name.
	generationSuffix = "-gen"

	// maxGenerationAttempts bounds the search for a free generation number when the computed name
	// is already taken by an object we cannot reuse (a leftover of a generation whose
	// MachineDeployment was deleted). Reaching the bound means something else is writing these
	// objects, which is worth an error rather than an infinite loop.
	maxGenerationAttempts = 8

	// keptGenerations is how many superseded generations survive the prune, per zone.
	//
	// The snapshot on a generation object is the only durable record of what a rollout changed:
	// the NodeGroup event carrying the same diff is dropped by Kubernetes after about an hour,
	// while the object used to disappear as soon as CAPI retired its MachineSet. An operator
	// asking "why did my machines roll last night" needs something to read. These objects have no
	// controller and cost a few kilobytes each.
	keptGenerations = 3
)

// recentGenerations returns the generations to keep for their snapshot alone: the newest
// keptGenerations of each zone. Names from the v1 era carry no generation and are not ranked —
// they are pruned as before, once nothing references them.
func recentGenerations(names []string) map[string]struct{} {
	byZone := map[string][]string{}
	for _, name := range names {
		if generationOf(name) == 0 {
			continue
		}
		zone := name[:strings.LastIndex(name, generationSuffix)]
		byZone[zone] = append(byZone[zone], name)
	}

	keep := map[string]struct{}{}
	for _, generations := range byZone {
		slices.SortFunc(generations, func(a, b string) int { return generationOf(b) - generationOf(a) })
		for _, name := range generations[:min(len(generations), keptGenerations)] {
			keep[name] = struct{}{}
		}
	}
	return keep
}

// readMachineTemplateContract returns the parsed v2 contract, or nil when the provider still ships
// the v1 trio (machine-template.yaml + instance-class.checksum + machine-deployment-spec-patch).
//
// The switch is the presence of the file, not a flag anywhere in Deckhouse: a provider module owns
// its own release cycle, so it must be able to move to v2 (or be rolled back) on its own, and
// node-controller must serve both shapes meanwhile.
func (r *MachineDeploymentReconciler) readMachineTemplateContract(ctx context.Context, cloudType string) (*machinetemplate.Contract, error) {
	data, found, err := r.readProviderTemplateIfPresent(ctx, cloudType, engineCAPITemplates, machineTemplateContractKey)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	contract, err := machinetemplate.ParseContract(data)
	if err != nil {
		return nil, fmt.Errorf("cloud provider %s: %w", cloudType, err)
	}
	return contract, nil
}

// machineTemplateGeneration is everything the generation decision needs for one zone.
type machineTemplateGeneration struct {
	ng       *deckhousev1.NodeGroup
	contract *machinetemplate.Contract
	render   machinetemplate.RenderContext
	// rolloutID is the NodeGroup's manual-rollout-id: the operator's explicit "roll now, with
	// today's provider config and today's template text".
	rolloutID string
	// currentName is the template the live MachineDeployment references, empty for a NodeGroup or
	// zone that has none yet.
	currentName string
	// baseName is "<nodeGroup>-<hash(clusterUUID+zone)>" — the same zone suffix the
	// MachineDeployment carries, so an operator can line the two up by eye.
	baseName string
	gvk      schema.GroupVersionKind
}

// ensureMachineTemplateGeneration returns the name of the infrastructure MachineTemplate the
// MachineDeployment must point at, creating a new generation only when the InstanceClass values
// the provider declared as rolling (or the manual rollout id) actually changed.
//
// The existing generation is never re-rendered and never patched in its spec. That is the point of
// the design: a CAPI infrastructure template is read afresh every time a Machine is created, so
// editing one in place does not roll anything — it silently changes what the *old* MachineSet
// builds its replacement machines from, mixing two configurations inside one generation. v1 did
// exactly this whenever the provider config or the template text changed without the checksum
// changing (an upstream immutability webhook turned it into a reconcile error, and a provider
// without one, like DVP, took the mutation silently).
func (r *MachineDeploymentReconciler) ensureMachineTemplateGeneration(ctx context.Context, in machineTemplateGeneration) (string, error) {
	logger := log.FromContext(ctx)

	nextGeneration := 1
	var reason string

	if in.currentName != "" {
		current, err := r.getMachineTemplate(ctx, in.gvk, in.currentName)
		if err != nil {
			return "", err
		}

		if current == nil {
			// The MachineDeployment references a template that is gone (deleted by hand, or a
			// partial restore). Recreating it under a NEW name would switch the reference and roll
			// every machine of the group; recreating it under the SAME name restores what the
			// MachineSet expects and rolls nothing.
			//
			// The restored object is rendered from today's template text and provider config, so
			// it can differ from the one that was deleted — the original content is not recoverable
			// once the object is gone. That is the lesser evil: without the object the MachineSet
			// cannot create replacement machines at all.
			obj, err := r.buildMachineTemplate(in, in.currentName, generationOf(in.currentName))
			if err != nil {
				return "", err
			}
			if err := r.Client.Create(ctx, obj); err != nil && !errors.IsAlreadyExists(err) {
				return "", err
			}
			logger.Info("recreated missing CAPI MachineTemplate referenced by MachineDeployment",
				"name", in.currentName, "ng", in.ng.Name)
			return in.currentName, nil
		}

		snapshot, hasSnapshot := machinetemplate.DecodeSnapshot(current.GetAnnotations())
		if !hasSnapshot {
			// First v2 reconcile of a cluster whose templates were named by the v1 checksum. The
			// object is adopted as the current generation by writing the snapshot into its
			// metadata (immutability webhooks guard spec, not metadata) — no new object, no name
			// change, no rollout. Generation numbering starts at the next real change.
			if err := r.adoptMachineTemplate(ctx, current, in); err != nil {
				return "", err
			}
			logger.Info("adopted existing CAPI MachineTemplate as the current generation",
				"name", in.currentName, "ng", in.ng.Name)
			return in.currentName, nil
		}

		changes, err := machinetemplate.Changes(snapshot.InstanceClass, in.render.InstanceClass, in.contract.RolloutFields)
		if err != nil {
			return "", fmt.Errorf("compare InstanceClass for NodeGroup %s: %w", in.ng.Name, err)
		}
		if len(changes) == 0 && snapshot.RolloutID == in.rolloutID {
			return in.currentName, nil
		}

		reason = rolloutReason(changes, snapshot.RolloutID, in.rolloutID)
		nextGeneration = snapshot.Generation + 1
	}

	name, created, err := r.createNextGeneration(ctx, in, nextGeneration)
	if err != nil {
		return "", err
	}
	if !created {
		// The object was already there with the very snapshot we wanted (a retry after a partial
		// reconcile). Nothing happened, so nothing is reported.
		return name, nil
	}

	if reason != "" && r.Recorder != nil {
		r.Recorder.Eventf(in.ng, corev1.EventTypeNormal, "MachinesRollout",
			"rolling machines in zone %s: %s", in.render.Zone, reason)
	}
	logger.Info("created new CAPI MachineTemplate generation",
		"name", name, "ng", in.ng.Name, "zone", in.render.Zone, "reason", reason)

	return name, nil
}

func rolloutReason(changes []machinetemplate.Change, storedRolloutID, rolloutID string) string {
	parts := make([]string, 0, 2)
	if len(changes) > 0 {
		parts = append(parts, "instanceClass "+machinetemplate.FormatChanges(changes))
	}
	if storedRolloutID != rolloutID {
		parts = append(parts, fmt.Sprintf("manualRolloutID %q → %q", storedRolloutID, rolloutID))
	}
	return strings.Join(parts, "; ")
}

// createNextGeneration creates the first generation object whose name is free, or reuses one that
// already holds exactly the snapshot we are about to write (a retry after a partial reconcile). It
// reports whether an object was actually created.
func (r *MachineDeploymentReconciler) createNextGeneration(ctx context.Context, in machineTemplateGeneration, generation int) (string, bool, error) {
	for attempt := range maxGenerationAttempts {
		number := generation + attempt
		name := fmt.Sprintf("%s%s%d", in.baseName, generationSuffix, number)

		obj, err := r.buildMachineTemplate(in, name, number)
		if err != nil {
			return "", false, err
		}
		err = r.Client.Create(ctx, obj)
		if err == nil {
			return name, true, nil
		}
		if !errors.IsAlreadyExists(err) {
			return "", false, err
		}

		reusable, err := r.holdsDesiredSnapshot(ctx, in, name)
		if err != nil {
			return "", false, err
		}
		if reusable {
			return name, false, nil
		}
	}
	return "", false, fmt.Errorf("no free MachineTemplate generation for NodeGroup %s in zone %s after %d attempts",
		in.ng.Name, in.render.Zone, maxGenerationAttempts)
}

// holdsDesiredSnapshot reports whether the named object already carries the snapshot the caller is
// about to write — i.e. whether it is the generation we wanted rather than a leftover under the
// same name.
func (r *MachineDeploymentReconciler) holdsDesiredSnapshot(ctx context.Context, in machineTemplateGeneration, name string) (bool, error) {
	existing, err := r.getMachineTemplate(ctx, in.gvk, name)
	if err != nil || existing == nil {
		return false, err
	}
	snapshot, hasSnapshot := machinetemplate.DecodeSnapshot(existing.GetAnnotations())
	if !hasSnapshot {
		return false, nil
	}
	changes, err := machinetemplate.Changes(snapshot.InstanceClass, in.render.InstanceClass, in.contract.RolloutFields)
	if err != nil {
		return false, err
	}
	return len(changes) == 0 && snapshot.RolloutID == in.rolloutID, nil
}

// buildMachineTemplate renders the provider template and stamps everything node-controller owns:
// the name, the namespace, the labels every prune and cleanup selects on, and the snapshot.
//
// The object is created, never applied: it is immutable for the life of its generation, and an
// apply would let a later reconcile write into it.
func (r *MachineDeploymentReconciler) buildMachineTemplate(in machineTemplateGeneration, name string, generation int) (*unstructured.Unstructured, error) {
	rendered, err := machinetemplate.Render(in.contract, in.render)
	if err != nil {
		return nil, fmt.Errorf("render machine template for NodeGroup %s zone %s: %w", in.ng.Name, in.render.Zone, err)
	}

	obj := &unstructured.Unstructured{Object: rendered}
	if obj.GroupVersionKind() != in.gvk {
		return nil, fmt.Errorf("machine template of NodeGroup %s renders %s, but the cloud-provider secret declares %s",
			in.ng.Name, obj.GroupVersionKind(), in.gvk)
	}

	annotations, err := machinetemplate.EncodeSnapshot(machinetemplate.Snapshot{
		InstanceClass: in.render.InstanceClass,
		RolloutID:     in.rolloutID,
		Generation:    generation,
	})
	if err != nil {
		return nil, err
	}

	obj.SetName(name)
	obj.SetNamespace(common.MachineNamespace)
	obj.SetLabels(map[string]string{
		"heritage":   "deckhouse",
		"module":     "node-manager",
		"node-group": in.ng.Name,
	})
	obj.SetAnnotations(annotations)
	return obj, nil
}

func (r *MachineDeploymentReconciler) adoptMachineTemplate(ctx context.Context, current *unstructured.Unstructured, in machineTemplateGeneration) error {
	snapshot, err := machinetemplate.EncodeSnapshot(machinetemplate.Snapshot{
		InstanceClass: in.render.InstanceClass,
		RolloutID:     in.rolloutID,
		Generation:    generationOf(current.GetName()),
	})
	if err != nil {
		return err
	}

	patched := current.DeepCopy()
	annotations := patched.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	maps.Copy(annotations, snapshot)
	patched.SetAnnotations(annotations)

	if err := r.Client.Patch(ctx, patched, client.MergeFrom(current)); err != nil {
		return fmt.Errorf("adopt CAPI MachineTemplate %s: %w", current.GetName(), err)
	}
	return nil
}

// generationOf recovers the counter from a name for the two objects whose snapshot cannot supply
// it: a v1 checksum-named object being adopted (no suffix — 0, so its successor is gen1) and a
// generation object being recreated after someone deleted it.
func generationOf(name string) int {
	idx := strings.LastIndex(name, generationSuffix)
	if idx < 0 {
		return 0
	}
	generation, err := strconv.Atoi(name[idx+len(generationSuffix):])
	if err != nil || generation < 0 {
		return 0
	}
	return generation
}

// getMachineTemplate reads one infrastructure MachineTemplate live. Live because the kind comes
// from the provider Secret and is not in cache.Options.ByObject — the same reason pruneStaleCAPI
// and deleteInfraMachineTemplates read live — and because this read decides whether machines roll.
func (r *MachineDeploymentReconciler) getMachineTemplate(ctx context.Context, gvk schema.GroupVersionKind, name string) (*unstructured.Unstructured, error) {
	obj := newUnstructuredForGVK(gvk)
	err := r.APIReader.Get(ctx, types.NamespacedName{Name: name, Namespace: common.MachineNamespace}, obj)
	if errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get CAPI MachineTemplate %s: %w", name, err)
	}
	return obj, nil
}
