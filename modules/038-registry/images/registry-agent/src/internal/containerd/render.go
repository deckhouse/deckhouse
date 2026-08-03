/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package containerd owns the registry drop-in directory of the container runtime.
//
// The runtime is pointed at the agent once, for every registry, through the
// `_default` fallback directory. Routing — cache or upstream, which upstream, with
// which credentials — is then entirely the agent's, decided per request from the
// original registry name the runtime passes along.
//
// That is what makes the node configuration static. Adding or removing an additional
// upstream changes nothing on any node, because the runtime already looks only at
// the agent. A directory per configured registry would instead mean rewriting node
// configuration on every change to a custom resource, and would put registry
// credentials into the runtime's own configuration files.
//
// Two long-standing containerd mechanisms make it work:
//
//   - `_default` is consulted for any registry without a directory of its own
//     (remotes/docker/config/config_unix.go, documented in docs/hosts.md). A directory
//     an administrator created by hand still takes precedence, so hand-written
//     configuration is not hijacked.
//   - the original registry is passed as the `ns` query parameter whenever the
//     endpoint differs from the requested registry (remotes/docker/resolver.go), which
//     is always the case here.
package containerd

import (
	"fmt"
	"path/filepath"
	"strings"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

const (
	// DefaultRoot is the drop-in directory containerd is pointed at through
	// `config_path`.
	//
	// Taken from the shared constants rather than spelled out here: a bashible step
	// waits for the rendered file as the sign that pulls on the node can succeed, so
	// this path is a contract with something outside this binary.
	DefaultRoot = constant.DropInRoot

	// DefaultHost is the fallback directory containerd consults for any registry
	// that has no directory of its own.
	DefaultHost = constant.DropInDefaultHost

	// StateFile lists the host directories the Deckhouse-managed configuration
	// created.
	//
	// A contract shared with the bashible step of the previous implementation, which
	// writes the same file. That is deliberate: it is how the agent learns which
	// per-registry directories its predecessor left behind, and those have to go — an
	// explicit directory takes precedence over `_default` and would route pulls
	// around the agent entirely.
	StateFile = "deckhouse_hosts_state.json"

	// ProxyCAFile holds the certificate authority of the agent's serving
	// certificate, written next to the configuration that names it.
	ProxyCAFile = "proxy-ca.crt"

	hostsFile = constant.DropInHostsFile
)

// Desired is the content of the drop-in directory: a file set keyed by path
// relative to the root, plus the host directories it covers.
//
// Produced as data rather than written directly, so the rendering is testable
// without a filesystem and the writer can compare it against what is on disk.
type Desired struct {
	Files map[string][]byte
	Hosts []string
}

// Options are the node-side parts of the rendering.
type Options struct {
	// Root is the drop-in directory. An input rather than a package variable, so the
	// rendering reaches into no global state.
	Root string

	// ProxyEndpoint is where the agent listens, as "host:port".
	ProxyEndpoint string

	// ProxyCA is the certificate authority of the agent's serving certificate.
	// Generated on the node at bootstrap and never leaving it: the only party that
	// has to trust it is the runtime on this very node.
	ProxyCA string
}

// Render produces the drop-in content for a node layout.
//
// The result depends on neither the additional routes, nor which backends are
// currently usable, nor any credential: all of that is resolved by the agent at
// request time. What the runtime is told is only "ask the agent", so this output
// changes about as rarely as the agent's own certificate.
func Render(spec *registryv1alpha1.RegistryNodeSpec, opts Options) (Desired, error) {
	if spec == nil {
		return Desired{}, fmt.Errorf("no node layout to render")
	}
	if opts.ProxyEndpoint == "" {
		return Desired{}, fmt.Errorf("no proxy endpoint")
	}
	if opts.ProxyCA == "" {
		// Without it the runtime cannot verify the agent and every pull on the node
		// fails at the handshake. Keeping the previous configuration is better than
		// writing one that cannot work.
		return Desired{}, fmt.Errorf("no proxy certificate authority")
	}
	if opts.Root == "" {
		opts.Root = DefaultRoot
	}

	if len(spec.Backends) == 0 && len(spec.AdditionalRoutes) == 0 {
		// Nothing to route: an Unmanaged node, or a layout not compiled yet. The
		// runtime keeps whatever it had, and the writer removes what the
		// Deckhouse-managed configuration previously owned.
		return Desired{Files: map[string][]byte{}}, nil
	}

	caPath := filepath.Join(opts.Root, DefaultHost, ProxyCAFile)

	return Desired{
		Hosts: []string{DefaultHost},
		Files: map[string][]byte{
			filepath.Join(DefaultHost, ProxyCAFile): []byte(opts.ProxyCA),
			filepath.Join(DefaultHost, hostsFile):   renderHosts(opts.ProxyEndpoint, caPath),
		},
	}, nil
}

// renderHosts writes the fallback configuration.
//
// Written by hand rather than through a TOML library so the output is byte-stable:
// the writer only touches what changed, and a library's map ordering is not
// something to rely on for that.
func renderHosts(endpoint, caPath string) []byte {
	var out strings.Builder

	out.WriteString("# Managed by the Deckhouse registry agent. Do not edit.\n")
	out.WriteString("#\n")
	out.WriteString("# Every registry without a directory of its own is served by the agent, which\n")
	out.WriteString("# decides per request whether to use the in-cluster cache or an upstream. The\n")
	out.WriteString("# original registry reaches the agent in the `ns` query parameter.\n")
	out.WriteString("[host]\n")
	fmt.Fprintf(&out, "\n  [host.%q]\n", fmt.Sprintf("https://%s", endpoint))
	// Pull and resolve only. An endpoint that could be pushed to would let anything
	// on the node write into the registry the whole cluster reads.
	out.WriteString("    capabilities = [\"pull\", \"resolve\"]\n")
	fmt.Fprintf(&out, "    ca = [%q]\n", caPath)

	return []byte(out.String())
}
