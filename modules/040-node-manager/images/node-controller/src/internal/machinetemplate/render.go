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
	"fmt"
	"text/template"

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
	// Provider is this provider's subtree of the d8-node-manager-cloud-provider secret.
	Provider map[string]any
	// Zone is the zone this generation is rendered for.
	Zone string
	// NodeGroupName is needed for tags and labels inside spec, nothing else.
	NodeGroupName string
	ClusterUUID   string
	PodSubnet     string
}

func (c RenderContext) toMap() map[string]any {
	return map[string]any{
		"instanceClass": c.InstanceClass,
		"provider":      c.Provider,
		"zone":          c.Zone,
		"nodeGroup":     map[string]any{"name": c.NodeGroupName},
		"cluster": map[string]any{
			"uuid":      c.ClusterUUID,
			"podSubnet": c.PodSubnet,
		},
	}
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
	t, err := parseTemplate(c.Template)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, rc.toMap()); err != nil {
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
