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

package immutable

import (
	"fmt"

	"k8s.io/client-go/tools/clientcmd"
)

// RetargetKubeconfig points the collected admin kubeconfig at the address dhctl
// reaches the API server on.
//
// The node puts its own address in the file, which is the right one for an
// operator and the wrong one for this process when it reaches the master
// through a bastion: there the API server is a local forward that exists only
// while the bootstrap runs. So the retargeted copy is the internal one — the
// copy handed to the operator keeps the node's address (see
// saveAdminKubeconfig). Everything else in the file, the cluster CA and the
// client certificate included, is used as it came.
func RetargetKubeconfig(content []byte, server string) ([]byte, error) {
	if server == "" {
		return nil, fmt.Errorf("retarget the admin kubeconfig: server URL is empty")
	}

	kubeconfig, err := clientcmd.Load(content)
	if err != nil {
		return nil, fmt.Errorf("parse the collected admin kubeconfig: %w", err)
	}
	if len(kubeconfig.Clusters) == 0 {
		return nil, fmt.Errorf("the collected admin kubeconfig names no cluster")
	}

	for _, cluster := range kubeconfig.Clusters {
		cluster.Server = server
	}

	out, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("serialize the collected admin kubeconfig: %w", err)
	}

	return out, nil
}

// WithClientKey pairs the document the node handed back with the private key it
// was never given.
//
// The node signs a certificate for the installer's key and returns only that
// certificate — public, and useless to anyone who intercepts it. The key stayed
// here the whole time, so this is where the two halves meet and the result
// becomes credentials.
func WithClientKey(content []byte, clientKeyPEM string) ([]byte, error) {
	if clientKeyPEM == "" {
		return nil, fmt.Errorf("complete the admin kubeconfig: the installer's client key is empty")
	}

	kubeconfig, err := clientcmd.Load(content)
	if err != nil {
		return nil, fmt.Errorf("parse the collected admin kubeconfig: %w", err)
	}
	if len(kubeconfig.AuthInfos) == 0 {
		return nil, fmt.Errorf("the collected admin kubeconfig names no user")
	}

	for _, authInfo := range kubeconfig.AuthInfos {
		if len(authInfo.ClientCertificateData) == 0 {
			return nil, fmt.Errorf("the collected admin kubeconfig carries no client certificate")
		}
		// A node that returned a key would mean the channel carried a secret
		// after all; refuse rather than quietly use it.
		if len(authInfo.ClientKeyData) != 0 {
			return nil, fmt.Errorf("the node returned a private key, which this channel must never carry")
		}
		authInfo.ClientKeyData = []byte(clientKeyPEM)
	}

	out, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("serialize the admin kubeconfig: %w", err)
	}
	return out, nil
}
