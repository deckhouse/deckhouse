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

package nodeconfig

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"k8s.io/client-go/rest"
)

// Where the node keeps the two files that make up the kubelet's identity.
//
// The same paths on both kinds of node: on an Engine one /etc/kubernetes is a symlink
// into /run, and the kubelet's own rotating certificate lives under /var either way.
// Source of truth for the Engine spelling: clusterCAPath and clientCertPath in nodelet's
// kubelet controller.
//
// The authority is read from the node config where that document carries it, so
// DefaultClusterCAPath is only the fallback for a node whose config predates the field.
const (
	DefaultClusterCAPath         = "/etc/kubernetes/pki/ca.crt"
	DefaultClientCertificatePath = "/var/lib/kubelet/pki/kubelet-client-current.pem"
)

// ErrNoIdentityYet is a node that has not finished its TLS bootstrap. Expected, not
// exceptional: the agent has to be answering the container runtime before the node has
// joined anything, and it routes from its seed until this becomes available.
var ErrNoIdentityYet = errors.New("the node has no kubelet identity yet")

// Identity is the kubelet's own credentials, borrowed to read this node's layout.
//
// Borrowed rather than issued, because there is nothing to issue one with: the layout is
// per-node, so a credential distributed through the cluster would have to be per-node
// too, and the node already holds exactly such a credential that the API server already
// trusts. The bashible step reaches the same conclusion and writes a kubeconfig around
// the same certificate; this builds the equivalent in memory, which an Engine node needs
// because nothing there writes that file.
//
// Both halves come from one file: kubelet-client-current.pem holds the certificate and
// its key, and is the file the kubelet itself rotates — so the identity follows the
// rotation without the agent restarting or anything being copied.
type Identity struct {
	// ClusterCAPath verifies the API server. Empty means DefaultClusterCAPath.
	ClusterCAPath string

	// ClientCertificatePath is the kubelet's rotating client certificate. Empty means
	// DefaultClientCertificatePath.
	ClientCertificatePath string
}

func (i Identity) clusterCAPath() string {
	if i.ClusterCAPath == "" {
		return DefaultClusterCAPath
	}
	return i.ClusterCAPath
}

func (i Identity) clientCertificatePath() string {
	if i.ClientCertificatePath == "" {
		return DefaultClientCertificatePath
	}
	return i.ClientCertificatePath
}

// RestConfig builds API credentials from what the node already has.
//
// Returns ErrNoIdentityYet, wrapped, whenever a piece is missing, so the caller can tell
// "not yet" from "misconfigured" — the first is every node's first minutes and the second
// is worth saying out loud.
func (i Identity) RestConfig(document *Document) (*rest.Config, error) {
	if document == nil {
		return nil, fmt.Errorf("%w: this node has no config of its own", ErrNoIdentityYet)
	}
	if len(document.Spec.APIServerEndpoints) == 0 {
		return nil, fmt.Errorf("%w: its config names no API server", ErrNoIdentityYet)
	}

	if err := readable(i.clientCertificatePath()); err != nil {
		return nil, err
	}

	tlsConfig := rest.TLSClientConfig{
		// One file for both halves, which is what it holds and how the kubelet's own
		// kubeconfig names it. client-go reloads it as the kubelet rotates it.
		CertFile: i.clientCertificatePath(),
		KeyFile:  i.clientCertificatePath(),
	}

	// The authority out of the document first. It is the same certificate as the one on
	// disk, and taking it from here is what lets the agent run with no mount of
	// /etc/kubernetes/pki — a directory that on a master also holds that authority's
	// private key.
	switch authority, decodeErr := base64.StdEncoding.DecodeString(
		document.Spec.Kubelet.CACert); {
	case document.Spec.Kubelet.CACert == "":
		if err := readable(i.clusterCAPath()); err != nil {
			return nil, err
		}
		tlsConfig.CAFile = i.clusterCAPath()
	case decodeErr != nil:
		return nil, fmt.Errorf("the cluster authority in the node config is not base64: %w", decodeErr)
	default:
		tlsConfig.CAData = authority
	}

	// The first endpoint, and only the first. The others are the same API server reached
	// another way, and a client that failed over between them would hide exactly the
	// outage the agent is built to keep working through: it routes from its cache when
	// the API server cannot be reached, which is the right answer either way.
	return &rest.Config{
		Host:            apiServerURL(document.Spec.APIServerEndpoints[0]),
		TLSClientConfig: tlsConfig,
	}, nil
}

// apiServerURL is the endpoint as a URL, whichever of the two spellings the field
// carries.
//
// A config written by node-controller carries the full "https://host:port"; nodelet's own
// ParseEndpoint accepts a bare "host:port" too and prefixes the scheme itself, so both
// have to work here. Prefixing unconditionally produced "https://https/10.12.2.104:6443",
// which resolves the literal host "https" — the agent then reported the API server as
// unreachable and went on routing from its seed, looking healthy the whole time.
func apiServerURL(endpoint string) string {
	if strings.Contains(endpoint, "://") {
		return endpoint
	}
	return "https://" + endpoint
}

func readable(path string) error {
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%w: %s is not there", ErrNoIdentityYet, path)
	case err != nil:
		return fmt.Errorf("checking %s: %w", path, err)
	case info.Size() == 0:
		// The kubelet creates this empty and fills it in, so an empty file is the same
		// "not yet" as an absent one rather than a broken installation.
		return fmt.Errorf("%w: %s is empty", ErrNoIdentityYet, path)
	}
	return nil
}
