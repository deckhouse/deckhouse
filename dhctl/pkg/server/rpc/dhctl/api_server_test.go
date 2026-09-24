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

package dhctl

import (
	"testing"

	"github.com/stretchr/testify/require"

	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/helper"
)

// The endpoint Deckhouse Commander sends with every check and converge request.
// Dropping any of these fields on the floor sends the operation over SSH instead,
// which is what regressed in v1.77.0.
func wantedConnection() *helper.APIServerConnection {
	return &helper.APIServerConnection{
		URL:                      "https://ampg-api-server-0d9f.example:6443",
		Token:                    "commander-agent-token",
		InsecureSkipTLSVerify:    true,
		CertificateAuthorityData: []byte("ca-data"),
	}
}

func TestCheckAPIServer(t *testing.T) {
	tests := []struct {
		name    string
		options *pb.CheckStartOptions
		want    *helper.APIServerConnection
	}{
		{
			name: "commander passes an api server endpoint: check must use it",
			options: &pb.CheckStartOptions{
				CommanderMode:                     true,
				ApiServerUrl:                      "https://ampg-api-server-0d9f.example:6443",
				ApiServerToken:                    "commander-agent-token",
				ApiServerInsecureSkipTlsVerify:    true,
				ApiServerCertificateAuthorityData: "ca-data",
			},
			want: wantedConnection(),
		},
		{
			name: "no endpoint: the ssh path stands",
			options: &pb.CheckStartOptions{
				CommanderMode: true,
			},
		},
		{
			name: "outside commander mode the endpoint is not honoured",
			options: &pb.CheckStartOptions{
				ApiServerUrl: "https://ampg-api-server-0d9f.example:6443",
			},
		},
		{
			name: "no options at all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, checkAPIServer(tt.options))
		})
	}
}

func TestConvergeAPIServer(t *testing.T) {
	tests := []struct {
		name    string
		options *pb.ConvergeStartOptions
		want    *helper.APIServerConnection
	}{
		{
			name: "commander passes an api server endpoint: converge must use it",
			options: &pb.ConvergeStartOptions{
				CommanderMode:                     true,
				ApiServerUrl:                      "https://ampg-api-server-0d9f.example:6443",
				ApiServerToken:                    "commander-agent-token",
				ApiServerInsecureSkipTlsVerify:    true,
				ApiServerCertificateAuthorityData: "ca-data",
			},
			want: wantedConnection(),
		},
		{
			name: "no endpoint: the ssh path stands",
			options: &pb.ConvergeStartOptions{
				CommanderMode: true,
			},
		},
		{
			name: "outside commander mode the endpoint is not honoured",
			options: &pb.ConvergeStartOptions{
				ApiServerUrl: "https://ampg-api-server-0d9f.example:6443",
			},
		},
		{
			name: "no options at all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, convergeAPIServer(tt.options))
		})
	}
}

func TestPrepareConnectionProcessName(t *testing.T) {
	require.Equal(t, "Preparing Kubernetes API client", prepareConnectionProcessName(wantedConnection()))
	require.Equal(t, "Preparing SSH client", prepareConnectionProcessName(nil))
	require.Equal(t, "Preparing SSH client", prepareConnectionProcessName(&helper.APIServerConnection{}))
}

// TestAPIServerRestConfig pins how the endpoint reaches client-go: the same
// mapping DKP used up to v1.76.
func TestAPIServerRestConfig(t *testing.T) {
	restConfig := wantedConnection().RestConfig()

	require.NotNil(t, restConfig)
	require.Equal(t, "https://ampg-api-server-0d9f.example:6443", restConfig.Host)
	require.Equal(t, "commander-agent-token", restConfig.BearerToken)
	require.True(t, restConfig.TLSClientConfig.Insecure)
	require.Equal(t, []byte("ca-data"), restConfig.TLSClientConfig.CAData)

	require.Nil(t, (*helper.APIServerConnection)(nil).RestConfig())
	require.Nil(t, (&helper.APIServerConnection{}).RestConfig())
}
