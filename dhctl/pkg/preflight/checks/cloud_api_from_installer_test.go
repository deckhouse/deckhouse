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

package checks

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// dvpProviderConfig renders the provider section of a DVPClusterConfiguration pointing at server.
func dvpProviderConfig(t *testing.T, server string) json.RawMessage {
	t.Helper()

	kubeconfig := fmt.Sprintf(`{"clusters":[{"cluster":{"server":%q,"insecure-skip-tls-verify":true}}]}`, server)
	provider, err := json.Marshal(map[string]any{
		"kubeconfigDataBase64": base64.StdEncoding.EncodeToString([]byte(kubeconfig)),
	})
	require.NoError(t, err)

	return provider
}

func dvpMetaConfig(t *testing.T, server string) *config.MetaConfig {
	t.Helper()

	return &config.MetaConfig{
		ClusterType:  config.CloudClusterType,
		ProviderName: "dvp",
		ProviderClusterConfig: map[string]json.RawMessage{
			"provider": dvpProviderConfig(t, server),
		},
	}
}

func TestCloudAPIFromInstaller(t *testing.T) {
	t.Run("the API answers", func(t *testing.T) {
		api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer api.Close()

		detail, err := CloudAPIFromInstallerCheck{MetaConfig: dvpMetaConfig(t, api.URL)}.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "answers")
	})

	// The credentials belong to the infrastructure utility, which has its own; this check is
	// about connectivity, so an unauthenticated answer is still an answer.
	t.Run("the API refuses the request but answers it", func(t *testing.T) {
		api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer api.Close()

		_, err := CloudAPIFromInstallerCheck{MetaConfig: dvpMetaConfig(t, api.URL)}.Run(t.Context())

		require.NoError(t, err)
	})

	// What the live failure looked like: the installer's container reached the registry and
	// nothing else, and the answer came back from the infrastructure plan instead.
	t.Run("nothing is listening", func(t *testing.T) {
		api := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		url := api.URL
		api.Close()

		_, err := CloudAPIFromInstallerCheck{MetaConfig: dvpMetaConfig(t, url)}.Run(t.Context())

		require.Error(t, err)

		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Fix, "the host or container where dhctl runs",
			"the fix must point at dhctl's own egress: the master this check's sibling talks about does not exist yet")
	})

	t.Run("a proxy in front of an API that is not there", func(t *testing.T) {
		t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")

		_, err := CloudAPIFromInstallerCheck{MetaConfig: dvpMetaConfig(t, "https://api.example.com")}.Run(t.Context())

		require.Error(t, err)

		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Fix, "HTTP_PROXY or HTTPS_PROXY",
			"the proxy dhctl hands the infrastructure utility is the one that has to be named")
	})

	// mc-flow: the provider is configured through its own ModuleConfig and Secret, whose shape is
	// the provider's business. Reading them here would put every provider's schema into dhctl, so
	// the check says what it could not look at.
	t.Run("a configuration with no provider document", func(t *testing.T) {
		_, err := CloudAPIFromInstallerCheck{MetaConfig: &config.MetaConfig{
			ClusterType:  config.CloudClusterType,
			ProviderName: "dvp",
		}}.Run(t.Context())

		require.ErrorIs(t, err, preflight.ErrNotApplicable)
		assert.Contains(t, err.Error(), "DVPClusterConfiguration")
	})

	t.Run("a provider whose API address is not in the configuration", func(t *testing.T) {
		_, err := CloudAPIFromInstallerCheck{MetaConfig: &config.MetaConfig{
			ClusterType:  config.CloudClusterType,
			ProviderName: "somecloud",
		}}.Run(t.Context())

		require.ErrorIs(t, err, preflight.ErrNotApplicable)
	})
}
