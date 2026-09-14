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

package checks

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"syscall"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

// hostLabel names the machine a check looked at, as user@host:port. Every on-node failure used to
// be phrased as though there were only one machine in the world; on a cluster with several the
// reader could not tell which one the message was about.
//
// The empty string means there is no SSH connection behind this interface: the check is running
// against the installer host itself.
func hostLabel(nodeInterface libcon.Interface) string {
	wrapper, ok := nodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok || wrapper == nil {
		return ""
	}
	client := wrapper.Client()
	if client == nil {
		return ""
	}
	sess := client.Session()
	if sess == nil {
		return ""
	}

	host := sess.Host()
	if host == "" {
		return ""
	}
	if port := sess.Port; port != "" {
		host = net.JoinHostPort(host, port)
	}
	if sess.User != "" {
		return sess.User + "@" + host
	}
	return host
}

// hostPhrase is hostLabel worded for the middle of a sentence, with a name for the case where
// there is no remote host at all.
func hostPhrase(nodeInterface libcon.Interface) string {
	if label := hostLabel(nodeInterface); label != "" {
		return label
	}
	return "the installer host"
}

// exitStatus returns the exit status of a command that ran on a node, from whichever backend ran
// it. Both are needed and only one used to be: the checks matched on *exec.ExitError, which the
// legacy clissh backend produces, while the default gossh backend returns x/crypto's
// *ssh.ExitError. On the default backend every branch that read an exit status was unreachable,
// so a script that failed with a clear message fell through to the generic wrapper and its
// output was dropped.
func exitStatus(err error) (int, bool) {
	var withExitStatus interface{ ExitStatus() int } // x/crypto/ssh
	if errors.As(err, &withExitStatus) {
		return withExitStatus.ExitStatus(), true
	}
	var withExitCode interface{ ExitCode() int } // os/exec
	if errors.As(err, &withExitCode) {
		return withExitCode.ExitCode(), true
	}
	return 0, false
}

// scriptFailure turns the result of a script that ran on a node into the error the reader sees.
// The order is what the reader needs rather than what the API returns: what the script said
// first, then the status it exited with, then — only when there is nothing else — the transport
// error that stopped it from running at all.
//
// what names the thing being checked, as a noun phrase that follows "cannot": "check sudo",
// "check the deckhouse user".
func scriptFailure(what string, nodeInterface libcon.Interface, out []byte, err error) error {
	host := hostPhrase(nodeInterface)

	if message := strings.TrimSpace(string(out)); message != "" {
		return fmt.Errorf("%s on %s", message, host)
	}
	if status, ok := exitStatus(err); ok {
		if stderr := strings.TrimSpace(stderrOf(err)); stderr != "" {
			return fmt.Errorf("script exited with status %d on %s: %s", status, host, stderr)
		}
		return fmt.Errorf("script exited with status %d on %s", status, host)
	}
	return fmt.Errorf("cannot %s on %s: %w", what, host, err)
}

// stderrOf returns the stderr an error carries. Only os/exec keeps it, and in a field rather
// than behind a method; x/crypto's *ssh.ExitError carries none, which is why a check that wants
// the script's diagnostics has to capture them itself.
func stderrOf(err error) string {
	var execErr *exec.ExitError
	if errors.As(err, &execErr) {
		return string(execErr.Stderr)
	}
	return ""
}

// hostLabelOfClient is hostLabel for a check that holds an SSH client rather than a node
// interface.
func hostLabelOfClient(client libcon.SSHClient) string {
	if client == nil {
		return "the master node"
	}
	sess := client.Session()
	if sess == nil || sess.Host() == "" {
		return "the master node"
	}
	host := sess.Host()
	if sess.Port != "" {
		host = net.JoinHostPort(host, sess.Port)
	}
	if sess.User != "" {
		return sess.User + "@" + host
	}
	return host
}

// classifyNetworkError turns a transport error into the sentence a reader can act on. The same
// four or five causes account for nearly every failure — a name that does not resolve, a port
// nothing listens on, a packet that goes nowhere, a certificate signed by a CA the client does
// not know — and each has a different fix. Reporting them all as one sentence, which is what
// "could not reach Cloud API from master node" did, throws that distinction away.
func classifyNetworkError(err error) string {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		if dnsErr.IsNotFound {
			return fmt.Sprintf("DNS lookup of %s failed: no such host", dnsErr.Name)
		}
		return fmt.Sprintf("DNS lookup of %s failed: %s", dnsErr.Name, dnsErr.Err)
	}

	var unknownAuthority x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthority) {
		return "TLS verification failed: certificate signed by unknown authority"
	}
	var hostnameErr x509.HostnameError
	if errors.As(err, &hostnameErr) {
		return fmt.Sprintf("TLS verification failed: the certificate is not valid for %s", hostnameErr.Host)
	}
	var invalidCert x509.CertificateInvalidError
	if errors.As(err, &invalidCert) {
		if invalidCert.Reason == x509.Expired {
			return "TLS verification failed: the certificate is expired or not yet valid (check the clock on both ends)"
		}
		return "TLS verification failed: " + invalidCert.Error()
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "the connection timed out"
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "the connection was refused: nothing is listening on that port"
	}
	if errors.Is(err, syscall.EHOSTUNREACH) || errors.Is(err, syscall.ENETUNREACH) {
		return "the host is unreachable: there is no route to it"
	}

	// A URL error wraps the operation and the address, both of which are already in `checked:`.
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return strings.TrimSpace(urlErr.Err.Error())
	}
	return strings.TrimSpace(err.Error())
}

