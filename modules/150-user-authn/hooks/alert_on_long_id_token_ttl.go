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

package hooks

import (
	"context"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Dex expiry.signingKeys is hardcoded to 6h. idTokenTTL values >= 6h cause signing
// keys to accumulate. ModuleConfig OpenAPI now rejects such values; this hook
// emits a metric for clusters that still have a long TTL (validation ratcheting
// does not re-check existing ModuleConfigs).

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/user-authn",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "module_config_id_token_ttl",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"user-authn"},
			},
			FilterFunc: filterUserAuthnIDTokenTTL,
		},
	},
}, auditLongIDTokenTTL)

const (
	longIDTokenTTLMetricName  = "d8_user_authn_id_token_ttl_too_long"
	longIDTokenTTLMetricGroup = "d8_user_authn_id_token_ttl_too_long"
	maxIDTokenTTL             = 6 * time.Hour
)

func filterUserAuthnIDTokenTTL(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ttl, found, err := unstructured.NestedString(obj.Object, "spec", "settings", "idTokenTTL")
	if err != nil {
		return nil, fmt.Errorf("read spec.settings.idTokenTTL from ModuleConfig %q: %w", obj.GetName(), err)
	}
	if !found {
		return "", nil
	}
	return ttl, nil
}

func auditLongIDTokenTTL(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(longIDTokenTTLMetricGroup)

	snaps := input.Snapshots.Get("module_config_id_token_ttl")
	if len(snaps) != 1 {
		return nil
	}

	var ttl string
	if err := snaps[0].UnmarshalTo(&ttl); err != nil {
		return fmt.Errorf("cannot unmarshal ModuleConfig idTokenTTL: %w", err)
	}
	if ttl == "" {
		return nil
	}

	d, err := time.ParseDuration(ttl)
	if err != nil {
		// Invalid format is rejected by ModuleConfig OpenAPI for new values;
		// skip emitting a metric for unparsable legacy strings.
		input.Logger.Warn("cannot parse user-authn idTokenTTL for alert", "idTokenTTL", ttl, "error", err)
		return nil
	}

	if d >= maxIDTokenTTL {
		input.MetricsCollector.Set(
			longIDTokenTTLMetricName,
			1,
			map[string]string{"id_token_ttl": ttl},
			metrics.WithGroup(longIDTokenTTLMetricGroup),
		)
	}
	return nil
}
