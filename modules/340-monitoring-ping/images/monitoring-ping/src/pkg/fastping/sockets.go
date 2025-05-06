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
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type pendingPacket struct {
	host     string
	sentTime int64
}

type socketConn struct {
	fd          int // raw socket file descriptor
	id          int
	seqPerHost  map[string]int
	pending     sync.Map // thread-safe map: key -> pendingPacket
	pendingLock sync.Mutex
}

func newSocket(ctx context.Context) (*socketConn, error) {
	// Create raw ICMP socket
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_ICMP)
	if err != nil {
		return nil, fmt.Errorf("failed to create raw socket: %w", err)
	}

	// Set socket to non-blocking mode
	err = syscall.SetNonblock(fd, true)
	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to set non-blocking mode: %w", err)
	}

	// Set socket receive buffer (4MB)
	err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, 4*1024*1024)
	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to set SO_RCVBUF: %w", err)
	}

	// Set socket send buffer (4MB)
	err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_SNDBUF, 4*1024*1024)
	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to set SO_SNDBUF: %w", err)
	}

	// Bind to 0.0.0.0
	err = syscall.Bind(fd, &syscall.SockaddrInet4{Addr: [4]byte{0, 0, 0, 0}})
	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to bind socket: %w", err)
	}

	// Enable kernel timestamping (nanosecond precision)
	err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_TIMESTAMPNS, 1)
	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to enable SO_TIMESTAMPNS: %w", err)
	}

	// Set receive timeout
	// Note: SO_RCVTIMEO is not effective in non-blocking mode, timeout is handled manually.
	tv := syscall.NsecToTimeval((5 * time.Second).Nanoseconds())
	err = syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to set SO_RCVTIMEO: %w", err)
	}

	conn := &socketConn{
		fd:         fd,
		id:         genIdentifier(),
		seqPerHost: make(map[string]int),
		pending:    sync.Map{},
	}

	// go conn.cleanupLoop(ctx)

	return conn, nil
}

func (s *socketConn) Close() error {
	// Close the raw socket
	if err := syscall.Close(s.fd); err != nil {
		log.Warn(fmt.Sprintf("failed to close socket fd: %v", err))
	}

	// Clear the pending map to free memory and avoid memory leak
	s.pending.Range(func(key, value any) bool {
		s.pending.Delete(key)
		return true
	})

	log.Info("socketConn closed and pending map cleared")
	return nil
}

// SendPacket builds and sends an ICMP Echo Request packet to the target host
func (s *socketConn) SendPacket(host string) error {
	ipAddr, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		return fmt.Errorf("failed to resolve host %s: %w", host, err)
	}

	// Increment and get sequence number for this host
	s.pendingLock.Lock()
	s.seqPerHost[host]++
	seq := s.seqPerHost[host]
	s.pendingLock.Unlock()

	// Save packet metadata in sync.Map
	sentTime := time.Now().UnixNano()
	key := makeKey(ipAddr.IP.String(), seq)
	s.pending.Store(key, pendingPacket{host: host, sentTime: sentTime})

	// Build ICMP Echo Request packet
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint8(8))         // Type: Echo Request
	binary.Write(&buf, binary.BigEndian, uint8(0))         // Code: 0
	binary.Write(&buf, binary.BigEndian, uint16(0))        // Checksum placeholder
	binary.Write(&buf, binary.BigEndian, uint16(s.id))     // Identifier
	binary.Write(&buf, binary.BigEndian, uint16(seq))      // Sequence number
	binary.Write(&buf, binary.BigEndian, uint64(sentTime)) // Payload: timestamp in nanoseconds

	pkt := buf.Bytes()
	checksum := computeChecksum(pkt)
	pkt[2] = byte(checksum >> 8) // Fill in checksum
	pkt[3] = byte(checksum)

	dstAddr := &syscall.SockaddrInet4{}
	copy(dstAddr.Addr[:], ipAddr.IP.To4())
	if err := syscall.Sendto(s.fd, pkt, 0, dstAddr); err != nil {
		return fmt.Errorf("failed to send ICMP packet to %s: %w", ipAddr.IP.String(), err)
	}

	return nil
}

