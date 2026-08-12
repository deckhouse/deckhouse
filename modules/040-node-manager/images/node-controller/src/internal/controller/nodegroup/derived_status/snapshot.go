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

	// DefaultZones, InstanceClass, KnownClassNames and Capacity are only read for a
	// CloudEphemeral NodeGroup whose provider named an InstanceClass kind.
	DefaultZones    []string
	InstanceClass   map[string]any
	KnownClassNames []string
	Capacity        *capacity.InstanceType

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
		snap.StaticConfig = s.readStatic(ctx)
	}

	if ng.Spec.NodeType != v1.NodeTypeCloudEphemeral {
		return snap, nil
	}

	snap.DefaultZones = s.readDefaultZones(ctx, provider)

	if provider.InstanceClassKind == "" {
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

	if ng.Spec.CloudInstances == nil {
		return snap, nil
	}
	kind := ng.Spec.CloudInstances.ClassReference.Kind
	name := ng.Spec.CloudInstances.ClassReference.Name
	if kind == "" || name == "" {
		return snap, nil
	}

	spec, err := s.readInstanceClassSpec(ctx, provider.InstanceClassAPIVersion, kind, name)
	if err != nil {
		// A deleted InstanceClass is not a failure: Validate already reports it as a NodeGroup
		// validation error, and instanceClass stays unset rather than guessed.
		if !apierrors.IsNotFound(err) {
			return Snapshot{}, fmt.Errorf("read instance class of %s: %w", ng.Name, err)
		}
		logger.V(1).Info("instance class not found, skipping capacity/instanceClass", "nodeGroup", ng.Name, "kind", kind, "name", name)
		return snap, nil
	}
	if spec == nil {
		return snap, nil
	}
	snap.InstanceClass, _ = spec.(map[string]any)

	// nodeCapacity is only needed for scale-from-zero (min==0 && max>0), which is also the only
	// case check #3 asks about — so the calculation happens once and both halves read the result.
	if ng.Spec.CloudInstances.MinPerZone == 0 && ng.Spec.CloudInstances.MaxPerZone > 0 {
		catalog := s.readInstanceTypesCatalog(ctx)
		snap.Capacity, snap.CapacityErr = capacity.CalculateNodeTemplateCapacity(kind, spec, catalog)
	}

	return snap, nil
}
