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

package rpp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/proxy"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
	"github.com/deckhouse/lib-dhctl/pkg/log"
	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	tlsutils "github.com/deckhouse/deckhouse/dhctl/pkg/util/tls"
)

const (
	localhost = "127.0.0.1"
)

type RegistryPackagesProxy struct {
	signCheck     bool
	configGetter  registry.ClientConfigGetter
	clusterDomain string
	dc            *directoryconfig.DirectoryConfig

	localPort  string
	remotePort string

	loggerProvider log.LoggerProvider

	proxy  *proxy.Proxy
	tunnel node.ReverseTunnel
}

func NewRegistryPackagesProxy(clusterDomain string, configGetter registry.ClientConfigGetter, logger log.LoggerProvider) *RegistryPackagesProxy {
	return &RegistryPackagesProxy{
		clusterDomain:  clusterDomain,
		configGetter:   configGetter,
		localPort:      "5444",
		remotePort:     "5444",
		signCheck:      false,
		loggerProvider: logger,
	}
}

func (p *RegistryPackagesProxy) WithSignCheck(f bool) *RegistryPackagesProxy {
	p.signCheck = f
	return p
}

func (p *RegistryPackagesProxy) WithLocalPort(port string) *RegistryPackagesProxy {
	if port != "" {
		p.localPort = port
	}

	return p
}

func (p *RegistryPackagesProxy) WithRemotePort(port string) *RegistryPackagesProxy {
	if port != "" {
		p.remotePort = port
	}

	return p
}

func (p *RegistryPackagesProxy) WithDirectoryConfig(dc *directoryconfig.DirectoryConfig) *RegistryPackagesProxy {
	p.dc = dc

	return p
}

func (p *RegistryPackagesProxy) Start(ctx context.Context) error {
	if err := p.startProxy(); err != nil {
		p.Stop()
		return fmt.Errorf("Cannot start registry packages proxy: %w", err)
	}

	return nil
}

func (p *RegistryPackagesProxy) upTunnel(ctx context.Context, sshCl node.SSHClient) error {
	if govalue.IsNil(sshCl) {
		return upTunnelError(fmt.Errorf("internal error - ssh client is nil"))
	}

	if govalue.IsNil(p.dc) {
		return upTunnelError(fmt.Errorf("internal error - directory is nil"))
	}

	if govalue.IsNil(p.proxy) {
		return upTunnelError(fmt.Errorf("internal error - proxy is not started"))
	}

	if err := p.startTunnel(ctx, sshCl); err != nil {
		return upTunnelError(err)
	}

	return nil
}

func (p *RegistryPackagesProxy) Stop() {
	p.debug("Stopping registry packages proxy...")

	const (
		notInitMsg = "skip stop because not initialized"
		stoppedMsg = "stopped"
	)

	tunnelMessage := notInitMsg
	proxyMessage := notInitMsg

	if !govalue.IsNil(p.tunnel) {
		p.tunnel.Stop()
		p.tunnel = nil
		tunnelMessage = stoppedMsg
	}

	if !govalue.IsNil(p.proxy) {
		p.proxy.StopProxy()
		p.proxy = nil
		proxyMessage = stoppedMsg
	}

	p.debug("Registry packages proxy tunnel %s", tunnelMessage)
	p.debug("Registry packages proxy server %s", proxyMessage)
}

func (p *RegistryPackagesProxy) startProxy() error {
	p.debug("Starting registry packages proxy...")

	if govalue.IsNil(p.configGetter) {
		return fmt.Errorf("internal error: proxy configuration getter is nil")
	}

	p.debug("Cluster domain for registry packages proxy: %s\n", p.clusterDomain)

	const oneDay = 1

	cert, err := tlsutils.GenerateCertificate(
		"registry-packages-proxy",
		p.clusterDomain,
		tlsutils.CertKeyTypeRSA,
		oneDay,
	)

	if err != nil {
		return fmt.Errorf("failed to generate TLS certificate for registry proxy: %v", err)
	}

	addr := net.JoinHostPort(localhost, p.localPort)
	listener, err := tls.Listen("tcp", addr, &tls.Config{
		Certificates: []tls.Certificate{*cert},
	})

	if err != nil {
		return fmt.Errorf("failed to listen registry proxy socket: %v", err)
	}

	srv := &http.Server{}
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	proxyConfig := &proxy.Config{
		SignCheck: p.signCheck,
	}

	registryCl := &registry.DefaultClient{}
	proxyLogger := newLogger(p.loggerProvider())

	proxy := proxy.NewProxy(srv, listener, p.configGetter, proxyLogger, registryCl)

	go proxy.Serve(proxyConfig)

	p.proxy = proxy

	return nil
}

func (p *RegistryPackagesProxy) startTunnel(ctx context.Context, sshCl node.SSHClient) error {
	p.debug("Up registry packages proxy tunnel...")

	listenAddress := localhost

	preflightUrl := fmt.Sprintf("https://%s/healthz", net.JoinHostPort(listenAddress, p.remotePort))

	checkingScript, err := template.RenderAndSavePreflightReverseTunnelOpenScript(preflightUrl, p.dc)
	if err != nil {
		return fmt.Errorf("cannot render reverse tunnel checking script: %v", err)
	}

	killScript, err := template.RenderAndSaveKillReverseTunnelScript(listenAddress, p.remotePort, p.dc)
	if err != nil {
		return fmt.Errorf("cannot render kill reverse tunnel script: %v", err)
	}

	checker := ssh.NewRunScriptReverseTunnelChecker(sshCl, checkingScript)
	killer := ssh.NewRunScriptReverseTunnelKiller(sshCl, killScript)

	addr := fmt.Sprintf("%s:%s:%s:%s", listenAddress, p.localPort, listenAddress, p.remotePort)

	tun := sshCl.ReverseTunnel(addr)
	err = tun.Up()
	if err != nil {
		return fmt.Errorf("cannot up tunnel for registry packages proxy: %w", err)
	}

	tun.StartHealthMonitor(ctx, checker, killer)

	p.tunnel = tun

	return nil
}

func (p *RegistryPackagesProxy) debug(f string, args ...any) {
	p.loggerProvider().DebugF(f, args...)
}

func upTunnelError(err error) error {
	return fmt.Errorf("Cannot up registry packages proxy tunnel: %w", err)
}
