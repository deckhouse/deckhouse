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
	"bytes"
	"fmt"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/deckhouse/node-controller/internal/machinetemplate"
)

const (
	clusterTemplateContractKey     = "cluster.yaml"
	clusterTemplateContractVersion = "v1"
	helmManagedByLabel             = "app.kubernetes.io/managed-by"
	helmReleaseNameAnnotation      = "meta.helm.sh/release-name"
	helmReleaseNamespaceAnnotation = "meta.helm.sh/release-namespace"
	werfFailModeAnnotation         = "werf.io/fail-mode"
	werfTrackTerminationAnnotation = "werf.io/track-termination-mode"
)

type clusterTemplateContract struct {
	Version  string `json:"version"`
	Template string `json:"template"`
}

// clusterTemplateContext is the provider-independent input exposed to cluster templates.
type clusterTemplateContext struct {
	Provider map[string]any
	Cluster  clusterTemplateClusterContext
}

type clusterTemplateClusterContext struct {
	Name            string
	Namespace       string
	PodSubnet       string
	ServiceSubnet   string
	Domain          string
	Prefix          string
	MasterEndpoints []map[string]interface{}
	MasterAddresses []string
}

func (c clusterTemplateContext) toMap() map[string]any {
	return map[string]any{
		"provider": c.Provider,
		"cluster": map[string]any{
			"name":            c.Cluster.Name,
			"namespace":       c.Cluster.Namespace,
			"podSubnet":       c.Cluster.PodSubnet,
			"serviceSubnet":   c.Cluster.ServiceSubnet,
			"domain":          c.Cluster.Domain,
			"prefix":          c.Cluster.Prefix,
			"masterEndpoints": c.Cluster.MasterEndpoints,
			"masterAddresses": c.Cluster.MasterAddresses,
		},
	}
}

func parseClusterTemplateContract(data []byte) (*clusterTemplateContract, error) {
	contract := &clusterTemplateContract{}

	if err := sigsyaml.UnmarshalStrict(data, contract); err != nil {
		return nil, fmt.Errorf("parse cluster-template contract: %w", err)
	}

	if contract.Version != clusterTemplateContractVersion {
		return nil, fmt.Errorf(
			"unsupported cluster-template contract version %q, want %q",
			contract.Version,
			clusterTemplateContractVersion,
		)
	}

	if strings.TrimSpace(contract.Template) == "" {
		return nil, fmt.Errorf("cluster-template contract has an empty template")
	}

	return contract, nil
}

// decodeClusterTemplateObjects decodes a rendered multi-document YAML manifest.
func decodeClusterTemplateObjects(data []byte) ([]*unstructured.Unstructured, error) {
	decoder := k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	objects := make([]*unstructured.Unstructured, 0)

	for {
		raw := map[string]interface{}{}

		err := decoder.Decode(&raw)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode cluster-template manifest: %w", err)
		}

		if len(raw) == 0 {
			continue
		}

		object := &unstructured.Unstructured{Object: raw}

		if object.GetAPIVersion() == "" {
			return nil, fmt.Errorf("cluster-template object has no apiVersion")
		}
		if object.GetKind() == "" {
			return nil, fmt.Errorf("cluster-template object has no kind")
		}
		if object.GetName() == "" {
			return nil, fmt.Errorf(
				"cluster-template object %s has no metadata.name",
				object.GetKind(),
			)
		}

		if object.GetNamespace() == "" {
			object.SetNamespace(capiNamespace)
		}

		objects = append(objects, object)
	}

	if len(objects) == 0 {
		return nil, fmt.Errorf("cluster-template rendered no Kubernetes objects")
	}

	return objects, nil
}

func renderClusterTemplate(
	contract *clusterTemplateContract,
	context clusterTemplateContext,
) ([]*unstructured.Unstructured, error) {
	rendered, err := machinetemplate.RenderSandboxedTemplate(
		"cluster-template",
		contract.Template,
		context.toMap(),
	)
	if err != nil {
		return nil, fmt.Errorf("render cluster-template contract: %w", err)
	}

	objects, err := decodeClusterTemplateObjects(rendered)
	if err != nil {
		return nil, err
	}

	return objects, nil
}

// validateClusterTemplateObjects limits the contract to the infrastructure cluster
// declared by the provider registration and the credential Secrets it refers to. The
// whole template is validated before the first object is written, so a bad second YAML
// document cannot leave a partially applied result behind.
func validateClusterTemplateObjects(
	objects []*unstructured.Unstructured,
	expectedAPIVersion string,
	expectedKind string,
	expectedName string,
) error {
	infrastructureCount := 0
	seen := make(map[string]struct{}, len(objects))

	for _, object := range objects {
		if object.GetNamespace() != capiNamespace {
			return fmt.Errorf(
				"provider infrastructure %s %s must be in namespace %s",
				object.GetKind(),
				object.GetName(),
				capiNamespace,
			)
		}

		identity := strings.Join([]string{
			object.GetAPIVersion(),
			object.GetKind(),
			object.GetNamespace(),
			object.GetName(),
		}, "/")
		if _, exists := seen[identity]; exists {
			return fmt.Errorf("cluster-template rendered duplicate object %s", identity)
		}
		seen[identity] = struct{}{}

		isInfrastructureObject :=
			object.GetAPIVersion() == expectedAPIVersion &&
				object.GetKind() == expectedKind
		isAuxiliarySecret :=
			object.GetAPIVersion() == "v1" &&
				object.GetKind() == "Secret"

		if !isInfrastructureObject && !isAuxiliarySecret {
			return fmt.Errorf(
				"cluster-template cannot create %s %s",
				object.GetAPIVersion(),
				object.GetKind(),
			)
		}

		if !isInfrastructureObject {
			continue
		}
		if object.GetName() != expectedName {
			return fmt.Errorf(
				"provider infrastructure %s must be named %q, got %q",
				expectedKind,
				expectedName,
				object.GetName(),
			)
		}

		infrastructureCount++
	}

	if infrastructureCount == 0 {
		return fmt.Errorf(
			"cluster-template did not render expected %s %s",
			expectedKind,
			expectedName,
		)
	}
	return nil
}

func prepareClusterTemplateObject(object *unstructured.Unstructured) {
	labels := object.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels["heritage"] = "deckhouse"
	labels["module"] = "node-manager"
	object.SetLabels(labels)

	// The before-helm migration hook sets the same annotation on live objects before
	// the old manifest is removed. Keeping it in the desired object closes the small
	// window in which a freshly started node-controller can reconcile before Helm has
	// finished pruning its previous release.
	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations["helm.sh/resource-policy"] = "keep"
	object.SetAnnotations(annotations)
}

func removeLegacyHelmMetadata(object *unstructured.Unstructured) bool {
	changed := false

	labels := object.GetLabels()
	if _, ok := labels[helmManagedByLabel]; ok {
		delete(labels, helmManagedByLabel)
		object.SetLabels(labels)
		changed = true
	}

	annotations := object.GetAnnotations()
	for _, key := range []string{
		helmReleaseNameAnnotation,
		helmReleaseNamespaceAnnotation,
		werfFailModeAnnotation,
		werfTrackTerminationAnnotation,
	} {
		if _, ok := annotations[key]; ok {
			delete(annotations, key)
			changed = true
		}
	}
	if changed {
		object.SetAnnotations(annotations)
	}

	return changed
}
