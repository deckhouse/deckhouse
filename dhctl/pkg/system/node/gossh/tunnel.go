// Copyright 2024 Flant JSC
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

package gossh

import (
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
)

type Tunnel struct {
	sshClient *ssh.Client
	address   string

	tunMutex sync.Mutex

	started        bool
	stopCh         chan struct{}
	remoteListener net.Listener

	errorCh chan tunnelWaitResult
	errCh   chan error
}

func NewTunnel(sshClient *ssh.Client, address string) *Tunnel {
	return &Tunnel{
		sshClient: sshClient,
		address:   address,
		errorCh:   make(chan tunnelWaitResult),
	}
}

func (t *Tunnel) Up() error {
	_, err := t.upNewTunnel(-1)
	return err
}

func (t *Tunnel) upNewTunnel(oldId int) (int, error) {
	t.tunMutex.Lock()
	defer t.tunMutex.Unlock()

	if t.started {
		log.DebugF("[%d] Reverse tunnel already up\n", oldId)
		return -1, fmt.Errorf("already up")
	}

	id := rand.Int()

	parts := strings.Split(t.address, ":")
	if len(parts) != 4 {
		return -1, fmt.Errorf("invalid address must be 'remote_bind:remote_port:local_bind:local_port': %s", t.address)
	}

	remoteBind, remotePort, localBind, localPort := parts[0], parts[1], parts[2], parts[3]

	log.DebugF("[%d] Remote bind: %s remote port: %s local bind: %s local port: %s\n", id, remoteBind, remotePort, localBind, localPort)

	log.DebugF("[%d] Start reverse tunnel\n", id)

	remoteAddress := net.JoinHostPort(remoteBind, remotePort)
	localAddress := net.JoinHostPort(localBind, localPort)

	// reverse listen on remote server port
	listener, err := t.sshClient.Listen("tcp", localAddress)
	if err != nil {
		return -1, errors.Wrap(err, fmt.Sprintf("failed to listen remote on %s", localAddress))
	}

	log.DebugF("[%d] Listen remote %s successful\n", id, localAddress)

	go t.acceptTunnelConnection(id, remoteAddress, listener)

	t.remoteListener = listener
	t.started = true

	return id, nil
}

func (t *Tunnel) acceptTunnelConnection(id int, localAddress string, listener net.Listener) {
	for {
		client, err := listener.Accept()
		if err != nil {
			e := fmt.Errorf("Accept(): %s", err.Error())
			t.errorCh <- tunnelWaitResult{
				id:  id,
				err: e,
			}
			return
		}

		log.DebugF("[%d] connection accepted. Try to connect to local %s\n", id, localAddress)

		local, err := net.Dial("tcp", localAddress)
		if err != nil {
			e := fmt.Errorf("Cannot dial to %s: %s", localAddress, err.Error())
			t.errorCh <- tunnelWaitResult{
				id:  id,
				err: e,
			}
			return
		}

		log.DebugF("[%d] Connected to local %s\n", id, localAddress)

		// handle the connection in another goroutine, so we can support multiple concurrent
		// connections on the same port
		go t.handleClient(id, local, client)
	}
}

func (t *Tunnel) handleClient(id int, client net.Conn, remote net.Conn) {
	defer func() {
		err := client.Close()
		if err != nil {
			log.DebugF("[%d] Cannot close connection: %s\n", id, err)
		}
	}()

	chDone := make(chan struct{}, 2)

	// Start remote -> local data transfer
	go func() {
		_, err := io.Copy(client, remote)
		if err != nil {
			log.WarnF(fmt.Sprintf("[%d] Error while copy remote->local: %s\n", id, err))
		}
		chDone <- struct{}{}
	}()

	// Start local -> remote data transfer
	go func() {
		_, err := io.Copy(remote, client)
		if err != nil {
			log.WarnF(fmt.Sprintf("[%d] Error while copy local->remote: %s\n", id, err))
		}
		chDone <- struct{}{}
	}()

	<-chDone
}

func (t *Tunnel) isStarted() bool {
	t.tunMutex.Lock()
	defer t.tunMutex.Unlock()
	r := t.started
	return r
}

func (t *Tunnel) tryToRestart(id int, killer node.ReverseTunnelKiller) (int, error) {
	t.stop(id, false)
	log.DebugF("[%d] Kill tunnel\n", id)
	if out, err := killer.KillTunnel(); err != nil {
		log.DebugF("[%d] Kill tunnel was finished with error: %v; stdout: '%s'\n", id, err, out)
		return id, err
	}
	return t.upNewTunnel(id)
}

func (t *Tunnel) HealthMonitor(errorOutCh chan<- error) {
	defer log.DebugF("Tunnel health monitor stopped\n")
	log.DebugF("Tunnel health monitor started\n")

	t.stopCh = make(chan struct{}, 1)

	for {
		select {
		case err := <-t.errCh:
			errorOutCh <- err
		case <-t.stopCh:
			_ = t.remoteListener.Close()
			return
		}
	}
}

func (t *Tunnel) Stop() {
	t.stop(-1, true)
}

func (t *Tunnel) stop(id int, full bool) {
	t.tunMutex.Lock()
	defer t.tunMutex.Unlock()

	if !t.started {
		log.DebugF("[%d] Reverse tunnel already stopped\n", id)
		return
	}

	log.DebugF("[%d] Stop reverse tunnel\n", id)
	defer log.DebugF("[%d] End stop reverse tunnel\n", id)

	if full && t.stopCh != nil {
		log.DebugF("[%d] Stop reverse tunnel health monitor\n", id)
		t.stopCh <- struct{}{}
	}

	err := t.remoteListener.Close()
	if err != nil {
		log.WarnF("[%d] Cannot close remote listener: %s\n", id, err.Error())
	}

	t.remoteListener = nil
	t.started = false
}

func (t *Tunnel) String() string {
	return fmt.Sprintf("%s:%s", "R", t.address)
}
