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

package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// The proxy follows the upstream the controller holds: the status, not the spec.
func TestFilterEffectiveUpstreamReadsTheHeldUpstream(t *testing.T) {
	config := &registryv1alpha1.RegistryConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "registry"},
		Spec: registryv1alpha1.RegistryConfigSpec{Primary: registryv1alpha1.PrimarySource{
			Upstream: &registryv1alpha1.Upstream{Endpoint: registryv1alpha1.Endpoint{Host: "typo.example.com"}},
		}},
		Status: registryv1alpha1.RegistryConfigStatus{
			EffectiveUpstream: &registryv1alpha1.Upstream{Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTPS, Host: "registry.deckhouse.io", Path: "/deckhouse/ee", CA: "UPSTREAM-CA",
			}},
		},
	}
	got, err := filterEffectiveUpstream(toUnstructured(t, config))
	require.NoError(t, err)
	assert.Equal(t, namedUpstream{Name: "registry", Upstream: &ProxyUpstream{
		Host: "registry.deckhouse.io", Path: "/deckhouse/ee", Scheme: "https", CA: "UPSTREAM-CA",
	}}, got)

	// None accepted yet: the nodes run the configured one, and so does the proxy.
	config.Status.EffectiveUpstream = nil
	got, err = filterEffectiveUpstream(toUnstructured(t, config))
	require.NoError(t, err)
	assert.Equal(t, "typo.example.com", got.(namedUpstream).Upstream.Host)

	config.Spec.Primary.Upstream = nil
	got, err = filterEffectiveUpstream(toUnstructured(t, config))
	require.NoError(t, err)
	assert.Nil(t, got.(namedUpstream).Upstream, "none at all, none proxied to")
}

func TestFilterUpstreamAuthReadsTheUpstreamKey(t *testing.T) {
	secret := &v1core.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: registry_const.AuthSecretName, Namespace: "d8-system"},
		Data: map[string][]byte{
			registry_const.AuthKeyUpstream: []byte("dXNlcm5hbWU6cGFzc3dvcmQ="),
			registry_const.AuthKeyStorage:  []byte("ZGgtdGVzdC1zZWNyZXQtMDEyMzQ1Njc4"),
		},
	}
	got, err := filterUpstreamAuth(toUnstructured(t, secret))
	require.NoError(t, err)
	assert.Equal(t, namedAuth{Name: registry_const.AuthSecretName, Auth: "dXNlcm5hbWU6cGFzc3dvcmQ="}, got)
}
