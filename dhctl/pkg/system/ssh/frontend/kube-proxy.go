// Copyright 2021 Flant JSC
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

package frontend

import (
	"fmt"
	"regexp"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
)

const DefaultLocalAPIPort = 22322

type KubeProxy struct {
	Session *session.Session

	KubeProxyPort string
	LocalPort     string

	proxy  *Command
	tunnel *Tunnel

	stop      bool
	port      string
	localPort int
}

func NewKubeProxy(sess *session.Session) *KubeProxy {
	return &KubeProxy{
		Session:   sess,
		port:      "0",
		localPort: DefaultLocalAPIPort,
	}
}

func (k *KubeProxy) Start(useLocalPort int) (port string, err error) {
	success := false
	defer func() {
		k.stop = false
		if !success {
			k.Stop()
		}
	}()

	proxyCommandErrorCh := make(chan error, 1)
	proxy, port, err := k.runKubeProxy(proxyCommandErrorCh)
	if err != nil {
		return "", err
	}

	k.proxy = proxy
	k.port = port

	tunnelErrorCh := make(chan error)
	tun, localPort, lastError := k.upTunnel(port, useLocalPort, tunnelErrorCh)
	if lastError != nil {
		return "", fmt.Errorf("tunnel up error: max retries reached, last error: %v", lastError)
	}

	k.tunnel = tun
	k.localPort = localPort

	go k.healthMonitor(proxyCommandErrorCh, tunnelErrorCh)

	success = true

	return fmt.Sprintf("%d", k.localPort), nil
}

func (k *KubeProxy) Stop() {
	if k == nil {
		return
	}
	if k.proxy == nil {
		return
	}
	if k.stop {
		return
	}
	if k.proxy != nil {
		log.DebugF("Stop proxy command\n")
		k.proxy.Stop()
		log.DebugF("Proxy command stopped\n")
	}
	if k.tunnel != nil {
		log.DebugF("Stop tunnel\n")
		k.tunnel.Stop()
		log.DebugF("Tunnel stopped\n")
	}
	k.stop = true
}

func (k *KubeProxy) Restart() error {
	k.Stop()
	_, err := k.Start(k.localPort)
	if err == nil {
		k.stop = false
	}

	return err
}

func (k *KubeProxy) tryToRestartFully() {
	log.DebugF("Try restart kubeproxy fully\n")
	for {
		err := k.Restart()
		if err == nil {
			return
		}

		// need warn for human
		log.WarnF("Proxy was not started %v\n", err)
		k.Session.ChoiceNewHost()
		log.DebugF("New host choice %v\n", k.Session.Host())
	}
}

func (k *KubeProxy) proxyCMD() *Command {
	command := fmt.Sprintf("kubectl proxy --port=%s --kubeconfig /etc/kubernetes/admin.conf", k.port)
	cmd := NewCommand(k.Session, command).Sudo()
	cmd.Executor = cmd.Executor.CaptureStderr(nil).CaptureStdout(nil)
	return cmd
}

func (k *KubeProxy) healthMonitor(proxyErrorCh, tunnelErrorCh chan error) {
	defer log.DebugF("Kubeproxy health monitor stopped\n")
	log.DebugF("Kubeproxy health monitor started\n")

	for {
		log.DebugF("Kubeproxy Monitor step\n")
		select {
		case err := <-proxyErrorCh:
			log.DebugF("Proxy failed %v\n", err)
			// if proxy crushed, we need to restart kube-proxy fully
			// with proxy and tunnel (tunnel depends on proxy)
			k.tryToRestartFully()
			// if we restart proxy fully
			// this monitor must be finished because new monitor was started
			return

		case err := <-tunnelErrorCh:
			log.DebugF("Tunnel failed %v\n Try to up tunnel\n", err)
			// we need fully stop tunnel because
			k.tunnel.Stop()
			k.tunnel, _, err = k.upTunnel(k.port, k.localPort, tunnelErrorCh)
			if err != nil {
				k.tryToRestartFully()
				return
			}

			log.DebugF("Tunnel re up successfully\n")
		}
	}
}

func (k *KubeProxy) upTunnel(kubeProxyPort string, useLocalPort int, tunnelErrorCh chan error) (tun *Tunnel, localPort int, err error) {
	rewriteLocalPort := false
	localPort = useLocalPort

	if useLocalPort < 1 {
		localPort = DefaultLocalAPIPort
		rewriteLocalPort = true
	}

	maxRetries := 5
	retries := 0
	var lastError error
	for {
		if k.proxy.WaitError() != nil {
			lastError = fmt.Errorf("proxy was failed while restart tunnel")
			break
		}

		// try to start tunnel from localPort to proxy port
		tunnelAddress := fmt.Sprintf("%d:localhost:%s", localPort, kubeProxyPort)
		log.DebugF("Try up tunnel on %v\n", tunnelAddress)
		tun = NewTunnel(k.Session, "L", tunnelAddress)
		err := tun.Up()
		if err != nil {
			tun.Stop()
			lastError = fmt.Errorf("tunnel '%s': %v", tunnelAddress, err)
			if rewriteLocalPort {
				localPort++
			}

			retries++
			if retries >= maxRetries {
				tun = nil
				break
			}
		} else {
			go tun.HealthMonitor(tunnelErrorCh)
			lastError = nil
			break
		}
	}

	dbgMsg := "Tunnel up\n"
	if lastError != nil {
		dbgMsg = fmt.Sprintf("Tunnel was not up: %v\n", lastError)
	}
	log.DebugF(dbgMsg)

	return tun, localPort, lastError
}

func (k *KubeProxy) runKubeProxy(waitCh chan error) (proxy *Command, port string, err error) {
	proxy = k.proxyCMD()

	port = ""
	portReady := make(chan struct{}, 1)
	portRe := regexp.MustCompile(`Starting to serve on .*?:(\d+)`)

	proxy.WithStdoutHandler(func(line string) {
		m := portRe.FindStringSubmatch(line)
		if len(m) == 2 && m[1] != "" {
			port = m[1]
			log.InfoF("Got proxy port = %s on host %s\n", port, k.Session.Host())
			portReady <- struct{}{}
		}
	})

	onStart := make(chan struct{}, 1)
	proxy.OnCommandStart(func() {
		onStart <- struct{}{}
	})

	proxy.WithWaitHandler(func(err error) {
		waitCh <- err
	})

	err = proxy.Start()
	if err != nil {
		return nil, "", fmt.Errorf("start kubectl proxy: %v", err)
	}

	returnWaitErr := func(err error) error {
		template := `Proxy exited suddenly:
%s%sStatus: %v`
		return fmt.Errorf(template, string(proxy.StdoutBytes()), string(proxy.StderrBytes()), err)
	}

	// we need to check that kubeproxy was started
	// that checking wait string pattern in output
	// but we may receive error and this error will get from waitCh
	select {
	case <-onStart:
	case err := <-waitCh:
		return nil, "", returnWaitErr(err)
	}

	// Wait for proxy startup
	t := time.NewTicker(20 * time.Second)
	defer t.Stop()
	select {
	case e := <-waitCh:
		return nil, "", returnWaitErr(e)
	case <-t.C:
		return nil, "", fmt.Errorf("timeout waiting for api proxy port")
	case <-portReady:
		if port == "" {
			return nil, "", fmt.Errorf("got empty port from kubectl proxy")
		}
	}

	log.DebugLn("Proxy process started\n")
	return proxy, port, nil
}