// ReadPacket reads an ICMP Echo Reply packet from the raw socket,
// extracts the kernel timestamp (SO_TIMESTAMPNS), and returns the RTT excluding kernel delay.
func (s *socketConn) ReadPacket(ctx context.Context, timeout time.Duration) (string, time.Duration, error) {
	// Allocate buffer for packet data and out-of-band control messages (for timestamp)
	buf := make([]byte, 1500)
	oob := make([]byte, 512)

	// Deadline for the read operation
	deadline := time.Now().Add(timeout)

	for {
		select {
		case <-ctx.Done():
			return "", 0, ctx.Err()
		default:
			// Receive message using recvmsg to also get control messages
			n, oobn, _, from, err := syscall.Recvmsg(s.fd, buf, oob, 0)
			if err != nil {
				if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
					if time.Now().After(deadline) {
						return "", 0, fmt.Errorf("read timeout after %s", timeout)
					}
					time.Sleep(10 * time.Microsecond)
					continue
				}
				if err == syscall.EINTR {
					continue
				}
				return "", 0, fmt.Errorf("recvmsg error: %w", err)
			}

			// Extract sender IP address
			var ip string
			if sa, ok := from.(*syscall.SockaddrInet4); ok {
				ip = net.IP(sa.Addr[:]).String()
			} else {
				return "", 0, fmt.Errorf("unexpected sockaddr type")
			}

			// Skip IP header to get ICMP message
			ipHeaderLen := (buf[0] & 0x0F) * 4
			icmpData := buf[ipHeaderLen:n]

			// Ensure this is an Echo Reply with at least 16 bytes (header + timestamp)
			if len(icmpData) < 16 || icmpData[0] != 0 {
				continue // Not an Echo Reply or invalid length
			}

			// Parse identifier and sequence number
			pktID := int(binary.BigEndian.Uint16(icmpData[4:6]))
			pktSeq := int(binary.BigEndian.Uint16(icmpData[6:8]))

			// Skip replies from other pinger instances
			if pktID != s.id {
				continue
			}

			// Extract embedded send timestamp (written during SendPacket)
			sendUnixNano := int64(binary.BigEndian.Uint64(icmpData[8:16]))
			sendTime := time.Unix(0, sendUnixNano)

			// Extract kernel timestamp from control message (SO_TIMESTAMPNS)
			kernelTime := extractTimestampNS(oob[:oobn])
			if kernelTime.IsZero() {
				log.Warn(fmt.Sprintf("Missing kernel timestamp for packet from %s", ip))
				continue
			}

			// Calculate kernel-to-user latency
			kernelDelay := time.Since(kernelTime)

			// Total RTT is time from sending until now
			fullRTT := time.Since(sendTime)

			// Subtract time spent in userspace after the kernel received the packet
			userRTT := fullRTT - kernelDelay

			// Log for debug
			// log.Info(fmt.Sprintf("RTT: %v | KernelDelay: %v | UserDelay: %v", fullRTT, kernelDelay, userRTT))

			// Match the reply with our pending requests
			key := makeKey(ip, pktSeq)
			val, ok := s.pending.LoadAndDelete(key)
			if !ok {
				log.Debug(fmt.Sprintf("received unexpected packet from %s, seq=%d", ip, pktSeq))
				continue
			}
			packetInfo := val.(pendingPacket)

			// Return user-level RTT excluding kernel-induced delay
			return packetInfo.host, userRTT, nil
		}
	}
}

func (p *Pinger) listenReplies(ctx context.Context, conn *socketConn) error {
	for {
		select {
		case <-ctx.Done():
			log.Info("listenReplies stopped due to context cancellation")
			return nil
		default:
			addr, rtt, err := conn.ReadPacket(ctx, p.timeout)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // Skip logging timeouts to reduce noise
				}
				// log.Warn(fmt.Sprintf("failed to read packet: %v from address: %s", err, addr))
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

			// log.Info(fmt.Sprintf("received ping from %s (%s), rtt: %v", host, addr, rtt))
			if p.OnRecv != nil {
				p.OnRecv(PacketResult{Host: host, RTT: rtt})
			}
		}
	}
}

func (p *Pinger) sendPings(ctx context.Context, conn *socketConn) error {
	log.Info(fmt.Sprintf("Sending pings, count: %d", p.count))

	for i := 0; i < p.count; i++ {
		for _, host := range p.hosts {
			select {
			case <-ctx.Done():
				log.Info("sendPings loop interrupted by context cancellation")
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

		// Sleep between rounds
		if i < p.count-1 {
			time.Sleep(p.interval)
		}
	}

	log.Info(fmt.Sprintf("Completed sending %d pings per host", p.count))
	return nil
}

// func (s *socketConn) cleanupLoop(ctx context.Context) {
// 	ticker := time.NewTicker(10 * time.Second)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			log.Info("cleanupLoop stopped due to context cancellation")
// 			return
// 		case <-ticker.C:
// 			now := time.Now().UnixNano()
// 			s.pending.Range(func(k, v any) bool {
// 				pp := v.(pendingPacket)
// 				if now-pp.sentTime > int64(64*time.Second) {
// 					s.pending.Delete(k)
// 				}
// 				return true
// 			})
// 		}
// 	}
// }

// func buildICMP(seq int, connId int, sendTime time.Time) []byte {
// 	pkt := make([]byte, 8+8) // 8 byte header + 8 byte timestamp
// 	id := uint16(connId)
// 	s := uint16(seq)

// 	pkt[0] = uint8(8) // Type = Echo Request
// 	pkt[1] = uint8(0) // Code = 0
// 	pkt[2] = uint8(0) // Checksum placeholder
// 	pkt[3] = 0

// 	pkt[4] = byte(id >> 8) // Identifier
// 	pkt[5] = byte(id)
// 	pkt[6] = byte(s >> 8) // Sequence number
// 	pkt[7] = byte(s)

// 	// Timestamp into payload
// 	binary.BigEndian.PutUint64(pkt[8:], uint64(sendTime.UnixNano()))

// 	// Checksum
// 	cs := computeChecksum(pkt)
// 	pkt[2] = byte(cs >> 8)
// 	pkt[3] = byte(cs & 0xff)

// 	return pkt
// }
