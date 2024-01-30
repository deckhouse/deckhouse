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
	"math/rand"
	"os"
	"regexp"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
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

	healthMonitorsByStartID map[int]chan struct{}
}

func NewKubeProxy(sess *session.Session) *KubeProxy {
	return &KubeProxy{
		Session:                 sess,
		port:                    "0",
		localPort:               DefaultLocalAPIPort,
		healthMonitorsByStartID: make(map[int]chan struct{}),
	}
}

func (k *KubeProxy) Start(useLocalPort int) (port string, err error) {
	startID := rand.Int()

	log.DebugF("Kube-proxy start id=[%d]; port:%d\n", startID, useLocalPort)

	success := false
	defer func() {
		k.stop = false
		if !success {
			log.DebugF("[%d] Kube-proxy was not started. Try to clear all\n", startID)
			k.Stop(startID)
		}
		log.DebugF("[%d] Kube-proxy starting was finished\n", startID)
	}()

	proxyCommandErrorCh := make(chan error, 1)
	proxy, port, err := k.runKubeProxy(proxyCommandErrorCh, startID)
	if err != nil {
		log.DebugF("[%d] Got error from runKubeProxy func: %v\n", startID, err)
		return "", err
	}

	log.DebugF("[%d] Proxy was started successfully\n", startID)

	k.proxy = proxy
	k.port = port

	tunnelErrorCh := make(chan error)
	tun, localPort, lastError := k.upTunnel(port, useLocalPort, tunnelErrorCh, startID)
	if lastError != nil {
		log.DebugF("[%d] Got error from upTunnel func: %v\n", startID, err)
		return "", fmt.Errorf("tunnel up error: max retries reached, last error: %v", lastError)
	}

	k.tunnel = tun
	k.localPort = localPort

	k.healthMonitorsByStartID[startID] = make(chan struct{}, 1)
	go k.healthMonitor(proxyCommandErrorCh, tunnelErrorCh, k.healthMonitorsByStartID[startID], startID)

	success = true

	return fmt.Sprintf("%d", k.localPort), nil
}

func (k *KubeProxy) StopAll() {
	for startID := range k.healthMonitorsByStartID {
		k.Stop(startID)
	}
}

func (k *KubeProxy) Stop(startID int) {
	if k == nil {
		log.DebugF("[%d] Stop kube-proxy: kube proxy object is nil. Skip.\n", startID)
		return
	}

	if k.stop {
		log.DebugF("[%d] Stop kube-proxy: kube proxy already stopped. Skip.\n", startID)
		return
	}

	if k.healthMonitorsByStartID[startID] != nil {
		k.healthMonitorsByStartID[startID] <- struct{}{}
		delete(k.healthMonitorsByStartID, startID)
	}
	if k.proxy != nil {
		log.DebugF("[%d] Stop proxy command\n", startID)
		k.proxy.Stop()
		log.DebugF("[%d] Proxy command stopped\n", startID)
		k.proxy = nil
	}
	if k.tunnel != nil {
		log.DebugF("[%d] Stop tunnel\n", startID)
		k.tunnel.Stop()
		log.DebugF("[%d] Tunnel stopped\n", startID)
		k.tunnel = nil
	}
	k.stop = true
}

func (k *KubeProxy) tryToRestartFully(startID int) {
	log.DebugF("[%d] Try restart kubeproxy fully\n", startID)
	for {
		k.Stop(startID)

		_, err := k.Start(k.localPort)

		if err == nil {
			k.stop = false
			log.DebugF("[%d] Proxy was restarted successfully\n", startID)
			return
		}

		const sleepTimeout = 5

		// need warn for human
		log.WarnF("Proxy was not restarted: %v. Sleep %d seconds before next attempt.\n", err, sleepTimeout)
		time.Sleep(sleepTimeout * time.Second)

		k.Session.ChoiceNewHost()
		log.DebugF("[%d] New host selected %v\n", startID, k.Session.Host())
	}
}

func (k *KubeProxy) proxyCMD(startID int) *Command {
	kubectlProxy := fmt.Sprintf("kubectl proxy --port=%s --kubeconfig /etc/kubernetes/admin.conf", k.port)
	if v := os.Getenv("KUBE_PROXY_ACCEPT_HOSTS"); v != "" {
		kubectlProxy += fmt.Sprintf(" --accept-hosts='%s'", v)
	}
	command := fmt.Sprintf("PATH=$PATH:%s/; %s", app.DeckhouseNodeBinPath, kubectlProxy)

	log.DebugF("[%d] Proxy command for start: %s\n", startID, command)

	cmd := NewCommand(k.Session, command).Sudo()
	cmd.Executor = cmd.Executor.CaptureStderr(nil).CaptureStdout(nil)
	return cmd
}

