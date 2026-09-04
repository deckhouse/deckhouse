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

	"github.com/deckhouse/node-controller/internal/machinetemplate"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	sigsyaml "sigs.k8s.io/yaml"
)

const (
	clusterTemplateContractKey     = "cluster-template.yaml"
	clusterTemplateContractVersion = "v1"
)

// clusterTemplateContract describes the provider-owned template stored in
// d8-cloud-provider-<type>-capi Secret.
type clusterTemplateContract struct {
	Version  string `json:"version"`
	Template string `json:"template"`
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

// decodeClusterTemplateObjects converts a rendered multi-document YAML
// into Kubernetes unstructured objects.
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

// renderClusterTemplate renders a provider-owned cluster template and
// converts its YAML documents into Kubernetes objects.
func renderClusterTemplate(
	contract *clusterTemplateContract,
	provider map[string]any,
	cluster map[string]any,
) ([]*unstructured.Unstructured, error) {
	context := map[string]any{
		"provider": provider,
		"cluster":  cluster,
	}

	rendered, err := machinetemplate.RenderSandboxedTemplate(
		"cluster-template",
		contract.Template,
		context,
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