// isCertificateError reports whether the failure is one no further attempt will change.
func isCertificateError(err error) bool {
	var unknownAuthority x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	var invalidCert x509.CertificateInvalidError
	return errors.As(err, &unknownAuthority) || errors.As(err, &hostnameErr) || errors.As(err, &invalidCert)
}

// NodeInterfaceFunc hands back the connection to the node, resolved at the moment the check runs
// rather than when its suite is built.
//
// The static suite is constructed in the pre-infra phase, and building the connection there
// opened SSH before the preflight header was even printed: a wrong credential surfaced as a raw
// lib-connection string ahead of everything, after a silent 50×2s retry loop, and the check whose
// job is to report exactly that ran afterwards on a connection that had already failed. Where
// there were no hosts at all, helper.GetNodeInterface handed back the installer container and
// every node check quietly inspected it instead.
type NodeInterfaceFunc func(ctx context.Context) (libcon.Interface, error)

// FixedNodeInterface is the resolver for a connection that is already in hand.
func FixedNodeInterface(nodeInterface libcon.Interface) NodeInterfaceFunc {
	return func(context.Context) (libcon.Interface, error) { return nodeInterface, nil }
}

// nodeCommandRunner is the part of libcon.Interface the on-node checks use.
type nodeCommandRunner = libcon.Interface

// nodeInterfaceResolverFor defers the connection to the moment a check runs. It is the same
// deferral the suites apply, available to a check that holds the initializer itself.
func nodeInterfaceResolverFor(initializer *providerinitializer.SSHProviderInitializer) NodeInterfaceFunc {
	return func(ctx context.Context) (libcon.Interface, error) {
		return helper.GetNodeInterface(ctx, initializer, initializer.GetSettings())
	}
}

// proxyConnectStatus recovers the status a proxy answered a CONNECT with, or 0 when err is not
// that.
//
// It has to be recovered from the text because Go does not surface it any other way: for an https
// URL the transport sends CONNECT itself, and a non-2xx answer to it never becomes an
// *http.Response the caller can inspect — it is returned as an error whose whole message is the
// status phrase. So the 407 and 403 branches that read resp.StatusCode only ever fired for a
// plain-http target, and the case they exist for — a proxy demanding credentials in front of an
// https registry — fell through to "the registry API did not answer", advising the reader to
// check a network path that is working.
func proxyConnectStatus(err error) int {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) || urlErr.Err == nil {
		return 0
	}

	// net/http returns exactly the reason phrase of the CONNECT response, so comparing against
	// the phrase table is an exact match rather than a guess at wording.
	text := urlErr.Err.Error()
	for status := http.StatusBadRequest; status < 600; status++ {
		if phrase := http.StatusText(status); phrase != "" && phrase == text {
			return status
		}
	}

	return 0
}

// proxyRefusal turns a proxy's refusal into a verdict. target is what the request was for, so the
// fix can name the host to allow.
func proxyRefusal(status int, proxyURL, target *url.URL) error {
	switch status {
	case http.StatusProxyAuthRequired:
		return preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("proxy %s", proxyURL.Redacted()),
			Observed: "HTTP 407: the proxy rejected the request without credentials",
			Expected: "the proxy to accept the credentials in ClusterConfiguration.proxy",
			Fix:      "put the user and password into ClusterConfiguration.proxy.httpsProxy (https://user:password@host:port)",
		})
	case http.StatusForbidden, http.StatusUnauthorized:
		return preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("%s via proxy %s", target, proxyURL.Redacted()),
			Observed: fmt.Sprintf("HTTP %d: the proxy refused to forward the request", status),
			Expected: "the proxy to allow requests to this address",
			Fix:      fmt.Sprintf("allow %s on the proxy, or add it to ClusterConfiguration.proxy.noProxy", target.Hostname()),
		})
	case 0:
		return nil
	default:
		return &preflight.Failure{
			Checked:  fmt.Sprintf("%s via proxy %s", target, proxyURL.Redacted()),
			Observed: fmt.Sprintf("HTTP %d from the proxy: the connection was not established", status),
			Expected: "the proxy to connect to the address",
			Fix: fmt.Sprintf("check that the proxy reaches %s, or add %s to ClusterConfiguration.proxy.noProxy",
				target.Host, target.Hostname()),
		}
	}
}
