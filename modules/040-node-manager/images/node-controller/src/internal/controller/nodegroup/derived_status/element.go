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
		if in.MinPerZone == 0 && in.MaxPerZone > 0 &&
			in.ClassRefKind == kindInUse && containsString(in.KnownClassNames, in.ClassRefName) {
			in.CapacityErr = s.capacityError(ctx, in.ClassRefKind, in.ClassRefName)
		}
	}

	return RunCloudChecks(in)
}

func (s *Service) capacityError(ctx context.Context, kind, name string) error {
	spec, err := s.readInstanceClassSpec(ctx, kind, name)
	if err != nil || spec == nil {
		return err
	}
	catalog := s.readInstanceTypesCatalog(ctx)
	_, err = calculateNodeCapacity(kind, spec, catalog)
	return err
}

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

func (s *Service) readStatic(ctx context.Context) map[string]interface{} {
	secret := &corev1.Secret{}
	if err := s.reader().Get(ctx, types.NamespacedName{Namespace: staticConfigSecretNamespace, Name: staticConfigSecretName}, secret); err != nil {
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
