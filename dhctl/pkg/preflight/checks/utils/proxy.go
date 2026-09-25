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

package utils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/http/httpproxy"

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

var ErrBadProxyConfig = errors.New("bad proxy config")

const ProxyTunnelPort = "22323"

// SetupSSHTunnelToProxyAddr opens a port-forward to proxyUrl. legacyMode
// must reflect the SSH backend the supplied client uses (legacy clissh
// vs modern gossh) — they need different forwarding-direction syntax.
// Pass sshclient.Config.IsLegacyMode() at the call site.
func SetupSSHTunnelToProxyAddr(ctx context.Context, sshCl libcon.SSHClient, proxyURL *url.URL, legacyMode bool) (libcon.Tunnel, error) {
	port := proxyURL.Port()
	if port == "" {
		switch proxyURL.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}

	var tunnel string
	if legacyMode {
		tunnel = strings.Join([]string{ProxyTunnelPort, proxyURL.Hostname(), port}, ":")
	} else {
		tunnel = strings.Join([]string{proxyURL.Hostname(), port, "127.0.0.1", ProxyTunnelPort}, ":")
	}

	tun := sshCl.Tunnel(tunnel)
	if err := tun.Up(ctx); err != nil {
		return nil, err
	}
	return tun, nil
}

// TLSOptions are the verification settings of the service behind the proxy — not of the proxy
// itself. They used to be dropped whenever a proxy was in play, so a private CA or an explicit
// "insecure" was honoured on the direct path and silently ignored on the proxied one.
type TLSOptions struct {
	// ServerName is the host the certificate has to be valid for. The tunnel makes every
	// request look like it is going to localhost, so without this the certificate is checked
	// against "localhost" and fails for reasons that have nothing to do with the cluster.
	ServerName string
	Insecure   bool
	CACert     string
}

// BuildHTTPClientWithLocalhostProxy builds the client that talks to the proxy through the local
// end of the SSH tunnel.
//
// The proxy URL is copied rather than rewritten: it is the caller's, shared with whatever else
// reads the proxy configuration, and pointing its Host at localhost in place corrupted it for
// every later reader.
func BuildHTTPClientWithLocalhostProxy(proxyURL *url.URL, tlsOpts TLSOptions) (*http.Client, error) {
	localhostProxy := *proxyURL
	localhostProxy.Host = net.JoinHostPort("localhost", ProxyTunnelPort)

	tlsConfig, err := BuildTLSConfig(tlsOpts)
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Transport: &http.Transport{
			Proxy:             http.ProxyURL(&localhostProxy),
			TLSClientConfig:   tlsConfig,
			DisableKeepAlives: true,
		},
	}, nil
}

// BuildTLSConfig turns the service's TLS options into a config. A CA that does not parse is an
// error rather than a silent fallback to the system pool: the run would then fail later against
// a certificate the operator believes they have trusted.
func BuildTLSConfig(opts TLSOptions) (*tls.Config, error) {
	cfg := &tls.Config{
		ServerName:         opts.ServerName,
		InsecureSkipVerify: opts.Insecure, //nolint:gosec // the operator asked for it in the provider configuration
	}

	if opts.CACert == "" {
		return cfg, nil
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(opts.CACert)) {
		return nil, fmt.Errorf("the configured CA certificate is not a valid PEM bundle")
	}
	cfg.RootCAs = pool
	return cfg, nil
}

// BuildHTTPClientThroughTunnel builds the client for the direct path: every connection is dialled
// to the local end of the SSH tunnel, whatever host the URL names.
func BuildHTTPClientThroughTunnel(tlsOpts TLSOptions) (*http.Client, error) {
	tlsConfig, err := BuildTLSConfig(tlsOpts)
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:   tlsConfig,
			DisableKeepAlives: true,
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				d := net.Dialer{Timeout: 20 * time.Second, KeepAlive: 20 * time.Second}
				return d.DialContext(ctx, network, net.JoinHostPort("127.0.0.1", ProxyTunnelPort))
			},
		},
	}, nil
}

