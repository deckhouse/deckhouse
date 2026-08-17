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

package registrydata

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/go_lib/registry/helpers"
	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
)

// mirrorThroughNode makes the cluster's own registry reachable from the machine dhctl runs on, by
// forwarding a local port to the store through the SSH connection that is already open.
//
// The problem it solves is that a cluster which manages its own registry has no address that means
// anything outside it. `deckhouse-registry` names `registry.d8-system.svc:5001`, which is a Service
// name; out of the cluster it does not resolve, and the fallback that reaches for it produced
//
//	dial tcp: lookup registry.d8-system.svc on 127.0.0.53:53: no such host
//
// on a cluster that was working perfectly. A reachable upstream is the usual way out of this and it is
// tried first, but an air-gapped cluster has none — by definition, and permanently.
//
// What makes the forward possible is that the store runs on the masters' host network, so it has an
// address that is dialable from any of them. dhctl is already connected to one — every call to the
// Kubernetes API goes through this very SSH connection — so no new access is needed, only a second
// channel on it.
//
// Three details decide whether this works, and all three are already true rather than arranged here:
//
//   - the far end is the STORE's own address, taken from the object that reports it, and not the
//     loopback port that a node's containerd pulls through. Measured on a master:
//     `10.110.0.13:5001` is the store, while `127.0.0.1:5001` is the node's own proxy — whose
//     authority is generated on the node and never published, so a tunnel to it fails verification
//     with "certificate signed by unknown authority" while looking like the right address;
//   - the store's certificate covers 127.0.0.1 (see the module's pki package, which adds "127.0.0.1"
//     and "localhost" to the serving certificate), so verification against the cluster's own CA
//     succeeds even though this end of the tunnel is a loopback address. The local port may be any
//     free one: a certificate says nothing about ports;
//   - the credentials are carried over already parsed, and not looked up again. A docker config is
//     keyed by registry host, so the entry for `registry.d8-system.svc:5001` does not match a rewritten
//     `127.0.0.1:<port>` address — re-resolving them would silently produce an anonymous pull and a 401
//     that reads like a permissions problem.
//
// The tunnel outlives the call that opened it and is reused by later ones, which is not an optimisation:
// bound to the caller's context it was torn down the moment that operation finished, and the next
// consumer of the same resolved registry — the lazy provider-plugin download, several steps later — got
// its connection accepted and then reset. Measured on a cluster: the cluster configuration was read and
// the resources deleted through one tunnel, and the infrastructure util then failed with
// "read: connection reset by peer" on a port that had already gone. dhctl is a short-lived process, so
// the listener lives until it exits.
//
// Returns ok=false when there is no SSH connection to work with (in-cluster callers, tests), leaving the
// caller's existing behaviour untouched.
func mirrorThroughNode(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	conf *image.RegistryConfig,
) (*image.RegistryConfig, string, bool, error) {
	if kubeCl == nil || conf == nil {
		return nil, "", false, nil
	}

	sshCl := kubeCl.SSHClient
	if sshCl == nil {
		// Some callers carry the connection as a node interface instead.
		if wrapper, ok := kubeCl.NodeInterface.(*ssh.NodeInterfaceWrapper); ok {
			sshCl = wrapper.Client()
		}
	}
	if sshCl == nil {
		return nil, "", false, nil
	}

	// Only the in-cluster mirror is worth rewriting. Any other address the cluster reports is one
	// somebody chose because it is reachable, and replacing it with a tunnel would be a downgrade.
	if !registry_const.IsInCluster(conf.GetRegistry()) {
		return nil, "", false, nil
	}

	// And only when the cluster keeps a store of its own, because that store is what is on the far end
	// of the tunnel. Without one there is nothing to forward to, and what the caller would get instead
	// of a DNS failure is a connection refused — the same dead end reached less clearly.
	store, held, err := readStoreAccess(ctx, kubeCl)
	if err != nil || !held {
		return nil, "", false, err
	}

	localPort, err := tunnelTo(ctx, sshCl, store.address)
	if err != nil {
		return nil, "", false, err
	}

	local := net.JoinHostPort("127.0.0.1", fmt.Sprint(localPort))

	rewritten, err := image.NewRegistryConfig(
		schemeHTTPS,
		replaceAddress(conf.GetRegistry(), local),
		store.username,
		store.password,
		store.ca,
	)
	if err != nil {
		return nil, "", false, fmt.Errorf("point the registry config at the tunnel: %w", err)
	}

	// And a docker config for the address the tunnel answers on, because a docker config is keyed by
	// registry host: the cluster's own is written for `registry.d8-system.svc:5001`, and a consumer
	// looking up `127.0.0.1:<port>` in it finds nothing. That is not a silent fallback to anonymous
	// either — measured on a cluster, the lazy provider-plugin download refuses outright with
	// "docker config doesn't contain 127.0.0.1:41285/system/deckhouse registry credentials".
	dockerCfg, err := helpers.DockerCfgFromCreds(store.username, store.password, local)
	if err != nil {
		return nil, "", false, fmt.Errorf("build a docker config for the tunnel: %w", err)
	}

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf(
		"Reaching the cluster store at %s through %s", conf.GetRegistry(), rewritten.GetRegistry()))

	return rewritten, base64.StdEncoding.EncodeToString(dockerCfg), true, nil
}

