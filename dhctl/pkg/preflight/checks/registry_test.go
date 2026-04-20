// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_mocks "github.com/deckhouse/deckhouse/dhctl/pkg/config/registrymocks"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks/utils"
)

func TestCheckgetProxyFromMetaConfigSuccessHTTPSProxy(t *testing.T) {
	s := require.New(t)

	metaConfig := &config.MetaConfig{
		Registry: registry_mocks.ConfigBuilder(
			registry_mocks.WithImagesRepo("registry.deckhouse.io/test"),
			registry_mocks.WithSchemeHTTPS(),
		),
		ClusterConfig: map[string]json.RawMessage{
			"clusterDomain":     []byte(`"cluster.local"`),
			"podSubnetCIDR":     []byte(`"10.0.0.0/8"`),
			"serviceSubnetCIDR": []byte(`"11.0.0.0/8"`),
			"proxy": []byte(
				`{
                   "httpsProxy": "https://login:pass@proxy.me",
                   "httpProxy":  "http://login:pass@proxy.me"
                 }`),
		},
	}

	proxyURL, noProxyList, err := utils.GetProxyFromMetaConfig(metaConfig)
	s.NoError(err)
	s.Equal("https://login:pass@proxy.me", proxyURL.String())
	s.ElementsMatch(noProxyList, []string{"127.0.0.1", "169.254.169.254", "cluster.local", "10.0.0.0/8", "11.0.0.0/8"})
}

func TestCheckgetProxyFromMetaConfigSuccessHTTPProxy(t *testing.T) {
	s := require.New(t)

	metaConfig := &config.MetaConfig{
		Registry: registry_mocks.ConfigBuilder(
			registry_mocks.WithImagesRepo("registry.deckhouse.io/test"),
			registry_mocks.WithSchemeHTTPS(),
		),
		ClusterConfig: map[string]json.RawMessage{
			"clusterDomain":     []byte(`"cluster.local"`),
			"podSubnetCIDR":     []byte(`"10.0.0.0/8"`),
			"serviceSubnetCIDR": []byte(`"11.0.0.0/8"`),
			"proxy": []byte(
				`{
                   "httpProxy":  "http://login:pass@proxy.me"
                 }`),
		},
	}

	proxyURL, noProxyList, err := utils.GetProxyFromMetaConfig(metaConfig)
	s.NoError(err)
	s.Equal("http://login:pass@proxy.me", proxyURL.String())
	s.ElementsMatch(noProxyList, []string{"127.0.0.1", "169.254.169.254", "cluster.local", "10.0.0.0/8", "11.0.0.0/8"})
}

func TestCheckgetProxyFromMetaConfigSuccessNoProxy(t *testing.T) {
	s := require.New(t)

	metaConfig := &config.MetaConfig{
		Registry: registry_mocks.ConfigBuilder(
			registry_mocks.WithImagesRepo("registry.deckhouse.io/test"),
			registry_mocks.WithSchemeHTTPS(),
		),
		ClusterConfig: map[string]json.RawMessage{
			"clusterDomain":     []byte(`"cluster.local"`),
			"podSubnetCIDR":     []byte(`"10.0.0.0/8"`),
			"serviceSubnetCIDR": []byte(`"11.0.0.0/8"`),
		},
	}

	proxyURL, noProxyList, err := utils.GetProxyFromMetaConfig(metaConfig)
	s.NoError(err)
	s.Nil(proxyURL)
	s.Nil(noProxyList)
}

func TestCheckResponseSuccess_OKResponse(t *testing.T) {
	s := require.New(t)
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}}
	resp.Header.Set("Docker-Distribution-API-Version", "registry/2.0")
	s.Nil(checkResponseIsFromDockerRegistry(resp))
}

func TestCheckResponseSuccess_UnauthorizedResponse(t *testing.T) {
	s := require.New(t)
	resp := &http.Response{StatusCode: http.StatusUnauthorized, Header: http.Header{}}
	resp.Header.Set("Docker-Distribution-API-Version", "registry/2.0")
	s.Nil(checkResponseIsFromDockerRegistry(resp))
}

func TestCheckResponse_NoAPIVersionHeader(t *testing.T) {
	s := require.New(t)
	resp := &http.Response{StatusCode: http.StatusUnauthorized, Header: http.Header{}}
	s.ErrorIs(checkResponseIsFromDockerRegistry(resp), ErrRegistryUnreachable)
}

func TestCheckResponse_WrongStatus(t *testing.T) {
	s := require.New(t)
	resp := &http.Response{StatusCode: http.StatusForbidden, Header: http.Header{}}
	s.ErrorIs(checkResponseIsFromDockerRegistry(resp), ErrRegistryUnreachable)
}

func TestCheckRegistryCredentials(t *testing.T) {
	type fields struct {
		installConfig *config.DeckhouseInstaller
		metaConfig    *config.MetaConfig
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "check registry.deckhouse.io/deckhouse/ce",
			fields: fields{
				installConfig: &config.DeckhouseInstaller{
					DevBranch: "pr0001",
					Registry: registry_mocks.ConfigBuilder(
						registry_mocks.WithImagesRepo("registry.deckhouse.io/deckhouse/ce"),
						registry_mocks.WithSchemeHTTPS(),
					),
				},
				metaConfig: &config.MetaConfig{
					Registry: registry_mocks.ConfigBuilder(
						registry_mocks.WithImagesRepo("registry.deckhouse.io/deckhouse/ce"),
						registry_mocks.WithSchemeHTTPS(),
					),
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := RegistryCredentials(tt.fields.metaConfig, tt.fields.installConfig)
			tt.wantErr(t, check.Run(context.Background()), fmt.Sprintf("CheckRegistryCredentials()"))
		})
	}
}
