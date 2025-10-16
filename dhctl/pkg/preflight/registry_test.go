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

package preflight

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/clissh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/gossh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
)

type testState struct{}

func (s *testState) SetGlobalPreflightchecksWasRan() error {
	return nil
}

func (s *testState) GlobalPreflightchecksWasRan() (bool, error) {
	return false, nil
}

func (s *testState) SetCloudPreflightchecksWasRan() error {
	return nil
}

func (s *testState) SetPostCloudPreflightchecksWasRan() error {
	return nil
}

func (s *testState) CloudPreflightchecksWasRan() (bool, error) {
	return false, nil
}

func (s *testState) PostCloudPreflightchecksWasRan() (bool, error) {
	return false, nil
}

func (s *testState) SetStaticPreflightchecksWasRan() error {
	return nil
}

func (s *testState) StaticPreflightchecksWasRan() (bool, error) {
	return false, nil
}

func TestCheckgetProxyFromMetaConfigSuccessHTTPSProxy(t *testing.T) {
	s := require.New(t)

	metaConfig := &config.MetaConfig{
		Registry: config.Registry{
			Data: config.RegistryData{
				Address: "registry.deckhouse.io",
				Scheme:  "https",
			},
		},
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

	proxyURL, noProxyList, err := getProxyFromMetaConfig(metaConfig)
	s.NoError(err)
	s.Equal("https://login:pass@proxy.me", proxyURL.String())
	s.ElementsMatch(noProxyList, []string{"127.0.0.1", "169.254.169.254", "cluster.local", "10.0.0.0/8", "11.0.0.0/8"})
}

func TestCheckgetProxyFromMetaConfigSuccessHTTPProxy(t *testing.T) {
	s := require.New(t)

	metaConfig := &config.MetaConfig{
		Registry: config.Registry{
			Data: config.RegistryData{
				Address: "registry.deckhouse.io",
				Scheme:  "https",
			},
		},
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

	proxyURL, noProxyList, err := getProxyFromMetaConfig(metaConfig)
	s.NoError(err)
	s.Equal("http://login:pass@proxy.me", proxyURL.String())
	s.ElementsMatch(noProxyList, []string{"127.0.0.1", "169.254.169.254", "cluster.local", "10.0.0.0/8", "11.0.0.0/8"})
}

func TestCheckgetProxyFromMetaConfigSuccessNoProxy(t *testing.T) {
	s := require.New(t)

	metaConfig := &config.MetaConfig{
		Registry: config.Registry{
			Data: config.RegistryData{
				Address: "registry.deckhouse.io",
				Scheme:  "https",
			},
		},
		ClusterConfig: map[string]json.RawMessage{
			"clusterDomain":     []byte(`"cluster.local"`),
			"podSubnetCIDR":     []byte(`"10.0.0.0/8"`),
			"serviceSubnetCIDR": []byte(`"11.0.0.0/8"`),
		},
	}

	proxyURL, noProxyList, err := getProxyFromMetaConfig(metaConfig)
	s.NoError(err)
	s.Nil(proxyURL)
	s.Nil(noProxyList)
}

func TestShouldSkipProxyCheck(t *testing.T) {
	s := require.New(t)

	var tests = []struct {
		registryAddress   string
		registryDockerCfg string
		noProxyAddresses  []string
		skipped           bool
	}{
		{
			registryAddress:   "192.168.199.129/d8/deckhouse/ee",
			registryDockerCfg: "registryDockerCfg: eyJhdXRocyI6eyIxOTIuMTY4LjE5OS4xMjkiOnsiYXV0aCI6ImEyOTJZV3hyYjNZNldHVnBiamxoWm1VPSJ9fX0K",
			noProxyAddresses:  []string{"127.0.0.1", "192.168.199.0/24"},
			skipped:           true,
		},
		{
			registryAddress:   "registry.deckhouse.io/ce",
			registryDockerCfg: "",
			noProxyAddresses:  []string{"registry.deckhouse.io"},
			skipped:           true,
		},
		{
			registryAddress:   "quay.io",
			registryDockerCfg: "registryDockerCfg: eyJhdXRocyI6eyJxdWF5LmlvIjp7fX19",
			noProxyAddresses:  []string{"docker.io"},
			skipped:           false,
		},
	}

	for _, test := range tests {
		clusterConfig := fmt.Sprintf(`
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"
proxy:
  httpProxy: http://proxyuser:proxypassword@192.168.199.236:8888
  httpsProxy: http://proxyuser:proxypassword@192.168.199.236:8888
  noProxy: ["%s"]
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: %s
  %s
  registryScheme: HTTP
`, strings.Join(test.noProxyAddresses, `", "`), test.registryAddress, test.registryDockerCfg)

		metaConfig, err := config.ParseConfigFromData(context.TODO(), clusterConfig, config.DummyPreparatorProvider())
		s.NoError(err)

		installer, err := config.PrepareDeckhouseInstallConfig(metaConfig)
		s.NoError(err)

		bootstrapState := &testState{}

		var sshCl node.SSHClient
		if sshclient.IsLegacyMode() {
			sshCl = &gossh.Client{}
		} else {
			sshCl = &clissh.Client{}
		}

		preflightChecker := NewChecker(ssh.NewNodeInterfaceWrapper(sshCl), installer, metaConfig, bootstrapState)

		err = preflightChecker.CheckRegistryAccessThroughProxy(context.Background())
		if test.skipped {
			s.NoError(err)
		} else {
			s.Error(err)
		}
	}
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
					Registry: config.Registry{
						Data: config.RegistryData{
							Address:   "registry.deckhouse.io",
							Path:      "/deckhouse/ce",
							Scheme:    "https",
							CA:        "",
							DockerCfg: "eyJhdXRocyI6IHsgInJlZ2lzdHJ5LmRlY2tob3VzZS5pbyI6IHt9fX0=",
						},
					},
				},
				metaConfig: &config.MetaConfig{
					Registry: config.Registry{
						Data: config.RegistryData{
							Address:   "registry.deckhouse.io",
							Path:      "/deckhouse/ce",
							Scheme:    "https",
							CA:        "",
							DockerCfg: "eyJhdXRocyI6IHsgInJlZ2lzdHJ5LmRlY2tob3VzZS5pbyI6IHt9fX0=",
						},
					},
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &Checker{
				installConfig: tt.fields.installConfig,
				metaConfig:    tt.fields.metaConfig,
			}
			tt.wantErr(t,
				pc.CheckRegistryCredentials(context.Background()),
				fmt.Sprintf("CheckRegistryCredentials()"),
			)
		})
	}
}
