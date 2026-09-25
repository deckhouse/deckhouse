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

package checkcloudapi

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEndpointForUnknownProvider: a provider with no entry returns nothing and no error, which
// the check reports as not applicable. Returning an error instead would refuse a bootstrap over a
// question this installer cannot ask.
func TestEndpointForUnknownProvider(t *testing.T) {
	endpoint, err := EndpointFor("someprivatecloud", []byte(`{"server": "x"}`))

	require.NoError(t, err)
	assert.Nil(t, endpoint)
	assert.False(t, KnownProviders("someprivatecloud"))
}

// TestEveryKnownProviderIsAlsoResolvable guards the two halves of the table against each other:
// a provider KnownProviders admits must actually have a handler behind it.
func TestEveryKnownProviderIsAlsoResolvable(t *testing.T) {
	for provider := range endpointHandlers {
		if !KnownProviders(provider) {
			t.Errorf("provider %q has a handler but KnownProviders says otherwise", provider)
		}
	}
}

// TestCABundleAcceptsBothForms: the schemas describe caBundle as base64, and operators paste raw
// PEM often enough that refusing it would be a check failing on a correct configuration.
func TestCABundleAcceptsBothForms(t *testing.T) {
	const pemBundle = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"

	t.Run("base64", func(t *testing.T) {
		decoded, err := decodeCABundle(base64.StdEncoding.EncodeToString([]byte(pemBundle)))
		require.NoError(t, err)
		assert.Equal(t, pemBundle, decoded)
	})

	t.Run("raw PEM", func(t *testing.T) {
		decoded, err := decodeCABundle(pemBundle)
		require.NoError(t, err)
		assert.Equal(t, pemBundle, decoded)
	})

	t.Run("nothing", func(t *testing.T) {
		decoded, err := decodeCABundle("")
		require.NoError(t, err)
		assert.Empty(t, decoded)
	})

	t.Run("neither", func(t *testing.T) {
		_, err := decodeCABundle("this is not a certificate")
		assert.Error(t, err)
	})
}

// TestAWSWithoutARegion: the endpoint is built from the region, so its absence is a configuration
// error rather than a provider this installer does not know.
func TestAWSWithoutARegion(t *testing.T) {
	_, err := EndpointFor("aws", []byte(`{"providerAccessKeyId": "id"}`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "AWSClusterConfiguration.provider.region is empty")
}

// TestDVPKubeconfigErrors: a kubeconfig that cannot be read is a mistake in the configuration,
// and saying which mistake beats reporting the cloud API as unreachable.
func TestDVPKubeconfigErrors(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantErr  string
	}{
		{
			name:     "no kubeconfig at all",
			provider: `{"namespace": "default"}`,
			wantErr:  "kubeconfigDataBase64 is empty",
		},
		{
			name:     "not base64",
			provider: `{"kubeconfigDataBase64": "not base64!"}`,
			wantErr:  "is not valid base64",
		},
		{
			// The message must not be the dump of the Go type yaml failed to build.
			name:     "base64 of something that is not a kubeconfig",
			provider: `{"kubeconfigDataBase64": "` + base64.StdEncoding.EncodeToString([]byte("just text")) + `"}`,
			wantErr:  "decodes, but is not a kubeconfig",
		},
		{
			name: "a kubeconfig with no clusters",
			provider: `{"kubeconfigDataBase64": "` +
				base64.StdEncoding.EncodeToString([]byte("apiVersion: v1\nkind: Config\nclusters: []\n")) + `"}`,
			wantErr: "names no cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := EndpointFor("dvp", []byte(tt.provider))

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestDVPKubeconfigCarriesItsCA: the certificate authority of the DVP cluster travels with the
// endpoint, or every request to it fails verification for a reason that has nothing to do with
// the cluster being created.
func TestDVPKubeconfigCarriesItsCA(t *testing.T) {
	const ca = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"

	kubeconfig := "apiVersion: v1\nkind: Config\nclusters:\n- name: dvp\n  cluster:\n" +
		"    server: https://dvp.example.com:6443\n" +
		"    certificate-authority-data: " + base64.StdEncoding.EncodeToString([]byte(ca)) + "\n"

	endpoint, err := EndpointFor("dvp", []byte(
		`{"kubeconfigDataBase64": "`+base64.StdEncoding.EncodeToString([]byte(kubeconfig))+`"}`))

	require.NoError(t, err)
	require.NotNil(t, endpoint)
	assert.Equal(t, "https://dvp.example.com:6443", endpoint.URL.String())
	assert.Equal(t, ca, endpoint.CACert)
}

// TestVSphereCABundleIsRead: it is in the schema and was ignored entirely, so a vSphere behind a
// private CA failed verification here while the cluster itself, which is given the bundle, would
// not have.
func TestVSphereCABundleIsRead(t *testing.T) {
	const ca = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"

	endpoint, err := EndpointFor("vsphere", []byte(
		`{"server": "vcenter.example.com", "caBundle": "`+base64.StdEncoding.EncodeToString([]byte(ca))+`"}`))

	require.NoError(t, err)
	require.NotNil(t, endpoint)
	assert.Equal(t, ca, endpoint.CACert)
	assert.Equal(t, "https://vcenter.example.com", endpoint.URL.String())
}

// TestInsecureTravelsWithTheEndpoint: a provider configured insecure must not be verified, or the
// check refuses a configuration the cluster itself accepts.
func TestInsecureTravelsWithTheEndpoint(t *testing.T) {
	endpoint, err := EndpointFor("vsphere", []byte(`{"server": "vcenter.example.com", "insecure": true}`))

	require.NoError(t, err)
	require.NotNil(t, endpoint)
	assert.True(t, endpoint.Insecure)
}

// TestOpenStackEndpoint: OpenStack is the one provider whose CA arrives as plain PEM rather than
// base64, because the cluster configuration carries it that way, and it is never verified as
// insecure.
func TestOpenStackEndpoint(t *testing.T) {
	const ca = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"

	t.Run("the auth URL and the CA", func(t *testing.T) {
		endpoint, err := EndpointFor("openstack", []byte(
			`{"authURL": "https://openstack.company.my:5000/v3", "caCert": `+quoteJSON(ca)+`}`))

		require.NoError(t, err)
		require.NotNil(t, endpoint)
		assert.Equal(t, "https://openstack.company.my:5000/v3", endpoint.URL.String())
		assert.Equal(t, ca, endpoint.CACert)
		assert.False(t, endpoint.Insecure)
		// The failure has to name what to edit, not only the address that did not answer.
		assert.Equal(t, "OpenStackClusterConfiguration.provider.authURL", endpoint.Field)
	})

	t.Run("an auth URL with no scheme", func(t *testing.T) {
		// https is the assumption everywhere: an endpoint written bare must not become a
		// relative URL that dials nothing.
		endpoint, err := EndpointFor("openstack", []byte(`{"authURL": "openstack.company.my:5000/v3"}`))

		require.NoError(t, err)
		require.NotNil(t, endpoint)
		assert.Equal(t, "https", endpoint.URL.Scheme)
	})

	t.Run("no auth URL at all", func(t *testing.T) {
		_, err := EndpointFor("openstack", []byte(`{"domainName": "Default"}`))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "OpenStackClusterConfiguration.provider.authURL")
	})

	t.Run("a provider section that is not an object", func(t *testing.T) {
		_, err := EndpointFor("openstack", []byte(`"not an object"`))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "OpenStack")
	})
}

// quoteJSON renders s as a JSON string literal.
func quoteJSON(s string) string {
	encoded, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}
