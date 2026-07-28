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

package nodebootstrap

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodeconfig"
)

// renderBootstrapData renders the cloud-config userdata a machine boots with: a
// full NodeConfig with the machine's name already filled in — no __NODE_NAME__
// placeholder — plus the bootstrap token kubelet presents on first contact.
func renderBootstrapData(ctx context.Context, cl client.Client, reader client.Reader, ng *v1.NodeGroup, machineName string) ([]byte, error) {
	spec, err := nodeconfig.RenderBootstrapSpec(ctx, cl, reader, ng, machineName)
	if err != nil {
		return nil, fmt.Errorf("render bootstrap spec: %w", err)
	}

	token, err := readBootstrapToken(ctx, reader, ng.Name)
	if err != nil {
		return nil, err
	}
	spec.Kubelet.BootstrapToken = token

	return wrapCloudConfig(spec, machineName, ng.Name)
}

// wrapCloudConfig marshals the NodeConfig for the machine and wraps it in the
// cloud-config document the on-node loader reads from /config/nodeconfig.yaml.
func wrapCloudConfig(spec internalv1alpha1.NodeSpec, machineName, ngName string) ([]byte, error) {
	config := &internalv1alpha1.NodeConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: internalv1alpha1.GroupVersion.String(),
			Kind:       "NodeConfig",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   machineName,
			Labels: map[string]string{nodecommon.NodeGroupLabel: ngName},
		},
		Spec: spec,
	}

	configYAML, err := sigsyaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal NodeConfig: %w", err)
	}

	cloudConfig := map[string]any{
		"write_files": []map[string]any{{
			"path":    nodeConfigPath,
			"content": string(configYAML),
		}},
	}
	body, err := sigsyaml.Marshal(cloudConfig)
	if err != nil {
		return nil, fmt.Errorf("marshal cloud-config: %w", err)
	}

	return append([]byte("#cloud-config\n"), body...), nil
}

// readBootstrapToken returns the newest non-expired bootstrap token of the
// NodeGroup, the same per-group rotating token bashible nodes are given. The
// token secrets live in kube-system, one or more per group, labelled with the
// group name.
func readBootstrapToken(ctx context.Context, reader client.Reader, ngName string) (string, error) {
	req, err := labels.NewRequirement(bootstrapTokenNGLabel, selection.Equals, []string{ngName})
	if err != nil {
		return "", fmt.Errorf("build bootstrap-token selector: %w", err)
	}

	secrets := &corev1.SecretList{}
	if err := reader.List(ctx, secrets,
		client.InNamespace(kubeSystemNS),
		client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(*req)},
	); err != nil {
		return "", fmt.Errorf("list bootstrap tokens: %w", err)
	}

	token, newest := "", time.Time{}
	for i := range secrets.Items {
		sec := &secrets.Items[i]
		if sec.Type != corev1.SecretTypeBootstrapToken {
			continue
		}
		if raw, ok := sec.Data["expiration"]; ok {
			expire, err := time.Parse(time.RFC3339, string(raw))
			if err != nil || time.Until(expire) < 0 {
				continue
			}
		}
		id, hasID := sec.Data["token-id"]
		secretPart, hasSecret := sec.Data["token-secret"]
		if !hasID || !hasSecret {
			continue
		}
		if token == "" || sec.CreationTimestamp.After(newest) {
			token = string(id) + "." + string(secretPart)
			newest = sec.CreationTimestamp.Time
		}
	}

	if token == "" {
		return "", fmt.Errorf("no valid bootstrap token for NodeGroup %s", ngName)
	}
	return token, nil
}
