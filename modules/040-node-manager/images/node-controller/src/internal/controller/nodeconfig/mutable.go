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

package nodeconfig

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// IsMutable reports whether bashible manages the group. A group that names no
// type predates the field and is bashible's as well.
func IsMutable(ng *v1.NodeGroup) bool {
	return ng.Spec.SystemType != v1.SystemTypeImmutable
}

// readMutableInputs reads what the document of a node bashible configures is
// rendered from. No Engine digest is read: a cluster without Engine groups need
// not ship one.
func (s *sourceReader) readMutableInputs(ctx context.Context) (clusterInputs, error) {
	var in clusterInputs
	images, err := s.readImagesDigests(ctx)
	if err != nil {
		return in, fmt.Errorf("read the release images: %w", err)
	}

	in.Platform, err = s.readPlatformDocument(ctx, images)
	if err != nil {
		return in, fmt.Errorf("read the platform document: %w", err)
	}

	endpoints, err := s.readAPIServerEndpoints(ctx)
	if err != nil {
		return in, fmt.Errorf("discover the API server endpoints: %w", err)
	}
	if len(endpoints) == 0 {
		return in, errors.New("no API server endpoints discovered")
	}
	in.APIServerEndpoints = endpoints

	in.RegistryPackagesProxyToken, err = s.readPackagesProxyToken(ctx)
	if err != nil {
		return in, fmt.Errorf("read the packages proxy token: %w", err)
	}

	nsprs, err := s.readNodeStaticPodRequests(ctx)
	if err != nil {
		return in, fmt.Errorf("read the static pod requests: %w", err)
	}
	in.NodeStaticPodRequests = orderedNSPRs(nsprs)
	_, in.NodeStaticPodRequestsRejected = rejectedNSPRs(in.NodeStaticPodRequests)

	ners, err := s.readNodeExtensionRequests(ctx)
	if err != nil {
		return in, fmt.Errorf("read the extension requests: %w", err)
	}
	in.NodeExtensions, in.NodeExtensionConflicts = settleNERs(ners, in.Platform.Extensions)
	return in, nil
}

// renderMutableSpec is the document of a node bashible configures: the platform's
// part comes from its document, and the requests add the static pods and the registry
// packages they ask this node for.
func renderMutableSpec(node *corev1.Node, ngName string, in clusterInputs) internalv1alpha1.NodeSpec {
	requested, _ := nodeExtensions(in, deckhousev1alpha1.ExtensionKindPackage, node, ngName)
	return internalv1alpha1.NodeSpec{
		SystemType:            internalv1alpha1.SystemTypeMutable,
		NodeName:              node.Name,
		APIServerEndpoints:    in.APIServerEndpoints,
		RegistryPackagesProxy: internalv1alpha1.RegistryPackagesProxy{AccessToken: in.RegistryPackagesProxyToken},
		FeatureGates:          maps.Clone(in.Platform.FeatureGates),
		StaticPods:            mergeStaticPods(in.Platform.StaticPods, nodeStaticPods(in.NodeStaticPodRequests, in.NodeStaticPodRequestsRejected, ngName)),
		Extensions:            mergeExtensions(in.Platform.Extensions, requested),
	}
}

// mergeStaticPods puts the platform's pods first and adds the requested ones; a platform
// pod wins over a request's pod of the same name.
func mergeStaticPods(platform, requested []internalv1alpha1.StaticPod) []internalv1alpha1.StaticPod {
	pods := slices.Clone(platform)
	for _, pod := range requested {
		taken := slices.ContainsFunc(platform, func(p internalv1alpha1.StaticPod) bool { return p.Name == pod.Name })
		if !taken {
			pods = append(pods, pod)
		}
	}
	return pods
}

// RenderMutableSpec renders the template of a group bashible manages: its nodes' document
// for a node not yet registered, labelled the way it will be.
func RenderMutableSpec(ctx context.Context, reader client.Reader, ng *v1.NodeGroup) (internalv1alpha1.NodeSpec, error) {
	in, err := (&sourceReader{Reader: reader}).readMutableInputs(ctx)
	if err != nil {
		return internalv1alpha1.NodeSpec{}, fmt.Errorf("read the inputs of a bashible node: %w", err)
	}
	return renderMutableSpec(registeringNode(ng, ""), ng.Name, in), nil
}
