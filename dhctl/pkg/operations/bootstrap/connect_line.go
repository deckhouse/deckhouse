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
	"os"
	"path/filepath"
	"sort"
	"strconv"
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

	tunnel := bastionTunnelCommand(immutable.BastionConfig(b.SSHProviderInitializer.GetConfig()))
	if tunnel != "" {
		logger.InfoContext(ctx, fmt.Sprintf(
			"The master answers at %s:%d, an address that exists only inside the cluster network. "+
				"Tunnel to it through the bastion once, then use the cluster from any shell:",
			bctx.immutable.masterIP, immutable.APIServerPort), dhlog.ShowInCompacted())
	} else {
		logger.InfoContext(ctx, "To use the cluster:", dhlog.ShowInCompacted())
	}

	// Banner, not ConnectionString: the terminal pins a connection string as a
	// single line, and this is four — the operator needs all of them, and needs
	// them where they do not scroll away behind minutes of module logs.
	lines := reachTheClusterLines(kubeconfigPath, tunnel)
	logger.InfoContext(ctx, strings.Join(lines, "\n"), dhlog.Banner())
	// And once as a single line: the banner lives on the live canvas, which the
	// closing summary does not have. Joined with && so that the summary's copy is
	// runnable too — including the tunnel, without which the rest reaches nothing.
	logger.InfoContext(ctx, strings.Join(runnableLines(lines), " && "), dhlog.ConnectionString())
}

// runnableLines drops what is prose rather than command: the note about the
// container path explains the line above it and would not survive a paste.
func runnableLines(lines []string) []string {
	runnable := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "(") {
			continue
		}
		runnable = append(runnable, line)
	}
	return runnable
}

// reachTheClusterLines is the block pinned at the top of the screen: the tunnel
// (once), the two exports and the call that proves them. Where dhctl runs inside
// its own container the path is one only that container can see, and the last
// line says so — the commands are copied onto the host, where nothing answers at
// that path and kubectl falls back to localhost:8080.
func reachTheClusterLines(kubeconfigPath, tunnel string) []string {
	lines := make([]string, 0, 5)
	if tunnel != "" {
		lines = append(lines, tunnel)
	}
	lines = append(lines, clusterUseCommands(kubeconfigPath, tunnel != "")...)
	if runningInContainer() {
		lines = append(lines, fmt.Sprintf(
			"(%s is a path inside the installer container: on your own machine it is in whatever directory you mounted at %s)",
			kubeconfigPath, filepath.Dir(kubeconfigPath)))
	}
	return lines
}

// runningInContainer reports the one case where the kubeconfig path printed above
// is true here and false everywhere else. /.dockerenv is written by the runtime
// that starts the installer image, and its absence is the native run.
func runningInContainer() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
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

// printHowToCleanUp says how to remove what a failed bootstrap left behind.
//
// By the time a bootstrap fails it has usually created cloud infrastructure that costs money and
// that the next run will not adopt, so removing it is the first thing wanted. Removing it needs the
// master's address - and that address was discovered by this run, not supplied by the operator, so
// it exists only somewhere thousands of log lines above the failure. Printing it again here, next
// to the failure, is the difference between a command to paste and a search through the scrollback.
//
// Nothing is printed when no master was ever reachable over SSH: the run failed before one existed,
// or this is the immutable path, whose master answers no sshd and whose credentials
// printCollectedKubeconfig already reports.
func (b *ClusterBootstrapper) printHowToCleanUp(ctx context.Context, bctx *bootstrapContext) {
	host := firstMasterAddressForSSH(bctx)
	if host == "" {
		return
	}

	logger := dhlog.FromContext(ctx)
	logger.InfoContext(ctx, "The cluster was not bootstrapped. To remove what this run created:", dhlog.ShowInCompacted())

	// ConnectionString, like the success path uses for the kubectl line: the terminal keeps it
	// through the closing summary, which is the one place still on screen once the live block is
	// gone. The two never collide - a run that reaches this has no kubeconfig line to displace.
	var cfg *sshconfig.Config
	if cc := b.SSHProviderInitializer.GetConfig(); cc != nil {
		cfg = cc.Config
	}
	logger.InfoContext(ctx, destroyCommand(host, cfg), dhlog.ConnectionString())
}

// firstMasterAddressForSSH returns the address of the master this run created, or "" when there is
// none. Sorted by node name so a multi-master bootstrap names the same one every time rather than
// whichever the map happened to yield.
func firstMasterAddressForSSH(bctx *bootstrapContext) string {
	if bctx == nil || len(bctx.masterAddressesForSSH) == 0 {
		return ""
	}

	names := make([]string, 0, len(bctx.masterAddressesForSSH))
	for name := range bctx.masterAddressesForSSH {
		names = append(names, name)
	}
	sort.Strings(names)

	return bctx.masterAddressesForSSH[names[0]]
}

// destroyCommand renders the destroy invocation for host, carrying over the connection settings
// this run used so the line is runnable as printed rather than a template to fill in.
//
// Only addressing is carried over. Passwords held in the same config (sudo, bastion) are never
// rendered: this line goes to the terminal, the debug log and the commander stream alike, and a
// password in any of them outlives the cluster it belonged to. dhctl asks for them again.
func destroyCommand(host string, cfg *sshconfig.Config) string {
	args := []string{"dhctl", "destroy", "--ssh-host", host}

	if cfg != nil {
		if cfg.User != "" {
			args = append(args, "--ssh-user", cfg.User)
		}
		if cfg.Port != nil && *cfg.Port != 0 && *cfg.Port != sshconfig.DefaultPort {
			args = append(args, "--ssh-port", strconv.Itoa(*cfg.Port))
		}
		if cfg.BastionHost != "" {
			args = append(args, "--ssh-bastion-host", cfg.BastionHost)
			if cfg.BastionUser != "" {
				args = append(args, "--ssh-bastion-user", cfg.BastionUser)
			}
			if cfg.BastionPort != nil && *cfg.BastionPort != 0 && *cfg.BastionPort != sshconfig.DefaultPort {
				args = append(args, "--ssh-bastion-port", strconv.Itoa(*cfg.BastionPort))
			}
		}
		for _, key := range cfg.PrivateKeys {
			// Only paths. A key supplied inline is the private key itself.
			if key.IsPath && key.Key != "" {
				args = append(args, "--ssh-agent-private-keys", key.Key)
			}
		}
	}

	return strings.Join(args, " ")
}
