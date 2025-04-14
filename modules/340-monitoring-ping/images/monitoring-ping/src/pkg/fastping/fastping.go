// Package ping Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fastping

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func (p *Pinger) RunWithContext(ctx context.Context) error {
	conn, err := newSocket()
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}
	defer conn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	errCh := make(chan error, 2)

	// Context for goroutines, cancellable independently
	gctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		defer wg.Done()
		if err := p.listenReplies(gctx, conn); err != nil && !errors.Is(err, context.Canceled) {
			log.Warn(fmt.Sprintf("listenReplies error: %v", err))
			errCh <- err
		}
		log.Debug("listenReplies goroutine stopped")
	}()

	go func() {
		defer wg.Done()
		if err := p.sendPings(gctx, conn); err != nil && !errors.Is(err, context.Canceled) {
			log.Warn(fmt.Sprintf("sendPings error: %v", err))
			errCh <- err
		}
		log.Debug("sendPings goroutine stopped")
		// Cancel context to signal listenReplies to wrap up
		cancel()
	}()

	// Wait for goroutines to finish
	go func() {
		wg.Wait()
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("RunWithContext stopped due to context cancellation")
		return ctx.Err()
	}
}

func (p *Pinger) StatsForHost(host string) (sent int, recv int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if sentVal, ok := p.sentCount[host]; ok {
		sent = sentVal
		recv = p.recvCount[host]
	} else {

		recv = p.recvCount[host]
		for ip, name := range p.hostMap {
			if name == host {
				sent += p.sentCount[ip]
			}
		}
	}

	return sent, recv
}
