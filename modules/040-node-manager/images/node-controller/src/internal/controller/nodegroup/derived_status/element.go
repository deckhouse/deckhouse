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
	"encoding/base64"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	sigsyaml "sigs.k8s.io/yaml"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

const (
	manualRolloutIDAnnotation = "manual-rollout-id"

	staticConfigSecretName      = "d8-static-cluster-configuration"
	staticConfigSecretNamespace = "kube-system"
	staticConfigKey             = "static-cluster-configuration.yaml"
)

// BuildElement assembles the internal.nodeGroups blob element for a single
// NodeGroup, mirroring one iteration of the get_crds hook loop
// (get_crds.go:332-554): it runs Compute for the derived fields, RunCloudChecks
// for the CloudEphemeral gate, and BuildNodeGroupBlob to fold the raw spec with
// the computed overlay. rawSpec is the NodeGroup's .spec as stored by the
// apiserver (CRD-shaped, unknown fields pruned).
//
// The returned error string is the validation error get_crds would write to the
// NodeGroup status (empty when valid). On a non-empty error get_crds reuses the
// previously-stored blob element to avoid disruption; that preserve-prior
// behaviour needs the prior blob and is therefore applied by the caller.
func (s *Service) BuildElement(ctx context.Context, ng *v1.NodeGroup, rawSpec map[string]interface{}) (map[string]interface{}, string, error) {
	result, err := s.Compute(ctx, ng)
	if err != nil {
		return nil, "", err
	}

	cloudProvider := s.readCloudProviderData(ctx)
	check := s.runCloudChecks(ctx, ng, cloudProvider)

	in := BlobInput{
		Name:            ng.Name,
		ManualRolloutID: ng.GetAnnotations()[manualRolloutIDAnnotation],
		NodeType:        ng.Spec.NodeType,
		RawSpec:         rawSpec,
		CloudProcessed:  check.Processed,
	}
	if ng.Spec.NodeType == v1.NodeTypeStatic {
		in.Static = s.readStatic(ctx)
	}

	return BuildNodeGroupBlob(in, result), check.Error, nil
}

// runCloudChecks gathers the inputs for RunCloudChecks from live kube objects.
// The expensive reads (instance-class listing, capacity) run only when the
// CloudEphemeral cloud branch would run (kindInUse != ""), matching get_crds.
func (s *Service) runCloudChecks(ctx context.Context, ng *v1.NodeGroup, cloudProvider map[string]interface{}) CloudCheckResult {
	kindInUse, _ := cloudProvider["instanceClassKind"].(string)

	in := CloudCheckInput{
		NodeType:  ng.Spec.NodeType,
		KindInUse: kindInUse,
	}
	if ng.Spec.CloudInstances != nil {
		in.ClassRefKind = ng.Spec.CloudInstances.ClassReference.Kind
		in.ClassRefName = ng.Spec.CloudInstances.ClassReference.Name
		in.MinPerZone = ng.Spec.CloudInstances.MinPerZone
		in.MaxPerZone = ng.Spec.CloudInstances.MaxPerZone
		in.SpecZones = ng.Spec.CloudInstances.Zones
	}

	if in.NodeType == v1.NodeTypeCloudEphemeral && kindInUse != "" {
		in.KnownClassNames = s.readInstanceClassNames(ctx, kindInUse)
		in.DefaultZones = s.readDefaultZones(ctx, cloudProvider)
		// Capacity is only consulted for scale-from-zero of a class that would
		// pass checks #1/#2 (get_crds computes it after those pass).
		if in.MinPerZone == 0 && in.MaxPerZone > 0 &&
			in.ClassRefKind == kindInUse && containsString(in.KnownClassNames, in.ClassRefName) {
			in.CapacityErr = s.capacityError(ctx, in.ClassRefKind, in.ClassRefName)
		}
	}

	return RunCloudChecks(in)
}

// capacityError reproduces the scale-from-zero capacity calculation error that
// get_crds check #3 consults (nil when capacity resolves).
func (s *Service) capacityError(ctx context.Context, kind, name string) error {
	spec, err := s.readInstanceClassSpec(ctx, kind, name)
	if err != nil || spec == nil {
		return err
	}
	catalog := s.readInstanceTypesCatalog(ctx)
	_, err = calculateNodeCapacity(kind, spec, catalog)
	return err
}

// readInstanceClassNames lists the InstanceClass objects of the given kind,
// returning their names for check #2's known-class-name set (the get_crds "ics"
// snapshot equivalent).
func (s *Service) readInstanceClassNames(ctx context.Context, kind string) []string {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: instanceClassGroup, Version: instanceClassVersion, Kind: kind + "List"})
	if err := s.Client.List(ctx, list); err != nil {
		return nil
	}
	names := make([]string, 0, len(list.Items))
	for i := range list.Items {
		names = append(names, list.Items[i].GetName())
	}
	return names
}

// readStatic reproduces internal.static (convert_static_cluster_configuration):
// the internalNetworkCIDRs field of the d8-static-cluster-configuration Secret,
// wrapped as {"internalNetworkCIDRs": [...]}. nil when the Secret is absent so
// the blob omits the "static" field (matching get_crds.go:345-351). When present
// the field is emitted even with an empty CIDR list, since internal.static then
// holds a single key.
func (s *Service) readStatic(ctx context.Context) map[string]interface{} {
	secret := &corev1.Secret{}
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: staticConfigSecretNamespace, Name: staticConfigSecretName}, secret); err != nil {
		return nil
	}
	raw, ok := secret.Data[staticConfigKey]
	if !ok {
		return nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(string(raw)); err == nil {
		raw = decoded
	}

	var cfg struct {
		InternalNetworkCIDRs []interface{} `json:"internalNetworkCIDRs"`
	}
	if err := sigsyaml.Unmarshal(raw, &cfg); err != nil {
		return nil
	}
	cidrs := cfg.InternalNetworkCIDRs
	if cidrs == nil {
		cidrs = []interface{}{}
	}
	return map[string]interface{}{"internalNetworkCIDRs": cidrs}
}