// tunnels remembers the forward opened to each store address, so that every consumer of the same
// resolved registry reaches it at the same port.
var tunnels = struct {
	sync.Mutex
	ports map[string]int
}{ports: map[string]int{}}

// tunnelTo opens a forward to the store, or returns the port of the one already open.
//
// The forward is deliberately not tied to the caller's context: see mirrorThroughNode. What it is tied
// to is the process, which for dhctl is one operation.
func tunnelTo(ctx context.Context, sshCl libcon.SSHClient, storeAddress string) (int, error) {
	tunnels.Lock()
	defer tunnels.Unlock()

	if port, already := tunnels.ports[storeAddress]; already {
		return port, nil
	}

	localPort, err := freeLocalPort()
	if err != nil {
		return 0, err
	}

	// "remote_bind:remote_port:local_bind:local_port": dhctl listens locally and dials the far end over
	// the SSH connection.
	address := fmt.Sprintf("%s:%d:127.0.0.1:%d", storeAddress, registry_const.Port, localPort)

	if err := sshCl.Tunnel(address).Up(context.WithoutCancel(ctx)); err != nil {
		return 0, fmt.Errorf("open a tunnel to the cluster store at %s: %w", address, err)
	}

	tunnels.ports[storeAddress] = localPort
	return localPort, nil
}

// storeSecret is what the store publishes for anything that needs to read from it: where it answers,
// the authority to verify it by, and an account to do it as.
//
// One secret rather than three sources, and it is the store's own: what `deckhouse-registry` carries is
// the BOOTSTRAP registry's authority (`CN = registry-ca`, generated by the installer), while the store
// serves with its own (`CN = registry-storage-ca`). Trusting the first while talking to the second is
// how a tunnel that reached exactly the right port still failed with "certificate signed by unknown
// authority" — measured on a cluster. The accounts differ too: `ro` there, `registry-ro` here.
const storeSecretName = "registry-storage-access"

// schemeHTTPS is what the store serves, always: it presents a certificate, which is the whole reason
// the authority above has to be right.
const schemeHTTPS = "HTTPS"

// storeAccess is how to reach the cluster's own store from outside it.
type storeAccess struct {
	// address is a replica's, without a port: the store answers on the same one everywhere.
	address string

	ca                 string
	username, password string
}

// readStoreAccess reads what the store publishes about itself.
//
// Reports held=false, without an error, for a cluster that keeps no store: an older registry module
// that has no such secret, or a cluster whose registry is somebody else's. Its caller then keeps the
// behaviour it had before, unchanged.
func readStoreAccess(ctx context.Context, kubeCl *client.KubernetesClient) (storeAccess, bool, error) {
	secret, err := kubeCl.CoreV1().
		Secrets(storeSecretNamespace).
		Get(ctx, storeSecretName, metav1.GetOptions{})

	switch {
	case err == nil:
	case apierrors.IsNotFound(err), meta.IsNoMatchError(err):
		return storeAccess{}, false, nil
	default:
		return storeAccess{}, false, fmt.Errorf("ask the cluster how to reach its store: %w", err)
	}

	access := storeAccess{
		address:  firstAddress(string(secret.Data["addresses"])),
		ca:       string(secret.Data["ca.crt"]),
		username: string(secret.Data["username"]),
		password: string(secret.Data["password"]),
	}

	// An address is the one part with no sensible default. The rest may legitimately be empty — a store
	// without authentication is a configuration, not a mistake — and an empty CA simply means the
	// system's own roots, which is what the verification will then say.
	if access.address == "" {
		return storeAccess{}, false, nil
	}

	return access, true, nil
}

// storeSecretNamespace is where the module keeps it.
const storeSecretNamespace = "d8-system"

// firstAddress takes one replica out of the list the store publishes.
//
// Any of them will do: every replica serves the same content, and this is a read. The list is separated
// by whatever the module writes — measured as a single address on a one-master cluster — so every
// plausible separator is accepted rather than one guessed at.
func firstAddress(addresses string) string {
	for _, candidate := range strings.FieldsFunc(addresses, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

// replaceAddress swaps the host:port of a repository, keeping the path it holds the images under.
func replaceAddress(repository, address string) string {
	if _, path, found := strings.Cut(repository, "/"); found {
		return address + "/" + path
	}
	return address
}

// freeLocalPort asks the kernel for a port nobody is using, the same way the telemetry relay does.
func freeLocalPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("find a free local port for the tunnel to the cluster store: %w", err)
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port, nil
}