func GetProxyFromMetaConfig(metaConfig *config.MetaConfig) (*url.URL, []string, error) {
	proxyConfig, err := metaConfig.EnrichProxyData()
	switch {
	case err != nil:
		return nil, nil, err
	case proxyConfig == nil:
		return nil, nil, nil
	}

	var proxyAddrClause any
	if proxyAddr, hasHTTPSProxy := proxyConfig["httpsProxy"]; hasHTTPSProxy {
		proxyAddrClause = proxyAddr
	} else if proxyAddr, hasHTTPProxy := proxyConfig["httpProxy"]; hasHTTPProxy {
		proxyAddrClause = proxyAddr
	} else {
		return nil, nil, fmt.Errorf("%w: no proxy address was given", ErrBadProxyConfig)
	}

	noProxyClause, hasNoProxy := proxyConfig["noProxy"]
	var noProxyAddresses []string
	if hasNoProxy {
		addrs, isStringSlice := noProxyClause.([]string)
		if !isStringSlice {
			return nil, nil, fmt.Errorf("%w: proxy.noProxy is not a set of addresses", ErrBadProxyConfig)
		}
		noProxyAddresses = addrs
	}

	proxyAddr, proxyAddrIsString := proxyAddrClause.(string)
	if !proxyAddrIsString {
		return nil, nil, fmt.Errorf(`%w: malformed proxy address: "%v"`, ErrBadProxyConfig, proxyAddr)
	}

	proxyURL, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, nil, fmt.Errorf(`%w: %w`, ErrBadProxyConfig, err)
	}

	return proxyURL, noProxyAddresses, nil
}

// ShouldSkipProxyCheck reports whether serviceURL is exempt from the proxy under noProxy.
//
// The rules are the ones every other tool in the cluster applies, because they are the ones the
// node will apply: httpproxy implements the NO_PROXY syntax in full — a leading dot or a bare
// domain matching subdomains, CIDR blocks, host:port entries, and "*" for everything. The
// previous implementation compared for exact string equality and resolved both sides to an IP,
// so ".example.com" and "10.0.0.0/8" — both documented, both common — matched nothing, and the
// check went through a proxy the node would have bypassed.
func ShouldSkipProxyCheck(serviceURL *url.URL, noProxyAddresses []string) bool {
	if serviceURL == nil || len(noProxyAddresses) == 0 {
		return false
	}

	cfg := &httpproxy.Config{
		// Both are set to the same placeholder: the question here is only whether noProxy
		// exempts this URL, and httpproxy answers it by returning no proxy for the URL.
		HTTPProxy:  "http://proxy.invalid",
		HTTPSProxy: "http://proxy.invalid",
		NoProxy:    strings.Join(noProxyAddresses, ","),
	}

	proxyFor := cfg.ProxyFunc()
	proxy, err := proxyFor(serviceURL)
	return err == nil && proxy == nil
}

// BuildHTTPClientFromEnvironment builds the client for a request dhctl makes itself, taking the
// proxy from its own environment.
//
// That is not a shortcut for ClusterConfiguration.proxy: those two are different proxies for
// different subjects. ClusterConfiguration.proxy configures the cluster's NODES. HTTP_PROXY and
// HTTPS_PROXY in dhctl's environment are what dhctl's own outgoing requests use — and what it
// hands the infrastructure utility verbatim (pkg/infrastructure/terraform/cmd.go, tofu/cmd.go),
// so a check built on this client takes the same path the utility will.
func BuildHTTPClientFromEnvironment(tlsOpts TLSOptions) (*http.Client, error) {
	tlsConfig, err := BuildTLSConfig(tlsOpts)
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Transport: &http.Transport{
			Proxy:             ProxyFromEnvironment,
			TLSClientConfig:   tlsConfig,
			DisableKeepAlives: true,
			DialContext:       (&net.Dialer{Timeout: 20 * time.Second, KeepAlive: 20 * time.Second}).DialContext,
		},
	}, nil
}

// StopAtFirstAnswer is a CheckRedirect that keeps the redirect the endpoint sent instead of
// following it. Set it on a client that asks whether an address answers, not what it serves.
//
// Two reasons, and either alone is enough. A 301 is an answer, so following it turns a reachable
// endpoint into a verdict about wherever it pointed. And these clients pin their TLS options to
// the endpoint — ServerName is its hostname, the CA is the one configured for it — so a redirect
// to another host is offered the wrong SNI and fails the handshake. That is what
// GET https://ec2.eu-central-1.amazonaws.com/ did: 301 to aws.amazon.com, then "tls: handshake
// failure", reported as an unreachable AWS API.
//
// It is deliberately not baked into the client builders: the registry checks share one of them,
// and a registry legitimately redirects.
func StopAtFirstAnswer(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

// ProxyFromEnvironment is http.ProxyFromEnvironment without its process-wide cache: the standard
// one reads HTTP_PROXY/HTTPS_PROXY/NO_PROXY exactly once, on its first call, which is fine for a
// CLI whose environment is fixed before main and wrong for anything that has to answer for the
// environment as it is now — a test among them. The resolution itself is the same code the
// standard library runs.
func ProxyFromEnvironment(req *http.Request) (*url.URL, error) {
	return httpproxy.FromEnvironment().ProxyFunc()(req.URL)
}
