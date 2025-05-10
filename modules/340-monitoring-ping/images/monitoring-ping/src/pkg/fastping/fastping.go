/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fastping

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func (p *Pinger) RunWithContext(ctx context.Context) error {
	conn, err := newSocket(ctx)
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}

	defer conn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	errCh := make(chan error, 2)

	// Context for goroutines, cancellable independently
	// gctx, cancel := context.WithCancel(ctx)
	durationPingWithTimeuout := p.count*int(p.interval) + int(p.timeout)
	durationPing := p.count*int(p.interval) + int(p.timeout)
	sendCtx, sendCancel := context.WithTimeout(ctx, time.Duration(durationPing))

	listenCtx, listenCancel := context.WithTimeout(ctx, time.Duration(durationPingWithTimeuout))
	defer listenCancel()
	defer sendCancel()

	go func() {
		defer wg.Done()

		if err := p.listenReplies(listenCtx, conn); err != nil && !errors.Is(err, context.Canceled) {
			log.Warn(fmt.Sprintf("listenReplies error: %v", err))
			errCh <- err
		}
		log.Debug("listenReplies goroutine stopped")
	}()

	go func() {
		defer wg.Done()
		if err := p.sendEventLoop(sendCtx, conn); err != nil && !errors.Is(err, context.Canceled) {
			log.Warn(fmt.Sprintf("sendEventLoop error: %v", err))
			errCh <- err
		}
		log.Debug("sendEventLoop goroutine stopped")
	}()

	// go func() {
	// 	defer wg.Done()
	// 	if err := p.sendPings(sendCtx, conn); err != nil && !errors.Is(err, context.Canceled) {
	// 		log.Warn(fmt.Sprintf("sendPings error: %v", err))
	// 		errCh <- err
	// 	}
	// 	log.Debug("sendPings goroutine stopped")
	// }()

	// Wait for goroutines to finish
	go func() {
		wg.Wait()
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			log.Info(fmt.Sprintf("RunWithContext exiting with error: %v", err))
			return err
		}
		log.Info("RunWithContext exiting normally (no error)")
		return nil
	case <-ctx.Done():
		log.Info("RunWithContext stopped due to context cancellation")
		return ctx.Err()
	}
}

// StatsForHost returns the number of sent and received ICMP packets for a given host.
// The host can be either an IP address (e.g., "8.8.8.8") or a domain name (e.g., "google.com").
//
// If the host is an IP address, the stats are returned directly from the maps.
// If the host is a domain name (that resolved to multiple IPs), it aggregates `sentCount`
// across all IPs that were resolved for this domain.
func (p *Pinger) StatsForHost(host string) (sent int, recv int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Fast path: if host is an IP that was pinged directly, return its stats
	if sentVal, ok := p.sentCount[host]; ok {
		sent = sentVal
		recv = p.recvCount[host]
	} else {
		// Otherwise, assume `host` is a domain name (e.g., "vk.com")
		// and aggregate `sentCount` for all IPs that map to this domain name
		recv = p.recvCount[host] // recv count is still stored by name

		// Walk through the hostMap and find all IPs mapped to this name
		for ip, name := range p.hostMap {
			if name == host {
				sent += p.sentCount[ip]
			}
		}
	}

	return sent, recv
}
