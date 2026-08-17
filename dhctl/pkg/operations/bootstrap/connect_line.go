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

package bootstrap

import (
	"context"
	"fmt"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
)

// printHowToReachTheCluster says where the credentials are — the equivalent of
// the SSH line the classic bootstrap prints. Called twice because two thousand
// log lines separate the points, and one print gets lost between them.
func (b *ClusterBootstrapper) printHowToReachTheCluster(ctx context.Context, kubeconfigPath string, bctx *bootstrapContext) {
	logger := dhlog.FromContext(ctx)

	// Tagged for the compact view: an untagged Info record is file-only on a
	// terminal. The bashible path tags its SSH line the same way (steps_ssh.go).
	logger.InfoContext(ctx, fmt.Sprintf("Admin kubeconfig written to %s — cluster-admin credentials, "+
		"and on a cluster of immutable nodes the only way in.", kubeconfigPath), dhlog.ShowInCompacted())
	logger.InfoContext(ctx, fmt.Sprintf("To use the cluster:  export KUBECONFIG=%s && kubectl get nodes", kubeconfigPath),
		dhlog.ShowInCompacted())

	// With a bastion that address is reachable from the bastion and nowhere else,
	// so the line above is true only inside the network. Print how to get there
	// rather than leave the operator to guess the shape of the tunnel.
	if line := bastionForwardLine(bastionConfig(b.SSHProviderInitializer.GetConfig()), bctx.immutable.masterIP, kubeconfigPath); line != "" {
		logger.InfoContext(ctx, "The master has no public address; reach it through the bastion first:",
			dhlog.ShowInCompacted())
		// ConnectionString rather than ShowInCompacted: the terminal UI pins it as
		// a milestone and repeats it in the closing summary, which is where an
		// operator looks for it after a long run.
		logger.InfoContext(ctx, "  "+line, dhlog.ConnectionString())
	}
}

// bastionForwardLine builds the commands that make the saved kubeconfig usable
// from outside, or "" when the master is directly reachable. It forwards to
// 127.0.0.1, which every apiserver certificate covers — no --tls-server-name needed.
// The server is retargeted with kubectl rather than sed: kubectl edits the field
// by name, is idempotent, and fails loudly where a regex silently misses.
func bastionForwardLine(cfg *sshconfig.Config, masterIP, kubeconfigPath string) string {
	if cfg == nil {
		return ""
	}

	bastion := cfg.BastionHost
	if cfg.BastionUser != "" {
		bastion = cfg.BastionUser + "@" + bastion
	}
	port := ""
	if cfg.BastionPort != nil && *cfg.BastionPort != 0 {
		port = fmt.Sprintf(" -p %d", *cfg.BastionPort)
	}

	// 6445 rather than 6443: the port is opened on the operator's own machine,
	// which may well be running a cluster of its own.
	const localPort = 6445
	// The cluster is always named "kubernetes": the node generates this
	// kubeconfig itself, kubeadm-style, and nothing downstream renames it.
	return fmt.Sprintf("ssh -f -N%s -L %d:%s:%d %s  &&  kubectl --kubeconfig %s config set-cluster kubernetes --server=https://127.0.0.1:%d",
		port, localPort, masterIP, immutable.APIServerPort, bastion,
		kubeconfigPath, localPort)
}
