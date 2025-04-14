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
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type pendingPacket struct {
	host     string
	sentTime time.Time
}

type socketConn struct {
	c           *icmp.PacketConn
	id          int
	seqPerHost  map[string]int
	pending     map[string]pendingPacket
	pendingLock sync.Mutex
}

func newSocket() (*socketConn, error) {
	c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, fmt.Errorf("error opening raw socket: %w", err)
	}

	conn := &socketConn{
		c:          c,
		id:         genIdentifier(),
		seqPerHost: make(map[string]int),
		pending:    make(map[string]pendingPacket),
	}

	go conn.cleanupLoop()

	return conn, nil
}

func (s *socketConn) Close() error {
	return s.c.Close()
}

func (s *socketConn) SendPacket(host string) error {
	dst, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		return fmt.Errorf("failed to resolve host %s: %w", host, err)
	}

	// Individual seq per host
	s.seqPerHost[host]++
	seq := s.seqPerHost[host]

	sendTime := time.Now()

	// Remember pkt for host
	ip := dst.IP.String()
	key := makeKey(ip, seq)
	log.Info(fmt.Sprintf("created key: %s, for addr: %s and seq:%d", key, ip, seq))
	s.pendingLock.Lock()
	s.pending[key] = pendingPacket{host: host, sentTime: sendTime}
	s.pendingLock.Unlock()

	message := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   s.id,
			Seq:  seq,
			Data: timeToBytes(sendTime),
		},
	}

	b, err := message.Marshal(nil)
	if err != nil {
		return fmt.Errorf("failed to marshal ICMP packet: %w", err)
	}

	if _, err = s.c.WriteTo(b, dst); err != nil {
		return err
	}

	return nil
}

func (s *socketConn) ReadPacket(timeout time.Duration) (string, time.Duration, error) {
	reply := make([]byte, 1500)

	if err := s.c.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return "", 0, fmt.Errorf("failed to set read deadline: %w", err)
	}

	n, peer, err := s.c.ReadFrom(reply)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			log.Debug("read timeout after %ss for peer %v", timeout, peer)
			return "", 0, err
		}
		return "", 0, fmt.Errorf("read error: %w", err)
	}

	parsedMessage, err := icmp.ParseMessage(ipv4.ICMPTypeEchoReply.Protocol(), reply[:n])
	if err != nil {
		log.Warn(fmt.Sprintf("failed to parse ICMP message from %s: %v", peer.String(), err))
		return "", 0, err
	}

	pkt, ok := parsedMessage.Body.(*icmp.Echo)
	if !ok || pkt.ID != s.id {
		log.Debug("ignoring packet with invalid ID or type from %s", peer.String())
		return "", 0, nil
	}

	host := peer.String()
	ip := peer.(*net.IPAddr).IP.String()
	key := makeKey(ip, pkt.Seq)

	s.pendingLock.Lock()
	packetInfo, exists := s.pending[key]
	if exists {
		delete(s.pending, key)
	}
	s.pendingLock.Unlock()

	if !exists {
		log.Info(fmt.Sprintf("received duplicate or unexpected packet seq: %d, key: %s from address: %s, skipping", pkt.Seq, key, host))
		return "", 0, nil
	}

	rtt := time.Since(packetInfo.sentTime)
	return packetInfo.host, rtt, nil
}

func (p *Pinger) listenReplies(ctx context.Context, conn *socketConn) error {
	for {
		select {
		case <-ctx.Done():
			log.Info("listenReplies stopped due to context cancellation")
			return nil
		default:
			addr, rtt, err := conn.ReadPacket(p.timeout)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // Skip logging timeouts to reduce noise
				}
				log.Warn(fmt.Sprintf("failed to read packet: %v from address: %s", err, addr))
				continue
			}
			if addr == "" {
				continue // Packet was ignored (e.g., wrong ID)
			}

			p.mu.Lock()
			host, ok := p.hostMap[addr]
			if host, ok := p.hostMap[addr]; ok {
				p.recvCount[host]++
			}
			p.mu.Unlock()

			if !ok {
				log.Warn(fmt.Sprintf("received packet from unknown host: %s", addr))
				continue
			}

			log.Info(fmt.Sprintf("received ping from %s (%s), rtt: %v", host, addr, rtt))
			if p.OnRecv != nil {
				p.OnRecv(PacketResult{Host: host, RTT: rtt})
			}
		}
	}
}

func (p *Pinger) sendPings(ctx context.Context, conn *socketConn) error {
	log.Info(fmt.Sprintf("Sending pings, count: %d, hosts: %v", p.count, p.hosts))
	for i := 0; i < p.count; i++ {
		for _, host := range p.hosts {
			select {
			case <-ctx.Done():
				log.Info("sendPings stopped due to context cancellation")
				return ctx.Err()
			default:
				if err := conn.SendPacket(host); err != nil {
					log.Warn(fmt.Sprintf("failed to send ping to %s: %v", host, err))
					continue
				}
				p.mu.Lock()
				p.sentCount[host]++
				p.mu.Unlock()
			}
		}
		select {
		case <-ctx.Done():
			log.Info("sendPings stopped due to context cancellation")
			return ctx.Err()
		case <-time.After(p.interval):
		}
	}
	log.Info(fmt.Sprintf("Completed sending %d pings", p.count))
	return nil
}

func (s *socketConn) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.pendingLock.Lock()
		now := time.Now()
		for k, v := range s.pending {
			if now.Sub(v.sentTime) > 64*time.Second {
				delete(s.pending, k)
			}
		}
		s.pendingLock.Unlock()
	}
}
