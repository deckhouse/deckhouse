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

	"github.com/name212/govalue"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh/utils"
	"github.com/deckhouse/lib-dhctl/pkg/log"

	rpp_log "github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/log"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/proxy"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
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
	clusterUUID   string
	dc            *directoryconfig.DirectoryConfig

	localPort           string
	remotePort          string
	bootstrapLocalPort  string
	bootstrapRemotePort string

	loggerProvider log.LoggerProvider
	interactive    bool

	proxy        *proxy.Proxy
	rppGetServer *proxy.RPPClientBinaryServer
	tunnels      []libcon.ReverseTunnel
}

const (
	registryPackagesProxyPort = "5444"
	rppGetBinaryPort          = "4282"
)

func NewRegistryPackagesProxy(clusterDomain string, configGetter registry.ClientConfigGetter, logger log.LoggerProvider, interactive bool) *RegistryPackagesProxy {
	return &RegistryPackagesProxy{
		clusterDomain:       clusterDomain,
		configGetter:        configGetter,
		localPort:           registryPackagesProxyPort,
		remotePort:          registryPackagesProxyPort,
		bootstrapLocalPort:  rppGetBinaryPort,
		bootstrapRemotePort: rppGetBinaryPort,
		signCheck:           false,
		loggerProvider:      logger,
		interactive:         interactive,
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

func (p *RegistryPackagesProxy) WithBootstrapLocalPort(port string) *RegistryPackagesProxy {
	if port != "" {
		p.bootstrapLocalPort = port
	}

	return p
}

func (p *RegistryPackagesProxy) WithBootstrapRemotePort(port string) *RegistryPackagesProxy {
	if port != "" {
		p.bootstrapRemotePort = port
	}

	return p
}

func (p *RegistryPackagesProxy) WithClusterUUID(clusterUUID string) *RegistryPackagesProxy {
	p.clusterUUID = clusterUUID

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

func (p *RegistryPackagesProxy) upTunnel(ctx context.Context, sshCl libcon.SSHClient) error {
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
	rppGetServerMessage := notInitMsg

	if len(p.tunnels) > 0 {
		for _, tunnel := range p.tunnels {
			tunnel.Stop()
		}
		p.tunnels = nil
		tunnelMessage = stoppedMsg
	}

	if !govalue.IsNil(p.proxy) {
		p.proxy.StopProxy()
		p.proxy = nil
		proxyMessage = stoppedMsg
	}

	if !govalue.IsNil(p.rppGetServer) {
		p.rppGetServer.Stop()
		p.rppGetServer = nil
		rppGetServerMessage = stoppedMsg
	}

	p.debug("Registry packages proxy tunnel %s", tunnelMessage)
	p.debug("Registry packages proxy server %s", proxyMessage)
	p.debug("rpp-get bootstrap server %s", rppGetServerMessage)
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

	bootstrapAddr := net.JoinHostPort(localhost, p.bootstrapLocalPort)
	bootstrapListener, err := net.Listen("tcp", bootstrapAddr)
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("failed to listen rpp-get socket: %v", err)
	}

	srv := &http.Server{}
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	proxyConfig := &proxy.Config{
		SignCheck: p.signCheck,
	}

	registryCl := &registry.DefaultClient{}
	var proxyLogger rpp_log.Logger

	if p.interactive {
		proxyLogger = newInteractiveLogger(p.loggerProvider())
	} else {
		proxyLogger = newLogger(p.loggerProvider())
	}

	packagesProxy := proxy.NewProxy(srv, listener, p.configGetter, proxyLogger, registryCl)
	rppGetServer := proxy.NewRPPClientBinaryServerFromRegistry(proxy.RPPClientBinaryServerOptions{
		Listener:           bootstrapListener,
		Logger:             proxyLogger,
		ClientConfigGetter: p.configGetter,
		RegistryClient:     registryCl,
		SignCheck:          proxyConfig.SignCheck,
		ClusterUUID:        p.clusterUUID,
	})

	go packagesProxy.Serve(proxyConfig)
	go rppGetServer.Serve()

	p.proxy = packagesProxy
	p.rppGetServer = rppGetServer

	return nil
}

func (p *RegistryPackagesProxy) startTunnel(ctx context.Context, sshCl libcon.SSHClient) error {
	p.debug("Up registry packages proxy tunnel...")

	tunnel, err := p.upSingleTunnel(ctx, sshCl, p.localPort, p.remotePort, true)
	if err != nil {
		return err
	}
	p.tunnels = append(p.tunnels, tunnel)

	bootstrapTunnel, err := p.upSingleTunnel(ctx, sshCl, p.bootstrapLocalPort, p.bootstrapRemotePort, false)
	if err != nil {
		return err
	}
	p.tunnels = append(p.tunnels, bootstrapTunnel)

	return nil
}

func (p *RegistryPackagesProxy) upSingleTunnel(ctx context.Context, sshCl libcon.SSHClient, localPort, remotePort string, healthCheck bool) (libcon.ReverseTunnel, error) {
	listenAddress := localhost
	addr := fmt.Sprintf("%s:%s:%s:%s", listenAddress, localPort, listenAddress, remotePort)

	tun := sshCl.ReverseTunnel(addr)
	if err := tun.Up(); err != nil {
		return nil, fmt.Errorf("cannot up tunnel for registry packages proxy: %w", err)
	}

	if !healthCheck {
		return tun, nil
	}

	preflightURL := fmt.Sprintf("https://%s/healthz", net.JoinHostPort(listenAddress, remotePort))
	checkingScript, err := template.RenderAndSavePreflightReverseTunnelOpenScript(preflightURL, p.dc)
	if err != nil {
		tun.Stop()
		return nil, fmt.Errorf("cannot render reverse tunnel checking script: %v", err)
	}

	killScript, err := template.RenderAndSaveKillReverseTunnelScript(listenAddress, remotePort, p.dc)
	if err != nil {
		tun.Stop()
		return nil, fmt.Errorf("cannot render kill reverse tunnel script: %v", err)
	}

	checker := utils.NewRunScriptReverseTunnelChecker(sshCl, checkingScript)
	killer := utils.NewRunScriptReverseTunnelKiller(sshCl, killScript)
	tun.StartHealthMonitor(ctx, checker, killer)

	return tun, nil
}

func (p *RegistryPackagesProxy) debug(f string, args ...any) {
	p.loggerProvider().DebugF(f, args...)
}

func upTunnelError(err error) error {
	return fmt.Errorf("Cannot up registry packages proxy tunnel: %w", err)
}
