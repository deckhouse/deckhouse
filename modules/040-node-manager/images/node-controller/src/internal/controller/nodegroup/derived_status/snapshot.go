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

package derived_status

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/capacity"
)

// Snapshot is everything this package reads. It is built once per pass, so the derive and the
// validate halves cannot disagree about the world, and so no source is read twice — the two halves
// used to read the zones, the InstanceClass and the type catalog once each.
type Snapshot struct {
	Provider      CloudProviderRegistration
	ClusterUUID   string
	TargetVersion *semver.Version
	DefaultCRI    string
	APIServerMin  *semver.Version

	// DefaultZones is read for every CloudEphemeral NodeGroup; InstanceClass, KnownClassNames and
	// the two capacities only once the provider named an InstanceClass kind and the NodeGroup
	// references it.
	DefaultZones    []string
	InstanceClass   map[string]any
	KnownClassNames []string
	Capacity        *capacity.InstanceType

	// TemplateCapacity is the same calculation as Capacity, kept for every NodeGroup that can hold
	// a machine instead of for scale-from-zero only. It feeds the MachineDeployment's
	// capacity.cluster-autoscaler.kubernetes.io/{cpu,memory} annotations and is deliberately never
	// published into the NodeGroup element — see the comment at its assignment below.
	TemplateCapacity *capacity.InstanceType

	// CapacityErr is the outcome of the single capacity calculation: check #3 needs the error and
	// Derive needs the value, and computing it twice is what the old two-pass shape did.
	CapacityErr error

	// StaticConfig is carried by Static NodeGroups only.
	StaticConfig map[string]interface{}
}

// BuildSnapshot is the only place in this package that talks to the API. Everything downstream is
// a pure function over the result.
//
// An absent source yields an empty field; an unreadable one is returned as an error, because an
// empty value here is indistinguishable from "no cloud provider" and would publish a NodeGroup
// without instanceClass — a checksum shift on every node.
func (s *Service) BuildSnapshot(ctx context.Context, ng *v1.NodeGroup) (Snapshot, error) {
	logger := log.FromContext(ctx)

	provider, err := s.readCloudProviderData(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	clusterUUID, err := s.readClusterUUID(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	targetVersion, defaultCRI, err := s.readClusterConfiguration(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	apiServerMin, err := s.readControlPlaneMinVersion(ctx)
	if err != nil {
		return Snapshot{}, err
	}

	snap := Snapshot{
		Provider:      provider,
		ClusterUUID:   clusterUUID,
		TargetVersion: targetVersion,
		DefaultCRI:    defaultCRI,
		APIServerMin:  apiServerMin,
	}

	if ng.Spec.NodeType == v1.NodeTypeStatic {
		snap.StaticConfig, err = s.readStatic(ctx)
		if err != nil {
			return Snapshot{}, err
		}
	}

	if ng.Spec.NodeType != v1.NodeTypeCloudEphemeral {
		return snap, nil
	}

	snap.DefaultZones, err = s.readDefaultZones(ctx, provider)
	if err != nil {
		return Snapshot{}, err
	}

	if provider.InstanceClassKind == "" {
		return snap, nil
	}

	// Everything below describes one specific InstanceClass, so a NodeGroup that names none is
	// done here — before the cluster-wide List of instance classes below.
	classRef := ng.Spec.CloudInstances
	if classRef == nil || classRef.ClassReference.Kind == "" || classRef.ClassReference.Name == "" {
		return snap, nil
	}

	// Without a published version there is no version to read the InstanceClass at, and guessing
	// one is what this whole mechanism exists to prevent. Describing the NodeGroup survives it —
	// rendering does not, and Validate reports it as a validation error.
	if provider.InstanceClassAPIVersion == "" {
		logger.V(1).Info("cloud provider published no instanceClassAPIVersion, skipping capacity/instanceClass", "nodeGroup", ng.Name, "kind", provider.InstanceClassKind)
		return snap, nil
	}

	// A failed List must not reach the checks: an empty name set reads as "instance class not
	// found", which marks the NodeGroup invalid and stops its MachineDeployments from being
	// rendered. Surface the error so the reconcile retries instead.
	names, err := s.readInstanceClassNames(ctx, provider.InstanceClassAPIVersion, provider.InstanceClassKind)
	if err != nil {
		return Snapshot{}, err
	}
	snap.KnownClassNames = names

	kind := classRef.ClassReference.Kind
	name := classRef.ClassReference.Name

	// Checks #1 and #2 reject a NodeGroup whose class reference names the wrong kind or a class
	// that does not exist, and a rejected NodeGroup publishes no cloud overlay at all — so reading
	// the class and computing its capacity below would be work thrown away.
	if kind != provider.InstanceClassKind || !containsString(names, name) {
		return snap, nil
	}

	spec, err := s.readInstanceClassSpec(ctx, provider.InstanceClassAPIVersion, kind, name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return Snapshot{}, fmt.Errorf("read instance class of %s: %w", ng.Name, err)
		}
		// The class was in the List a few lines up and is gone now — deleted mid-pass. Checks #1
		// and #2 both see the stale name and would pass, so without this the NodeGroup is declared
		// processed and publishes instanceClass: null, dropping the real class from the element.
		// Recording it as a capacity failure is how the previous shape refused: check #3 reads it.
		snap.CapacityErr = err
		logger.V(1).Info("instance class disappeared mid-pass, refusing to publish it", "nodeGroup", ng.Name, "kind", kind, "name", name)
		return snap, nil
	}
	if spec == nil {
		return snap, nil
	}
	snap.InstanceClass = spec

	// The autoscaler needs a template NodeInfo for every node group it discovers, not only for the
	// ones that scale from zero: while a group has no registered Node — a single-replica group whose
	// Node is still booting, or any group mid-churn — the clusterapi provider has nothing to build a
	// NodeInfo from, and ResourcesLeft then fails with "No node info for: <group>", which aborts
	// scale-up for every other group in the cluster too. So the capacity is calculated for every
	// NodeGroup that can hold a machine, and still calculated only once.
	if classRef.MaxPerZone > 0 {
		catalog, err := s.readInstanceTypesCatalog(ctx)
		if err != nil {
			return Snapshot{}, err
		}
		templateCapacity, capacityErr := capacity.CalculateNodeTemplateCapacity(kind, spec, catalog)
		snap.TemplateCapacity = templateCapacity

		// Scale-from-zero alone publishes the value into the NodeGroup element and treats a failure
		// as a NodeGroup error (check #3). Publishing it for the rest would add a key to the element
		// bashible-apiserver hashes into every node's configurationChecksum — AddToChecksum excludes
		// only the replica counters under cloudInstances — re-running node configuration across the
		// fleet for a value no node reads.
		if classRef.MinPerZone == 0 {
			snap.Capacity, snap.CapacityErr = templateCapacity, capacityErr
		} else if capacityErr != nil {
			logger.V(1).Info("node capacity unresolved, MachineDeployment will carry no capacity annotations",
				"nodeGroup", ng.Name, "kind", kind, "name", name, "err", capacityErr.Error())
		}
	}

	return snap, nil
}
