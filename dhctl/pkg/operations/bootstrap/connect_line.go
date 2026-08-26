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
	// With a bastion the node's address is reachable from the bastion and nowhere
	// else, so the plain export is true only inside that network.
	tunnel := bastionTunnelCommand(bastionConfig(b.SSHProviderInitializer.GetConfig()))
	if tunnel == "" {
		logger.InfoContext(ctx, fmt.Sprintf("To use the cluster:  export KUBECONFIG=%s && kubectl get nodes", kubeconfigPath),
			dhlog.ShowInCompacted())
		return
	}

	// The tunnel is its own step on purpose: printed as one command with kubectl
	// it reads as something to repeat before every call, and an operator who
	// repeats it collects a background ssh per invocation.
	logger.InfoContext(ctx, "The master has no public address. Open a tunnel through the bastion — once, it stays up in the background:",
		dhlog.ShowInCompacted())
	// ConnectionString rather than ShowInCompacted: the terminal UI pins it as
	// a milestone and repeats it in the closing summary, which is where an
	// operator looks for it after a long run.
	logger.InfoContext(ctx, "  "+tunnel, dhlog.ConnectionString())
	logger.InfoContext(ctx, "Then, in any shell:", dhlog.ShowInCompacted())
	for _, line := range []string{
		fmt.Sprintf("export KUBECONFIG=%s", kubeconfigPath),
		fmt.Sprintf("export HTTPS_PROXY=socks5://127.0.0.1:%d", socksPort),
		"kubectl get nodes",
	} {
		logger.InfoContext(ctx, "  "+line, dhlog.ShowInCompacted())
	}
}

// socksPort is where the tunnel listens on the operator's own machine. 1080 is
// the conventional SOCKS port, and unlike 6443 it is not one a local cluster of
// their own would already be holding.
const socksPort = 1080

// bastionTunnelCommand builds the one command that makes the saved kubeconfig
// usable from outside, or "" when the master is directly reachable. A SOCKS
// proxy carries the kubeconfig's own address, so the file stays exactly as the
// node wrote it: a retargeted server outlives the tunnel it was written for.
func bastionTunnelCommand(cfg *sshconfig.Config) string {
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

	return fmt.Sprintf("ssh -f -N%s -D %d %s", port, socksPort, bastion)
}
