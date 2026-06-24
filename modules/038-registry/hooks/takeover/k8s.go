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

package takeover

import (
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

// KubernetesConfigs builds the snapshots: this hook's own registry-takeover
// store (the phase), the legacy orchestrator's registry-state (presence
// only, read-only), the legacy desired-state registry-config secret, and
// the three readiness probes (agent DaemonSet, cache StatefulSet, cache Lease).
func KubernetesConfigs(takeoverSnap, oldStateSnap, legacyConfigSnap, agentDSSnap, cacheSTSSnap, cacheLeaseSnap string) []go_hook.KubernetesConfig {
	return []go_hook.KubernetesConfig{
		{
			Name:              takeoverSnap,
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: helpers.NamespaceSelector,
			NameSelector:      &types.NameSelector{MatchNames: []string{"registry-takeover"}},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var secret v1core.Secret
				if err := sdk.FromUnstructured(obj, &secret); err != nil {
					return nil, fmt.Errorf("failed to convert secret %q to struct: %w", obj.GetName(), err)
				}
				v := Values{
					Phase:               string(secret.Data["phase"]),
					FlippedAt:           string(secret.Data["flippedAt"]),
					TakingOverStartedAt: string(secret.Data["takingOverStartedAt"]),
				}
				if raw := secret.Data["derived"]; len(raw) > 0 {
					var d DerivedConfig
					if err := json.Unmarshal(raw, &d); err != nil {
						return nil, fmt.Errorf("unmarshal derived from registry-takeover: %w", err)
					}
					v.Derived = &d
				}
				return v, nil
			},
		},
		{
			Name:              oldStateSnap,
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: helpers.NamespaceSelector,
			NameSelector:      &types.NameSelector{MatchNames: []string{"registry-state"}},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				// Presence only — the name is enough to know the old arch ran here.
				return obj.GetName(), nil
			},
		},
		{
			Name:              legacyConfigSnap,
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: helpers.NamespaceSelector,
			NameSelector:      &types.NameSelector{MatchNames: []string{"registry-config"}},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var secret v1core.Secret
				if err := sdk.FromUnstructured(obj, &secret); err != nil {
					return nil, fmt.Errorf("failed to convert secret %q to struct: %w", obj.GetName(), err)
				}
				return LegacyConfig{
					Mode:       string(secret.Data["mode"]),
					ImagesRepo: string(secret.Data["imagesRepo"]),
					Scheme:     string(secret.Data["scheme"]),
					CA:         string(secret.Data["ca"]),
					Username:   string(secret.Data["username"]),
					Password:   string(secret.Data["password"]),
					TTL:        string(secret.Data["ttl"]),
				}, nil
			},
		},
		{
			Name:              agentDSSnap,
			ApiVersion:        "apps/v1",
			Kind:              "DaemonSet",
			NamespaceSelector: helpers.NamespaceSelector,
			NameSelector:      &types.NameSelector{MatchNames: []string{"registry-agent"}},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var ds appsv1.DaemonSet
				if err := sdk.FromUnstructured(obj, &ds); err != nil {
					return nil, fmt.Errorf("convert DaemonSet %q: %w", obj.GetName(), err)
				}
				return AgentDSStatus{
					NumberReady:            int(ds.Status.NumberReady),
					DesiredNumberScheduled: int(ds.Status.DesiredNumberScheduled),
				}, nil
			},
		},
		{
			Name:              cacheSTSSnap,
			ApiVersion:        "apps/v1",
			Kind:              "StatefulSet",
			NamespaceSelector: helpers.NamespaceSelector,
			NameSelector:      &types.NameSelector{MatchNames: []string{"registry-cache"}},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var sts appsv1.StatefulSet
				if err := sdk.FromUnstructured(obj, &sts); err != nil {
					return nil, fmt.Errorf("convert StatefulSet %q: %w", obj.GetName(), err)
				}
				replicas := 0
				if sts.Spec.Replicas != nil {
					replicas = int(*sts.Spec.Replicas)
				}
				return CacheSTSStatus{
					ReadyReplicas: int(sts.Status.ReadyReplicas),
					Replicas:      replicas,
				}, nil
			},
		},
		{
			Name:              cacheLeaseSnap,
			ApiVersion:        "coordination.k8s.io/v1",
			Kind:              "Lease",
			NamespaceSelector: helpers.NamespaceSelector,
			NameSelector:      &types.NameSelector{MatchNames: []string{"registry-cache-leader"}},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var lease coordinationv1.Lease
				if err := sdk.FromUnstructured(obj, &lease); err != nil {
					return nil, fmt.Errorf("convert Lease %q: %w", obj.GetName(), err)
				}
				out := CacheLeaseStatus{}
				if lease.Spec.HolderIdentity != nil {
					out.Holder = *lease.Spec.HolderIdentity
				}
				if lease.Spec.RenewTime != nil {
					out.RenewTime = lease.Spec.RenewTime.Time
				}
				if lease.Spec.LeaseDurationSeconds != nil {
					out.LeaseDurationSeconds = int(*lease.Spec.LeaseDurationSeconds)
				}
				return out, nil
			},
		},
	}
}