func (k *KubeProxy) healthMonitor(proxyErrorCh, tunnelErrorCh chan error, stopCh chan struct{}, startID int) {
	defer log.DebugF("[%d] Kubeproxy health monitor stopped\n", startID)
	log.DebugF("[%d] Kubeproxy health monitor started\n", startID)

	for {
		log.DebugF("[%d] Kubeproxy Monitor step\n", startID)
		select {
		case err := <-proxyErrorCh:
			log.DebugF("[%d] Proxy failed with error %v\n", startID, err)
			// if proxy crushed, we need to restart kube-proxy fully
			// with proxy and tunnel (tunnel depends on proxy)
			k.tryToRestartFully(startID)
			// if we restart proxy fully
			// this monitor must be finished because new monitor was started
			return

		case err := <-tunnelErrorCh:
			log.DebugF("[%d] Tunnel failed %v. Stopping previous tunnel\n", startID, err)
			// we need fully stop tunnel because
			k.tunnel.Stop()

			log.DebugF("[%d] Tunnel stopped before restart. Starting new tunnel...\n", startID)

			k.tunnel, _, err = k.upTunnel(k.port, k.localPort, tunnelErrorCh, startID)
			if err != nil {
				log.DebugF("[%d] Tunnel was not up: %v. Try to restart fully\n", startID, err)
				k.tryToRestartFully(startID)
				return
			}

			log.DebugF("[%d] Tunnel re up successfully\n")

		case <-stopCh:
			log.DebugF("[%d] Kubeproxy monitor stopped")
			return
		}
	}
}

func (k *KubeProxy) upTunnel(kubeProxyPort string, useLocalPort int, tunnelErrorCh chan error, startID int) (tun *Tunnel, localPort int, err error) {
	log.DebugF("[%d] Starting up tunnel with proxy port %s and local port %d\n", startID, kubeProxyPort, useLocalPort)

	rewriteLocalPort := false
	localPort = useLocalPort

	if useLocalPort < 1 {
		log.DebugF("[%d] Incorrect local port %d use default %d\n", startID, useLocalPort, DefaultLocalAPIPort)
		localPort = DefaultLocalAPIPort
		rewriteLocalPort = true
	}

	maxRetries := 5
	retries := 0
	var lastError error
	for {
		log.DebugF("[%d] Start %d iteration for up tunnel\n", startID, retries)

		if k.proxy.WaitError() != nil {
			lastError = fmt.Errorf("proxy was failed while restart tunnel")
			break
		}

		// try to start tunnel from localPort to proxy port
		var tunnelAddress string
		if v := os.Getenv("KUBE_PROXY_BIND_ADDR"); v != "" {
			tunnelAddress = fmt.Sprintf("%s:%d:localhost:%s", v, localPort, kubeProxyPort)
		} else {
			tunnelAddress = fmt.Sprintf("%d:localhost:%s", localPort, kubeProxyPort)
		}

		log.DebugF("[%d] Try up tunnel on %v\n", startID, tunnelAddress)
		tun = NewTunnel(k.Session, "L", tunnelAddress)
		err := tun.Up()
		if err != nil {
			log.DebugF("[%d] Start tunnel was failed. Cleaning...\n", startID)
			tun.Stop()
			lastError = fmt.Errorf("tunnel '%s': %v", tunnelAddress, err)
			log.DebugF("[%d] Start tunnel was failed. Error: %v\n", startID, lastError)
			if rewriteLocalPort {
				localPort++
				log.DebugF("[%d] New local port %d\n", startID, localPort)
			}

			retries++
			if retries >= maxRetries {
				log.DebugF("[%d] Last iteration finished\n", startID)
				tun = nil
				break
			}
		} else {
			log.DebugF("[%d] Tunnel was started. Starting health monitor\n", startID)
			go tun.HealthMonitor(tunnelErrorCh)
			lastError = nil
			break
		}
	}

	dbgMsg := fmt.Sprintf("Tunnel up on local port %d", localPort)
	if lastError != nil {
		dbgMsg = fmt.Sprintf("Tunnel was not up: %v", lastError)
	}

	log.DebugF("[%d] %s\n", startID, dbgMsg)

	return tun, localPort, lastError
}

func (k *KubeProxy) runKubeProxy(waitCh chan error, startID int) (proxy *Command, port string, err error) {
	log.DebugF("[%d] Begin starting proxy\n", startID)
	proxy = k.proxyCMD(startID)

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
		log.DebugF("[%d] Command started\n", startID)
		onStart <- struct{}{}
	})

	proxy.WithWaitHandler(func(err error) {
		log.DebugF("[%d] Wait error: %v\n", startID, err)
		waitCh <- err
	})

	log.DebugF("[%d] Start proxy command\n", startID)
	err = proxy.Start()
	if err != nil {
		log.DebugF("[%d] Start proxy command error: %v\n", startID, err)
		return nil, "", fmt.Errorf("start kubectl proxy: %v", err)
	}

	log.DebugF("[%d] Proxy command was started\n", startID)

	returnWaitErr := func(err error) error {
		log.DebugF("[%d] Proxy command waiting error: %v\n", startID, err)
		template := `Proxy exited suddenly: %s%s
Status: %v`
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
		log.DebugF("[%d] Starting proxy command timeout\n", startID)
		return nil, "", fmt.Errorf("timeout waiting for api proxy port")
	case <-portReady:
		if port == "" {
			log.DebugF("[%d] Starting proxy command: empty port\n", startID)
			return nil, "", fmt.Errorf("got empty port from kubectl proxy")
		}
	}

	log.DebugF("[%d] Proxy process started with port: %s\n", startID, port)
	return proxy, port, nil
}
