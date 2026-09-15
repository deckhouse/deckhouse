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

package cloudprovider

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/machinetemplate"
)

const (
	clusterTemplateContractVersion = "v1"
	capiNamespace                  = common.MachineNamespace
)

// RenderData contains the common data loaded from the provider and cluster. Controllers add
// only the fields needed by their template before rendering it.
type RenderData struct {
	Provider map[string]any
	Cluster  machinetemplate.ClusterFacts

	Prefix       string
	ControlPlane []ControlPlaneEndpoint

	InstanceClass map[string]any
	Zone          string
	NodeGroupName string
}

type ControlPlaneEndpoint struct {
	Host string
	Port int
}

// MachineTemplate is the parsed CAPI machine-template contract.
type MachineTemplate struct {
	Contract *machinetemplate.Contract
}

func (t *MachineTemplate) renderContext(data RenderData) machinetemplate.RenderContext {
	return machinetemplate.RenderContext{
		InstanceClass: data.InstanceClass,
		Provider:      data.Provider,
		Zone:          data.Zone,
		NodeGroupName: data.NodeGroupName,
		Cluster:       data.Cluster,
	}
}

func (t *MachineTemplate) Render(data RenderData) (map[string]any, error) {
	return machinetemplate.Render(t.Contract, t.renderContext(data))
}

func (t *MachineTemplate) ApplyMachineDeploymentFields(spec map[string]any, data RenderData) error {
	return machinetemplate.ApplyMachineDeploymentFields(spec, t.Contract, t.renderContext(data))
}

type templateEnvelope struct {
	Version  string `json:"version"`
	Template string `json:"template"`
}

func parseTemplateEnvelope(name string, data []byte) (*template.Template, error) {
	envelope := templateEnvelope{}
	if err := sigsyaml.UnmarshalStrict(data, &envelope); err != nil {
		return nil, fmt.Errorf("parse %s contract: %w", name, err)
	}
	if envelope.Version != clusterTemplateContractVersion {
		return nil, fmt.Errorf("unsupported %s contract version %q, want %q", name, envelope.Version, clusterTemplateContractVersion)
	}
	if strings.TrimSpace(envelope.Template) == "" {
		return nil, fmt.Errorf("%s contract has an empty template", name)
	}
	return machinetemplate.ParseSandboxed(name, envelope.Template)
}

// ClusterTemplate renders exactly one infrastructure cluster object.
type ClusterTemplate struct {
	parsed             *template.Template
	expectedAPIVersion string
	expectedKind       string
	expectedName       string
}

func newClusterTemplate(data []byte, registration Registration) (*ClusterTemplate, error) {
	parsed, err := parseTemplateEnvelope("cluster.yaml", data)
	if err != nil {
		return nil, err
	}
	return &ClusterTemplate{
		parsed:             parsed,
		expectedAPIVersion: registration.CAPIClusterAPIVersion,
		expectedKind:       registration.CAPIClusterKind,
		expectedName:       registration.CAPIClusterName,
	}, nil
}

func (t *ClusterTemplate) context(data RenderData) map[string]any {
	context := map[string]any{
		"provider": data.Provider,
		"cluster":  data.Cluster.ToMap(),
		"prefix":   data.Prefix,
	}
	if len(data.ControlPlane) > 0 {
		endpoints := make([]map[string]any, 0, len(data.ControlPlane))
		for _, endpoint := range data.ControlPlane {
			endpoints = append(endpoints, map[string]any{"host": endpoint.Host, "port": endpoint.Port})
		}
		context["controlPlane"] = map[string]any{"endpoints": endpoints}
	}
	return context
}

func (t *ClusterTemplate) Render(data RenderData) (*unstructured.Unstructured, error) {
	rendered, err := machinetemplate.ExecuteSandboxed(t.parsed, t.context(data))
	if err != nil {
		return nil, err
	}
	object, err := decodeExactlyOne("cluster.yaml", rendered)
	if err != nil {
		return nil, err
	}
	if object.GetAPIVersion() != t.expectedAPIVersion || object.GetKind() != t.expectedKind {
		return nil, fmt.Errorf(
			"cluster.yaml renders %s/%s, registration declares %s/%s",
			object.GetAPIVersion(), object.GetKind(), t.expectedAPIVersion, t.expectedKind,
		)
	}
	if object.GetName() != t.expectedName {
		return nil, fmt.Errorf("cluster.yaml renders %s named %q, registration declares %q", t.expectedKind, object.GetName(), t.expectedName)
	}
	return object, nil
}

// CredentialsTemplate renders the optional credentials Secret referenced by an infrastructure
// cluster object.
type CredentialsTemplate struct {
	parsed *template.Template
}

func newCredentialsTemplate(data []byte) (*CredentialsTemplate, error) {
	parsed, err := parseTemplateEnvelope("credentials.yaml", data)
	if err != nil {
		return nil, err
	}
	return &CredentialsTemplate{parsed: parsed}, nil
}

func (t *CredentialsTemplate) Render(data RenderData) (*corev1.Secret, error) {
	context := map[string]any{
		"provider": data.Provider,
		"cluster":  data.Cluster.ToMap(),
	}
	rendered, err := machinetemplate.ExecuteSandboxed(t.parsed, context)
	if err != nil {
		return nil, err
	}
	object, err := decodeExactlyOne("credentials.yaml", rendered)
	if err != nil {
		return nil, err
	}
	if object.GetAPIVersion() != "v1" || object.GetKind() != "Secret" {
		return nil, fmt.Errorf("credentials.yaml renders %s/%s, want v1/Secret", object.GetAPIVersion(), object.GetKind())
	}

	secret := &corev1.Secret{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, secret); err != nil {
		return nil, fmt.Errorf("decode credentials Secret: %w", err)
	}
	return secret, nil
}

func decodeExactlyOne(name string, data []byte) (*unstructured.Unstructured, error) {
	decoder := k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	var result *unstructured.Unstructured

	for {
		raw := map[string]any{}
		err := decoder.Decode(&raw)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode %s output: %w", name, err)
		}
		if len(raw) == 0 {
			continue
		}
		if result != nil {
			return nil, fmt.Errorf("%s rendered more than one object", name)
		}
		result = &unstructured.Unstructured{Object: raw}
	}

	if result == nil {
		return nil, fmt.Errorf("%s rendered no object", name)
	}
	if result.GetAPIVersion() == "" || result.GetKind() == "" || result.GetName() == "" {
		return nil, fmt.Errorf("%s output has no apiVersion, kind or metadata.name", name)
	}
	switch result.GetNamespace() {
	case "":
		result.SetNamespace(capiNamespace)
	case capiNamespace:
	default:
		return nil, fmt.Errorf("%s output must be in namespace %s, got %s", name, capiNamespace, result.GetNamespace())
	}

	return result, nil
}
