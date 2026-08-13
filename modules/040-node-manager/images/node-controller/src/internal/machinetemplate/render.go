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

package machinetemplate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// RenderContext is the whole world a v2 template sees — five roots, all of them real objects the
// cluster actually holds. The v1 engine instead synthesized a fake helm values tree
// (.Values.nodeManager.internal.cloudProvider.<type>…) that no helm release stood behind, which is
// why a provider had to spell its own name inside its own file.
type RenderContext struct {
	// InstanceClass is the <Provider>InstanceClass spec verbatim. Numbers arrive as JSON numbers
	// (float64), so a template that needs an integer writes `| int`.
	InstanceClass map[string]any
	// Provider is this provider's subtree of its registration Secret.
	Provider map[string]any
	// Zone is the zone this generation is rendered for.
	Zone string
	// NodeGroupName is needed for tags and labels inside spec, nothing else.
	NodeGroupName string
	ClusterUUID   string
	PodSubnet     string
}

// toMap builds the template context, handing the template its own copy of the two maps.
//
// The sandbox keeps sprig's set/unset/merge/mergeOverwrite, and those mutate their argument in
// place. Without the copy a template could edit .instanceClass — and node-controller snapshots
// that very map onto the object right after rendering, so the mutation would be recorded as "the
// InstanceClass this generation was built from". Every later reconcile would then compare the real
// spec against the mutated snapshot, find a difference, and create another generation: a full
// rollout on every pass, forever. The same map is also reused for each zone of one reconcile, so a
// mutation would leak across zones.
//
// Rendering happens only when a generation is created, so the copy is not on any hot path.
func (c RenderContext) toMap() (map[string]any, error) {
	instanceClass, err := deepCopy(c.InstanceClass)
	if err != nil {
		return nil, fmt.Errorf("copy InstanceClass for rendering: %w", err)
	}
	provider, err := deepCopy(c.Provider)
	if err != nil {
		return nil, fmt.Errorf("copy provider configuration for rendering: %w", err)
	}

	return map[string]any{
		"instanceClass": instanceClass,
		"provider":      provider,
		"zone":          c.Zone,
		"nodeGroup":     map[string]any{"name": c.NodeGroupName},
		"cluster": map[string]any{
			"uuid":      c.ClusterUUID,
			"podSubnet": c.PodSubnet,
		},
	}, nil
}

func deepCopy(m map[string]any) (map[string]any, error) {
	if m == nil {
		return nil, nil
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func parseTemplate(text string) (*template.Template, error) {
	// missingkey=error turns a typo in a context path into a loud render failure. Under the v1
	// default a missing path rendered "<no value>" into the object and reached the cloud.
	t, err := template.New("machine-template").Funcs(sandboxFuncMap).Option("missingkey=error").Parse(text)
	if err != nil {
		return nil, fmt.Errorf("parse machine template: %w", err)
	}
	return t, nil
}

// Render renders the contract template and returns the infrastructure MachineTemplate object.
//
// The result carries apiVersion, kind and spec only: metadata belongs to node-controller, which
// owns the name (it encodes the generation), the labels every prune and cleanup selects on, and
// the annotations holding the rollout snapshot. A template that writes its own metadata is
// rejected instead of being silently overwritten — under v1 that freedom is what left dead
// `helm.sh/resource-policy: keep` annotations and hardcoded namespaces in every provider file.
func Render(c *Contract, rc RenderContext) (map[string]any, error) {
	context, err := rc.toMap()
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := c.parsed.Execute(&buf, context); err != nil {
		return nil, fmt.Errorf("render machine template: %w", err)
	}

	obj := map[string]any{}
	if err := yaml.Unmarshal(buf.Bytes(), &obj); err != nil {
		return nil, fmt.Errorf("parse rendered machine template: %w", err)
	}

	if _, ok := obj["metadata"]; ok {
		return nil, fmt.Errorf("rendered machine template must not set metadata: node-controller owns name, labels and annotations")
	}
	apiVersion, _ := obj["apiVersion"].(string)
	if apiVersion == "" {
		return nil, fmt.Errorf("rendered machine template has no apiVersion")
	}
	kind, _ := obj["kind"].(string)
	if kind == "" {
		return nil, fmt.Errorf("rendered machine template has no kind")
	}
	if _, ok := obj["spec"].(map[string]any); !ok {
		return nil, fmt.Errorf("rendered machine template has no spec")
	}

	return obj, nil
}

// ApplyMachineDeploymentFields writes the provider's machineDeployment.additionalFields into the
// spec of the generic MachineDeployment node-controller builds. Both halves of what a provider
// file produces — the machine template and these fields — are rendered here, so the controller
// only ever deals with Kubernetes objects.
//
// It replaces the v1 machine-deployment-spec-patch.yaml: a raw YAML patch with ${zone} substituted
// into it by string replacement.
func ApplyMachineDeploymentFields(spec map[string]any, c *Contract, rc RenderContext) error {
	if len(c.MachineDeployment.parsedFields) == 0 {
		return nil
	}
	context, err := rc.toMap()
	if err != nil {
		return err
	}

	for path, tmpl := range c.MachineDeployment.parsedFields {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, context); err != nil {
			return fmt.Errorf("render machineDeployment.additionalFields[%s]: %w", path, err)
		}
		fields := append([]string{"template", "spec"}, strings.Split(path, ".")...)
		if err := unstructured.SetNestedField(spec, buf.String(), fields...); err != nil {
			return fmt.Errorf("set MachineDeployment field %s: %w", path, err)
		}
	}
	return nil
}
