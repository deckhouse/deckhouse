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
	"context"
	"fmt"
	"net/url"
	"strconv"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
)

// defaultAPIServerPort is what an https URL without one means.
const defaultAPIServerPort = 443

// OpenKubeconfigChannel makes a kubeconfig whose server only exists inside the
// cluster network usable from outside: it forwards that exact address through
// the bastion and returns a client configuration pointed at the forward, in
// memory, together with the tunnel's closer.
func OpenKubeconfigChannel(
	ctx context.Context,
	connectionConfig *sshconfig.ConnectionConfig,
	sett settings.Settings,
	kubeconfigPath, contextName string,
) (*rest.Config, func(), error) {
	content, err := readKubeconfigForChannel(kubeconfigPath)
	if err != nil {
		return nil, nil, err
	}

	host, port, err := kubeconfigServer(content, contextName)
	if err != nil {
		return nil, nil, err
	}

	address, stopTunnel, err := OpenBastionChannel(ctx, connectionConfig, sett, host, port, "Kubernetes API")
	if err != nil {
		return nil, nil, err
	}

	// The name stays the host the kubeconfig already named, not a node name: the
	// forward carries this one address, so the certificate it must match is the
	// one that address is served with.
	retargeted, err := RetargetKubeconfig(ctx, content, "https://"+address, host)
	if err != nil {
		stopTunnel()
		return nil, nil, err
	}

	restConfig, err := RESTConfigFromKubeconfig(retargeted, contextName)
	if err != nil {
		stopTunnel()
		return nil, nil, err
	}

	return restConfig, stopTunnel, nil
}

// readKubeconfigForChannel loads the operator's kubeconfig in the form the
// client is built from: file references made absolute, because a configuration
// built from bytes has no directory to resolve them against, and no proxy,
// because the local end of the forward is reached directly and http.ProxyURL
// exempts no address, loopback included.
func readKubeconfigForChannel(path string) ([]byte, error) {
	kubeconfig, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("read the kubeconfig %s: %w", path, err)
	}
	if err := clientcmd.ResolveLocalPaths(kubeconfig); err != nil {
		return nil, fmt.Errorf("resolve the file references of the kubeconfig %s: %w", path, err)
	}

	for _, cluster := range kubeconfig.Clusters {
		cluster.ProxyURL = ""
	}

	content, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("serialize the kubeconfig %s: %w", path, err)
	}

	return content, nil
}

// kubeconfigServer is the address the kubeconfig's current cluster is reached
// at. The port is explicit in every server URL dhctl writes, and 443 for one it
// did not write.
func kubeconfigServer(content []byte, contextName string) (string, int, error) {
	kubeconfig, err := clientcmd.Load(content)
	if err != nil {
		return "", 0, fmt.Errorf("parse the kubeconfig: %w", err)
	}

	if contextName == "" {
		contextName = kubeconfig.CurrentContext
	}
	kubeContext, ok := kubeconfig.Contexts[contextName]
	if !ok {
		return "", 0, fmt.Errorf("the kubeconfig has no context %q", contextName)
	}
	cluster, ok := kubeconfig.Clusters[kubeContext.Cluster]
	if !ok {
		return "", 0, fmt.Errorf("the kubeconfig has no cluster %q", kubeContext.Cluster)
	}

	parsed, err := url.Parse(cluster.Server)
	if err != nil {
		return "", 0, fmt.Errorf("parse the server URL %s: %w", cluster.Server, err)
	}
	if parsed.Hostname() == "" {
		return "", 0, fmt.Errorf("the server URL %s names no host", cluster.Server)
	}
	if parsed.Port() == "" {
		return parsed.Hostname(), defaultAPIServerPort, nil
	}

	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		return "", 0, fmt.Errorf("parse the port of the server URL %s: %w", cluster.Server, err)
	}
	return parsed.Hostname(), port, nil
}
