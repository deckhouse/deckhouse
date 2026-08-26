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
	"strings"

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
	logger.InfoContext(ctx, fmt.Sprintf("Admin kubeconfig: %s — cluster-admin, and the only way in.", kubeconfigPath),
		dhlog.ShowInCompacted())

	// With a bastion the node's address is reachable from the bastion and nowhere
	// else, so the plain export is true only inside that network.
	tunnel := bastionTunnelCommand(bastionConfig(b.SSHProviderInitializer.GetConfig()))
	if tunnel != "" {
		logger.InfoContext(ctx, fmt.Sprintf(
			"The master answers at %s:%d, an address that exists only inside the cluster network. "+
				"Tunnel to it through the bastion once, then use the cluster from any shell:",
			bctx.immutable.masterIP, immutable.APIServerPort), dhlog.ShowInCompacted())
		logger.InfoContext(ctx, "  "+tunnel, dhlog.ShowInCompacted())
	} else {
		logger.InfoContext(ctx, "To use the cluster:", dhlog.ShowInCompacted())
	}

	for _, line := range clusterUseCommands(kubeconfigPath, tunnel != "") {
		logger.InfoContext(ctx, "  "+line, dhlog.ShowInCompacted())
	}
	// ConnectionString rather than ShowInCompacted: the terminal UI pins it as a
	// milestone and repeats it in the closing summary, which is where an operator
	// looks for it after a long run. The tunnel is deliberately not in it: it is
	// opened once, while this is what gets typed again and again.
	logger.InfoContext(ctx, "  "+strings.Join(clusterUseCommands(kubeconfigPath, tunnel != ""), " && "), dhlog.ConnectionString())
}

// clusterUseCommands is what an operator runs to work with the cluster: the
// kubeconfig, the proxy that carries its address when the master sits behind a
// bastion, and a call that proves both.
func clusterUseCommands(kubeconfigPath string, throughBastion bool) []string {
	commands := []string{fmt.Sprintf("export KUBECONFIG=%s", kubeconfigPath)}
	if throughBastion {
		commands = append(commands, fmt.Sprintf("export HTTPS_PROXY=socks5://127.0.0.1:%d", socksPort))
	}
	return append(commands, "kubectl get nodes")
}

// socksPort is where the tunnel listens on the operator's own machine. Not 1080:
// that is the conventional SOCKS port and the one another proxy is likely to be
// holding. 18443 says what it carries — the API server behind it answers on 6443.
const socksPort = 18443

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
