package frontend

import (
	"fmt"
	"regexp"
	"time"

	"github.com/deckhouse/deckhouse/candictl/pkg/log"
	"github.com/deckhouse/deckhouse/candictl/pkg/system/ssh/session"
)

var LocalAPIPort = 22322

type KubeProxy struct {
	Session *session.Session

	KubeProxyPort string
	LocalPort     string

	proxy  *Command
	tunnel *Tunnel

	stop bool
	port string
}

func NewKubeProxy(sess *session.Session) *KubeProxy {
	return &KubeProxy{Session: sess, port: "0"}
}

func (k *KubeProxy) ProxyCMD() *Command {
	command := fmt.Sprintf("kubectl proxy --port=%s --kubeconfig /etc/kubernetes/admin.conf", k.port)
	cmd := NewCommand(k.Session, command).Sudo()
	cmd.Executor = cmd.Executor.CaptureStderr(nil).CaptureStdout(nil)
	return cmd
}

func (k *KubeProxy) Start() (port string, err error) {
	success := false
	defer func() {
		if !success {
			k.Stop()
		}
	}()

	k.proxy = k.ProxyCMD()

	port = ""
	portReady := make(chan struct{}, 1)
	portRe := regexp.MustCompile(`Starting to serve on .*?:(\d+)`)

	k.proxy.WithStdoutHandler(func(line string) {
		m := portRe.FindStringSubmatch(line)
		if len(m) == 2 && m[1] != "" {
			port = m[1]
			log.InfoF("Got proxy port = %s\n", port)
			k.port = port
			portReady <- struct{}{}
		}
	})

	onStart := make(chan struct{}, 1)
	k.proxy.OnCommandStart(func() {
		onStart <- struct{}{}
	})
	waitCh := make(chan error, 1)
	k.proxy.WithWaitHandler(func(err error) {
		waitCh <- err
	})

	log.DebugLn("Start proxy process")
	err = k.proxy.Start()
	if err != nil {
		return "", fmt.Errorf("start kubectl proxy: %v", err)
	}

	<-onStart

	// Wait for proxy startup
	t := time.NewTicker(20 * time.Second)
	defer t.Stop()
	select {
	case e := <-waitCh:
		template := `Proxy exited suddenly:
%s%sStatus: %v`
		return "", fmt.Errorf(template, string(k.proxy.StdoutBytes()), string(k.proxy.StderrBytes()), e)
	case <-t.C:
		return "", fmt.Errorf("timeout waiting for api proxy port")
	case <-portReady:
		if port == "" {
			return "", fmt.Errorf("got empty port from kubectl proxy")
		}
	}

	localPort := LocalAPIPort
	maxRetries := 12
	retry := 0
	var lastError error
	var tun *Tunnel

	for {
		// try to start tunnel from localPort to proxy port
		tunnelAddress := fmt.Sprintf("%d:localhost:%s", localPort, k.port)
		tun = NewTunnel(k.Session, "L", tunnelAddress)
		// TODO if local port is busy, increase port and start again
		err := tun.Up()
		if err != nil {
			tun.Stop()
			lastError = fmt.Errorf("tunnel '%s': %v", tunnelAddress, err)
			localPort++
			retry++
			if retry >= maxRetries {
				tun = nil
				break
			}
		} else {
			break
		}
	}

	if tun == nil {
		return "", fmt.Errorf("tunnel up error: max retries reached, last error: %v", lastError)
	}

	k.tunnel = tun
	success = true

	go func() {
		err := k.tunnel.HealthMonitor()
		if err != nil {
			log.ErrorLn(err)
		}
	}()

	go func() {
		for !k.stop {
			proxyErr := <-waitCh
			log.DebugF("Kubectl proxy crushed: %v\n", proxyErr)

			k.proxy = k.ProxyCMD()
			k.proxy.WithWaitHandler(func(err error) {
				waitCh <- err
			})

			err = k.proxy.Start()
			if err != nil {
				log.DebugF("Start kubectl proxy: %v\n", err)
				return
			}

			log.DebugLn("Kubectl proxy restarted")
		}
	}()
	return fmt.Sprintf("%d", localPort), nil
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
		k.proxy.Stop()
	}
	if k.tunnel != nil {
		k.tunnel.Stop()
	}
	k.stop = true
}

func (k *KubeProxy) Restart() error {
	k.Stop()
	_, err := k.Start()
	return err
}
