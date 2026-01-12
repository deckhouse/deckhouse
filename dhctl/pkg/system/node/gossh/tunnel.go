// Copyright 2025 Flant JSC
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

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type Tunnel struct {
	sshClient *Client
	address   string

	tunMutex sync.Mutex

	started        bool
	stopCh         chan struct{}
	remoteListener net.Listener

	errorCh chan error
}

func NewTunnel(sshClient *Client, address string) *Tunnel {
	return &Tunnel{
		sshClient: sshClient,
		address:   address,
		errorCh:   make(chan error, 10),
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
		log.DebugF("[%d] Tunnel already up\n", oldId)
		return -1, fmt.Errorf("already up")
	}

	id := rand.Int()

	parts := strings.Split(t.address, ":")
	if len(parts) != 4 {
		return -1, fmt.Errorf("invalid address must be 'remote_bind:remote_port:local_bind:local_port': %s", t.address)
	}

	remoteBind, remotePort, localBind, localPort := parts[0], parts[1], parts[2], parts[3]

	log.DebugF("[%d] Remote bind: %s remote port: %s local bind: %s local port: %s\n", id, remoteBind, remotePort, localBind, localPort)

	log.DebugF("[%d] Start tunnel\n", id)

	remoteAddress := net.JoinHostPort(remoteBind, remotePort)
	localAddress := net.JoinHostPort(localBind, localPort)

	listener, err := net.Listen("tcp", localAddress)
	if err != nil {
		return -1, errors.Wrap(err, fmt.Sprintf("failed to listen local on %s", localAddress))
	}

	log.DebugF("[%d] Listen remote %s successful\n", id, localAddress)

	go t.acceptTunnelConnection(id, remoteAddress, listener)

	t.remoteListener = listener
	t.started = true

	return id, nil
}

func (t *Tunnel) acceptTunnelConnection(id int, remoteAddress string, listener net.Listener) {
	for {
		localConn, err := listener.Accept()
		if err != nil {
			e := fmt.Errorf("[%d] Accept(): %s", id, err.Error())
			t.errorCh <- e
			continue
		}

		remoteConn, err := t.sshClient.GetClient().Dial("tcp", remoteAddress)
		if err != nil {
			e := fmt.Errorf("[%d] Cannot dial to %s: %s", id, remoteAddress, err.Error())
			t.errorCh <- e
			continue
		}

		go func() {
			defer localConn.Close()
			defer remoteConn.Close()
			go func() {
				_, err := io.Copy(remoteConn, localConn)
				if err != nil {
					t.errorCh <- err
				}

			}()

			_, err := io.Copy(localConn, remoteConn)
			if err != nil {
				t.errorCh <- err
			}

		}()
	}
}

func (t *Tunnel) HealthMonitor(errorOutCh chan<- error) {
	defer log.DebugF("Tunnel health monitor stopped\n")
	log.DebugF("Tunnel health monitor started\n")

	t.stopCh = make(chan struct{}, 1)

	for {
		select {
		case err := <-t.errorCh:
			errorOutCh <- err
		case <-t.stopCh:
			if t.remoteListener != nil {
				_ = t.remoteListener.Close()
			}
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
		log.DebugF("[%d] Tunnel already stopped\n", id)
		return
	}

	log.DebugF("[%d] Stop tunnel\n", id)
	defer log.DebugF("[%d] End stop tunnel\n", id)

	if full && t.stopCh != nil {
		log.DebugF("[%d] Stop tunnel health monitor\n", id)
		t.stopCh <- struct{}{}
	}

	err := t.remoteListener.Close()
	if err != nil {
		log.WarnF("[%d] Cannot close listener: %s\n", id, err.Error())
	}

	t.remoteListener = nil
	t.started = false
}

func (t *Tunnel) String() string {
	return fmt.Sprintf("%s:%s", "L", t.address)
}
