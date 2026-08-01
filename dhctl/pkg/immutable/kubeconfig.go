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
// The node writes that kubeconfig for its own use, so its server URL is the
// node-local kube-api-proxy on 127.0.0.1:6445 — an address that means nothing
// anywhere else. Everything else in the file, the cluster CA and the client
// certificate included, is used as it came.
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
