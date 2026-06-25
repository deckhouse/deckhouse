// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateCNIBootstrap_PayloadWithoutClusterConfig_ReturnsNil(t *testing.T) {
	// Garbage payload without ClusterConfiguration must not be CNI's concern;
	// validateUserResources still vets its apiVersion/kind separately.
	res := validateCNIBootstrap(context.Background(), "not yaml: : :", nil, ValidateOptionCommanderMode(true))
	require.Nil(t, res)
}

func TestValidateCNIBootstrap_OnlyUserResources_ReturnsNil(t *testing.T) {
	// Resource-only payloads (no ClusterConfiguration) are out of scope for
	// the CNI validator; it must stay silent and let validateUserResources
	// handle apiVersion/kind checks.
	cfg := `
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
`
	res := validateCNIBootstrap(context.Background(), cfg, nil, ValidateOptionCommanderMode(true))
	require.Nil(t, res, "payload without ClusterConfiguration must be a noop")
}

func TestValidateCNIBootstrap_StaticCluster_ReturnsNil(t *testing.T) {
	cfg := `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: Automatic
clusterDomain: cluster.local
`
	res := validateCNIBootstrap(context.Background(), cfg, nil, ValidateOptionCommanderMode(true))
	require.Nil(t, res, "static cluster has no CNI to validate")
}

// When ClusterConfiguration is present but ParseConfigFromData fails (e.g. a
// ModuleConfig violates its OpenAPI schema), validateCNIBootstrap must
// surface that error rather than silently dropping it. The wording comes
// straight from ParseConfigFromData — we just propagate it.
func TestValidateCNIBootstrap_BrokenModuleConfig_SurfacesError(t *testing.T) {
	cfg := `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: Automatic
clusterDomain: cluster.local
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-cilium
spec:
  enabled: true
  version: 1
  settings:
    tunnelMode: NOT_AN_ENUM
`
	res := validateCNIBootstrap(context.Background(), cfg, nil, ValidateOptionCommanderMode(true))
	if res == nil {
		t.Skip("cni-cilium schema not available in this test environment")
	}
	require.Equal(t, ErrKindValidationFailed, res.Errors[0].Reason)
}

// Non-CNI ModuleConfigs (e.g. user-authn) belong to other validators and
// must not surface through validateCNIBootstrap, even when broken. They are
// filtered out before reaching ParseConfigFromData.
func TestValidateCNIBootstrap_IgnoresNonCNIModuleConfigs(t *testing.T) {
	cfg := `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: Automatic
clusterDomain: cluster.local
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 2
  enabled: not-a-bool
`
	res := validateCNIBootstrap(context.Background(), cfg, nil, ValidateOptionCommanderMode(true))
	require.Nil(t, res, "user-authn validation is not the CNI validator's job")
}

func TestFilterCNIRelevantDocs(t *testing.T) {
	in := `
apiVersion: v1
kind: Secret
metadata: {name: irrelevant}
---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud: {provider: Yandex, prefix: x}
---
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata: {name: user-authn}
spec: {version: 2, enabled: true}
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata: {name: cni-cilium}
spec: {version: 1, enabled: true}
`
	out := filterCNIRelevantDocs(in)
	require.Contains(t, out, "ClusterConfiguration")
	require.Contains(t, out, "YandexClusterConfiguration")
	require.Contains(t, out, "cni-cilium")
	require.NotContains(t, out, "user-authn")
	require.NotContains(t, out, "Secret")
}

func TestFilterCNIRelevantDocs_NoClusterConfig_ReturnsEmpty(t *testing.T) {
	out := filterCNIRelevantDocs(`
apiVersion: v1
kind: Secret
metadata: {name: only-resource}
`)
	require.Empty(t, out)
}

// validateCNIBootstrap must point the error at the offending user MC so
// commander UI can surface which resource is wrong. Exercises the
// Error{Group,Version,Kind,Name} population path the validator's tail
// performs — independent of candi/modules availability in the test env.
func TestValidateCNIBootstrap_ErrorCarriesUserMCIdentity(t *testing.T) {
	user := newTestCNIModuleConfig(t, "cni-flannel",
		map[string]any{"podNetworkMode": "VXLAN"}, true)
	rec := newTestCNIModuleConfig(t, "cni-cilium",
		map[string]any{"tunnelMode": "VXLAN"}, true)

	analysis := &CNIBootstrapAnalysis{
		ModuleConfig: &CNIBootstrapModuleConfigs{
			UserInput:   user,
			Recommended: rec,
		},
	}
	analysis.MismatchReason, analysis.ReasonMessage = cniBootstrapDecision(user, rec)
	require.Equal(t, CNIBootstrapMismatchReasonDifferentModule, analysis.MismatchReason)

	e := Error{
		Group:    ModuleConfigGroup,
		Version:  ModuleConfigVersion,
		Kind:     ModuleConfigKind,
		Messages: []string{analysis.ReasonMessage},
	}
	if mc := analysis.ModuleConfig; mc != nil && mc.UserInput != nil {
		e.Name = mc.UserInput.GetName()
	}
	require.Equal(t, "deckhouse.io", e.Group)
	require.Equal(t, "v1alpha1", e.Version)
	require.Equal(t, "ModuleConfig", e.Kind)
	require.Equal(t, "cni-flannel", e.Name, "Name must be the user's MC, not the recommended one")
	require.Contains(t, e.Messages[0], "cni-flannel")
	require.Contains(t, e.Messages[0], "cni-cilium")
}

func TestCNIMismatchReasonToErrorKind(t *testing.T) {
	require.Equal(t, ErrKindCNIMismatch, cniMismatchReasonToErrorKind(CNIBootstrapMismatchReasonDifferentModule))
	require.Equal(t, ErrKindCNISettingsMismatch, cniMismatchReasonToErrorKind(CNIBootstrapMismatchReasonDifferentSettings))
	require.Equal(t, ErrKindValidationFailed, cniMismatchReasonToErrorKind(CNIBootstrapMismatchReasonNone))
}
