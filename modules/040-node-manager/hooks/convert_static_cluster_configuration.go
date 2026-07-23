/*
Copyright 2021 Flant JSC

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

package hooks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/lib-dhctl/pkg/yaml/validation"
)

func applyStaticClusterConfigurationFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := new(v1.Secret)
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	return secret.Data["static-cluster-configuration.yaml"], nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "static_cluster_configuration",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{
					"kube-system",
				}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"d8-static-cluster-configuration",
			}},
			FilterFunc: applyStaticClusterConfigurationFilter,
		},
	},
}, convertStaticClusterConfigurationHandler)

const internalNetworkCIDRsPath = "nodeManager.internal.static.internalNetworkCIDRs"

func convertStaticClusterConfigurationHandler(ctx context.Context, input *go_hook.HookInput) error {
	secret := input.Snapshots.Get("static_cluster_configuration")

	if len(secret) == 0 {
		return nil
	}

	staticConfiguration := make([]byte, 0)
	err := secret[0].UnmarshalTo(&staticConfiguration)
	if err != nil {
		return fmt.Errorf("failed to unmarshal first 'static_cluster_configuration' snapshot: %w", err)
	}

	internalNetwork, err := internalNetworkFromStaticConfiguration(staticConfiguration)
	if err != nil {
		return err
	}

	if isEmptyInternalNetwork(internalNetwork) {
		if existing := input.Values.Get(internalNetworkCIDRsPath); len(existing.Array()) > 0 {
			return fmt.Errorf(
				"static-cluster-configuration.yaml no longer contains 'internalNetworkCIDRs', but %q is currently set to %s; "+
					"refusing to silently clear it, since this looks like an accidental config change that could break node network setup",
				internalNetworkCIDRsPath, existing.String(),
			)
		}
	}

	input.Values.Set(internalNetworkCIDRsPath, internalNetwork)
	return nil
}

func internalNetworkFromStaticConfiguration(data []byte) (any, error) {
	if isBlankYAMLDocument(data) {
		return []any{}, nil
	}

	if err := validation.ValidateData([]string{}, &data); err != nil {
		if !errors.Is(err, validation.ErrSchemaNotFound) {
			return nil, err
		}
	}
	res := make(map[any]any)
	err := yaml.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}

	intNet, ok := res["internalNetworkCIDRs"]
	if ok {
		return intNet, nil
	}

	return []any{}, nil
}

// isBlankYAMLDocument reports whether data contains no actual YAML content:
// only whitespace and/or "---" document separators (e.g. "", "\n", "---", "---\n").
func isBlankYAMLDocument(data []byte) bool {
	trimmed := strings.TrimFunc(string(data), func(r rune) bool {
		return r == '-' || unicode.IsSpace(r)
	})
	return trimmed == ""
}

func isEmptyInternalNetwork(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case []any:
		return len(t) == 0
	case string:
		return t == ""
	default:
		return false
	}
}
