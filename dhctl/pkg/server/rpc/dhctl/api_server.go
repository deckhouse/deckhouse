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
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util"
)

// commanderAPIServer returns the Kubernetes API endpoint the operation must be
// driven over, or nil when the request carries none and the SSH path stands.
//
// Deckhouse Commander sends the endpoint with every check and converge request in
// commander mode: the URL is its AMPG tunnel to the managed cluster's
// kube-apiserver and the token belongs to the agent's service account. Honouring
// it keeps dhctl off SSH. The alternative is an SSH session to a master and a root
// kubectl proxy there on every run, and Commander also manages clusters whose
// masters it cannot reach over SSH at all.
func commanderAPIServer(
	commanderMode bool,
	url string,
	token string,
	insecureSkipTLSVerify bool,
	certificateAuthorityData string,
) *helper.APIServerConnection {
	if !commanderMode || url == "" {
		return nil
	}

	return &helper.APIServerConnection{
		URL:                      url,
		Token:                    token,
		InsecureSkipTLSVerify:    insecureSkipTLSVerify,
		CertificateAuthorityData: util.StringToBytes(certificateAuthorityData),
	}
}

// checkAPIServer reads the endpoint out of a check request.
func checkAPIServer(options *pb.CheckStartOptions) *helper.APIServerConnection {
	if options == nil {
		return nil
	}

	return commanderAPIServer(
		options.CommanderMode,
		options.ApiServerUrl,
		options.ApiServerToken,
		options.ApiServerInsecureSkipTlsVerify,
		options.ApiServerCertificateAuthorityData,
	)
}

// convergeAPIServer reads the endpoint out of a converge request.
func convergeAPIServer(options *pb.ConvergeStartOptions) *helper.APIServerConnection {
	if options == nil {
		return nil
	}

	return commanderAPIServer(
		options.CommanderMode,
		options.ApiServerUrl,
		options.ApiServerToken,
		options.ApiServerInsecureSkipTlsVerify,
		options.ApiServerCertificateAuthorityData,
	)
}

// prepareConnectionProcessName names the log process that builds the providers,
// so the task log says which way the cluster is reached.
func prepareConnectionProcessName(apiServer *helper.APIServerConnection) string {
	if apiServer.Defined() {
		return "Preparing Kubernetes API client"
	}

	return "Preparing SSH client"
}
