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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

func (s *Service) ResolveNodeGroup(ctx context.Context, ng *v1.NodeGroup, rawSpec map[string]interface{}) (ResolvedNodeGroup, string, error) {
	snap, err := s.BuildSnapshot(ctx, ng)
	if err != nil {
		return ResolvedNodeGroup{}, "", err
	}
	result, err := Derive(ctx, ng, snap)
	if err != nil {
		return ResolvedNodeGroup{}, "", err
	}
	check := Validate(ng, snap)

	in := ResolveInput{
		Name:            ng.Name,
		ManualRolloutID: ng.GetAnnotations()[manualRolloutIDAnnotation],
		NodeType:        ng.Spec.NodeType,
		RawSpec:         rawSpec,
		Static:          snap.StaticConfig,
		CloudProcessed:  check.Processed,
	}

	return ResolveNodeGroup(in, result), check.Error, nil
}

func (s *Service) readInstanceClassNames(ctx context.Context, version, kind string) ([]string, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: instanceClassGroup, Version: version, Kind: kind + "List"})
	if err := s.Client.List(ctx, list); err != nil {
		return nil, fmt.Errorf("list %s at %s: %w", kind, version, err)
	}
	names := make([]string, 0, len(list.Items))
	for i := range list.Items {
		names = append(names, list.Items[i].GetName())
	}
	return names, nil
}

// readStatic returns the static cluster configuration. An absent Secret is a cluster that has none
// and yields nothing; an unreadable one is returned, because a silent nil drops the static block
// from the published element and shifts the configuration checksum on every static node.
func (s *Service) readStatic(ctx context.Context) (map[string]interface{}, error) {
	secret := &corev1.Secret{}
	err := s.Client.Get(ctx, types.NamespacedName{Namespace: staticConfigSecretNamespace, Name: staticConfigSecretName}, secret)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read static cluster configuration secret: %w", err)
	}
	raw, ok := secret.Data[staticConfigKey]
	if !ok {
		return nil, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(string(raw)); err == nil {
		raw = decoded
	}

	var cfg struct {
		InternalNetworkCIDRs []interface{} `json:"internalNetworkCIDRs"`
	}
	if err := sigsyaml.Unmarshal(raw, &cfg); err != nil {
		return nil, nil
	}
	cidrs := cfg.InternalNetworkCIDRs
	if cidrs == nil {
		cidrs = []interface{}{}
	}
	return map[string]interface{}{"internalNetworkCIDRs": cidrs}, nil
}
