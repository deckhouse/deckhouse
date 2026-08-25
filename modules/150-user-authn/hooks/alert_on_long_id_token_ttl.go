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

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// Dex expiry.signingKeys is hardcoded to 6h. idTokenTTL values >= 6h force Dex to
// retain every previous signing key until tokens expire, so the key set grows
// without bound. ModuleConfig OpenAPI now rejects such values; this hook emits
// a metric for clusters that still have a long TTL (validation ratcheting does
// not re-check existing ModuleConfigs).

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

// userAuthnIDTokenTTL distinguishes "field absent" from "field present but empty".
// The OpenAPI pattern formally accepts an empty string, so Present must be tracked.
type userAuthnIDTokenTTL struct {
	Present bool
	TTL     string
}

func filterUserAuthnIDTokenTTL(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ttl, found, err := unstructured.NestedString(obj.Object, "spec", "settings", "idTokenTTL")
	if err != nil {
		return nil, fmt.Errorf("read spec.settings.idTokenTTL from ModuleConfig %q: %w", obj.GetName(), err)
	}
	return userAuthnIDTokenTTL{Present: found, TTL: ttl}, nil
}

func auditLongIDTokenTTL(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(longIDTokenTTLMetricGroup)

	snaps := input.Snapshots.Get("module_config_id_token_ttl")
	for snap, err := range sdkobjectpatch.SnapshotIter[userAuthnIDTokenTTL](snaps) {
		if err != nil {
			return fmt.Errorf("iterate module_config_id_token_ttl snapshots: %w", err)
		}
		if !snap.Present {
			continue
		}
		if snap.TTL == "" {
			input.Logger.Warn("user-authn idTokenTTL is set to an empty string; treating as unset")
			continue
		}

		d, err := time.ParseDuration(snap.TTL)
		if err != nil {
			// Invalid format is rejected by ModuleConfig OpenAPI for new values;
			// skip emitting a metric for unparsable legacy strings.
			input.Logger.Warn("cannot parse user-authn idTokenTTL", "idTokenTTL", snap.TTL, "error", err)
			continue
		}

		if d >= maxIDTokenTTL {
			input.MetricsCollector.Set(
				longIDTokenTTLMetricName,
				1,
				map[string]string{"id_token_ttl": snap.TTL},
				metrics.WithGroup(longIDTokenTTLMetricGroup),
			)
		}
	}
	return nil
}
