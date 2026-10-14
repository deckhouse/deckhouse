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
	"bytes"
	"context"
	"fmt"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// platformDocument is what the platform puts on every node bashible configures: the
// release's candi/nodelet/nodeconfig-mutable.yaml.tpl with its digests filled in.
type platformDocument struct {
	FeatureGates map[string]bool
	Extensions   []internalv1alpha1.Extension
	StaticPods   []internalv1alpha1.StaticPod
}

// platformFile is the part of a NodeConfig the platform document may carry; anything
// else in the file is refused rather than dropped.
type platformFile struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Spec       struct {
		SystemType   internalv1alpha1.SystemType  `json:"systemType"`
		FeatureGates map[string]bool              `json:"featureGates"`
		Extensions   []internalv1alpha1.Extension `json:"extensions"`
		StaticPods   []internalv1alpha1.StaticPod `json:"staticPods"`
	} `json:"spec"`
}

// renderPlatformDocument fills the release's digests into the platform document. Mirrors
// FirstMaster in dhctl/pkg/nodeletseed/seed.go, which seeds the first master from the
// same file: a digest the release lacks stops the render, as a node would wait for it.
func renderPlatformDocument(text string, images map[string]map[string]string) (platformDocument, error) {
	tpl, err := template.New(platformDocumentKey).Option("missingkey=error").Parse(text)
	if err != nil {
		return platformDocument{}, fmt.Errorf("parse %s: %w", platformDocumentKey, err)
	}
	var rendered bytes.Buffer
	if err := tpl.Execute(&rendered, map[string]any{"images": images}); err != nil {
		return platformDocument{}, fmt.Errorf("render %s: %w", platformDocumentKey, err)
	}
	var file platformFile
	if err := yaml.UnmarshalStrict(rendered.Bytes(), &file); err != nil {
		return platformDocument{}, fmt.Errorf("parse rendered %s: %w", platformDocumentKey, err)
	}
	if file.Spec.SystemType != internalv1alpha1.SystemTypeMutable {
		return platformDocument{}, fmt.Errorf("%s describes a %q node, not a Mutable one", platformDocumentKey, file.Spec.SystemType)
	}

	for i, ext := range file.Spec.Extensions {
		if ext.Name == "" || ext.Digest == "" {
			return platformDocument{}, fmt.Errorf("%s: extension %d has no name or no digest", platformDocumentKey, i)
		}
		file.Spec.Extensions[i].RequestedBy = platformExtensionRequestedBy
	}
	return platformDocument{
		FeatureGates: file.Spec.FeatureGates,
		Extensions:   file.Spec.Extensions,
		StaticPods:   file.Spec.StaticPods,
	}, nil
}

// readPlatformDocument reads the platform document of this release.
func (s *sourceReader) readPlatformDocument(ctx context.Context, images map[string]map[string]string) (platformDocument, error) {
	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{Namespace: cloudInstanceManagerNS, Name: platformTemplatesConfigMapName}
	if err := s.Reader.Get(ctx, key, cm); err != nil {
		return platformDocument{}, fmt.Errorf("read the platform node document: %w", err)
	}
	text, ok := cm.Data[platformDocumentKey]
	if !ok {
		return platformDocument{}, fmt.Errorf("configmap %s has no %q key", key, platformDocumentKey)
	}
	return renderPlatformDocument(text, images)
}
