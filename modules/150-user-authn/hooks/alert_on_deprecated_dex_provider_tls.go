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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// This hook scans DexProvider resources for LDAP TLS configurations that
// combine mutually exclusive flags. Dex silently ignores the redundant flag
// (see github.com/dexidp/dex connector/ldap/ldap.go switch on InsecureNoSSL),
// which masks misconfiguration. The CRD now rejects new conflicting objects;
// this hook produces a metric so that pre-existing objects (which CEL does
// not re-validate retroactively, thanks to CRD validation ratcheting) trigger
// the D8DexProviderLDAPTLSConflict alert until updated.

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/user-authn",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "dex_providers_tls_audit",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "DexProvider",
			FilterFunc: filterDexProviderTLSAudit,
		},
	},
}, auditDexProviderLDAPTLSConflicts)

const (
	dexTLSConflictMetricName  = "d8_dex_provider_ldap_tls_conflict"
	dexTLSConflictMetricGroup = "d8_dex_provider_ldap_tls_conflict"

	conflictInsecureNoSSLStartTLS          = "insecureNoSSL+startTLS"
	conflictInsecureNoSSLInsecureSkipVerif = "insecureNoSSL+insecureSkipVerify"
	conflictInsecureNoSSLRootCAData        = "insecureNoSSL+rootCAData"
)

type dexProviderTLSAudit struct {
	Name               string
	IsLDAP             bool
	InsecureNoSSL      bool
	StartTLS           bool
	InsecureSkipVerify bool
	HasRootCAData      bool
}

func filterDexProviderTLSAudit(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	out := dexProviderTLSAudit{Name: obj.GetName()}

	providerType, _, err := unstructured.NestedString(obj.Object, "spec", "type")
	if err != nil {
		return nil, fmt.Errorf("read spec.type from DexProvider %q: %w", out.Name, err)
	}
	if providerType != "LDAP" {
		return out, nil
	}
	out.IsLDAP = true

	out.InsecureNoSSL, _, err = unstructured.NestedBool(obj.Object, "spec", "ldap", "insecureNoSSL")
	if err != nil {
		return nil, fmt.Errorf("read spec.ldap.insecureNoSSL from DexProvider %q: %w", out.Name, err)
	}
	out.StartTLS, _, err = unstructured.NestedBool(obj.Object, "spec", "ldap", "startTLS")
	if err != nil {
		return nil, fmt.Errorf("read spec.ldap.startTLS from DexProvider %q: %w", out.Name, err)
	}
	out.InsecureSkipVerify, _, err = unstructured.NestedBool(obj.Object, "spec", "ldap", "insecureSkipVerify")
	if err != nil {
		return nil, fmt.Errorf("read spec.ldap.insecureSkipVerify from DexProvider %q: %w", out.Name, err)
	}
	rootCA, _, err := unstructured.NestedString(obj.Object, "spec", "ldap", "rootCAData")
	if err != nil {
		return nil, fmt.Errorf("read spec.ldap.rootCAData from DexProvider %q: %w", out.Name, err)
	}
	out.HasRootCAData = rootCA != ""

	return out, nil
}

func auditDexProviderLDAPTLSConflicts(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(dexTLSConflictMetricGroup)

	snaps := input.Snapshots.Get("dex_providers_tls_audit")
	for p, err := range sdkobjectpatch.SnapshotIter[dexProviderTLSAudit](snaps) {
		if err != nil {
			return fmt.Errorf("iterate dex_providers_tls_audit snapshots: %w", err)
		}
		if !p.IsLDAP || !p.InsecureNoSSL {
			continue
		}
		if p.StartTLS {
			emitDexTLSConflict(input, p.Name, conflictInsecureNoSSLStartTLS)
		}
		if p.InsecureSkipVerify {
			emitDexTLSConflict(input, p.Name, conflictInsecureNoSSLInsecureSkipVerif)
		}
		if p.HasRootCAData {
			emitDexTLSConflict(input, p.Name, conflictInsecureNoSSLRootCAData)
		}
	}
	return nil
}

func emitDexTLSConflict(input *go_hook.HookInput, name, conflict string) {
	input.MetricsCollector.Set(
		dexTLSConflictMetricName,
		1,
		map[string]string{"name": name, "conflict": conflict},
		metrics.WithGroup(dexTLSConflictMetricGroup),
	)
}
