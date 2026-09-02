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
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

// defaultAPIServerPort is what an https URL without one means.
const defaultAPIServerPort = 443

// OpenKubeconfigChannel makes a kubeconfig whose server only exists inside the
// cluster network usable from outside: it forwards that exact address through
// the bastion and writes a copy pointed at the forward. Returns the copy's path
// and the closer of both the tunnel and the copy.
func OpenKubeconfigChannel(
	ctx context.Context,
	connectionConfig *sshconfig.ConnectionConfig,
	sett settings.Settings,
	kubeconfigPath, contextName, tmpDir string,
) (string, func(), error) {
	content, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return "", nil, fmt.Errorf("read the kubeconfig %s: %w", kubeconfigPath, err)
	}

	host, port, err := kubeconfigServer(content, contextName)
	if err != nil {
		return "", nil, err
	}

	address, stopTunnel, err := OpenBastionChannel(ctx, connectionConfig, sett, host, port, "Kubernetes API")
	if err != nil {
		return "", nil, err
	}

	// The name stays the host the kubeconfig already named, not a node name: the
	// forward carries this one address, so the certificate it must match is the
	// one that address is served with.
	retargeted, err := RetargetKubeconfig(ctx, content, "https://"+address, host)
	if err != nil {
		stopTunnel()
		return "", nil, err
	}

	path, err := WriteTemporaryKubeconfig(ctx, tmpDir, retargeted)
	if err != nil {
		stopTunnel()
		return "", nil, err
	}

	return path, func() {
		RemoveTemporaryKubeconfig(ctx, path)
		stopTunnel()
	}, nil
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

// WriteTemporaryKubeconfig stores a kubeconfig in a file the Kubernetes client
// can be built from. It holds cluster-admin credentials, so it is mode 0600 and
// removed again once dhctl exits.
func WriteTemporaryKubeconfig(ctx context.Context, dir string, content []byte) (string, error) {
	file, err := os.CreateTemp(dir, "dhctl-immutable-kubeconfig-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create a temporary kubeconfig: %w", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("create a temporary kubeconfig %s: %w", path, err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return "", fmt.Errorf("write the temporary kubeconfig %s: %w", path, err)
	}

	return path, nil
}

// RemoveTemporaryKubeconfig deletes the file WriteTemporaryKubeconfig made.
// Reported rather than returned: the caller is on its way out.
func RemoveTemporaryKubeconfig(ctx context.Context, path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("remove %s: %v", path, err))
	}
}
