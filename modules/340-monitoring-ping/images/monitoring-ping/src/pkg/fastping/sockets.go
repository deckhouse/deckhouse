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
	"bytes"
	"container/heap"
	"context"
	"encoding/binary"
	"fmt"
	"golang.org/x/sys/unix"
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
	epfd int        // event poll file descriptor
	id          int
	seqPerHost  map[string]int
	pending     sync.Map // thread-safe map: key -> pendingPacket
	pendingLock sync.Mutex
}

func newSocket(_ context.Context) (*socketConn, error) {
	// Create raw ICMP socket
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_ICMP)
	if err != nil {
		return nil, fmt.Errorf("failed to create raw socket: %w", err)
	}

	// Create EPOLL FD
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to create epfd: %w", err)
	}

	// Register EPOLL
	event := &unix.EpollEvent{
		Events: unix.EPOLLIN,
		Fd:     int32(fd),
	}
	if err := unix.EpollCtl(epfd, unix.EPOLL_CTL_ADD, fd, event); err != nil {
		syscall.Close(fd)
		unix.Close(epfd)
		return nil, fmt.Errorf("failed to register socket in epoll: %w", err)
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
		epfd: epfd,
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

	// Close EPOLL FD
	if s.epfd > 0 {
		unix.Close(s.epfd)
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
			// Wait for the socket to become readable using epoll
			timeoutMS := int(time.Until(deadline).Milliseconds())
			if timeoutMS < 0 {
				return "", 0, fmt.Errorf("read timeout after %s", timeout)
			}
			events := make([]unix.EpollEvent, 1)
			n, err := unix.EpollWait(s.epfd, events, timeoutMS)
			if err != nil {
				if err == syscall.EINTR {
					continue // interrupted by signal, retry
				}
				return "", 0, fmt.Errorf("epoll_wait error: %w", err)
			}
			if n == 0 {
				return "", 0, fmt.Errorf("read timeout after %s", timeout)
			}

			// Receive message using recvmsg to also get control messages
			n, oobn, _, from, err := syscall.Recvmsg(s.fd, buf, oob, 0)
			if err != nil {
				if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
					continue // shouldn't happen after epoll, but just in case
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

// sendEventLoop schedules ping packets using a priority queue and sends them at precise intervals.
// It avoids spawning many goroutines, providing efficient timing even for thousands of hosts.
func (p *Pinger) sendEventLoop(ctx context.Context, conn *socketConn) error {
	log.Info("Starting sendEventLoop with min-heap scheduling")

	h := make(pingHeap, 0, len(p.hosts)*p.count)
	heap.Init(&h)

	now := time.Now()

	// Initialize first wave of scheduled pings for all hosts
	for _, host := range p.hosts {
		heap.Push(&h, &scheduledPing{
			host:   host,
			sendAt: now,
			count:  p.count,
		})
	}

	for h.Len() > 0 {
		select {
		case <-ctx.Done():
			log.Info("sendEventLoop stopped due to context cancellation")
			return ctx.Err()
		default:
		}

		next := heap.Pop(&h).(*scheduledPing)
		now = time.Now()
		sleepDur := next.sendAt.Sub(now)

		if sleepDur > 0 {
			time.Sleep(sleepDur)
		}

		// Send the actual ICMP packet
		if err := conn.SendPacket(next.host); err != nil {
			log.Warn(fmt.Sprintf("Failed to send ping to %s: %v", next.host, err))
		} else {
			p.mu.Lock()
			p.sentCount[next.host]++
			p.mu.Unlock()
		}

		// Schedule the next ping if remaining
		next.count--
		if next.count > 0 {
			next.sendAt = next.sendAt.Add(p.interval)
			heap.Push(&h, next)
		}
	}

	log.Info("Completed all scheduled pings")
	return nil
}
