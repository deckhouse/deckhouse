/*
Copyright 2026 Flant JSC

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

package memberlist

import (
	"errors"
	"net/netip"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// memberlistRecvBuf is the receive buffer that memberlist asks for its UDP socket.
const memberlistRecvBuf = 2 << 20

// raiseRecvBuf sets SO_RCVBUF of the UDP socket on addr:port to
// net.core.rmem_max. memberlist asks for 2 MiB only and does not give its
// socket, so the function finds the socket among the descriptors of the process.
func raiseRecvBuf(addr netip.Addr, port int) error {
	raw, err := os.ReadFile("/proc/sys/net/core/rmem_max")
	if err != nil {
		return err
	}

	size, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return err
	}

	if size <= memberlistRecvBuf {
		return nil
	}

	fd, err := findUDPSocket(addr, port)
	if err != nil {
		return err
	}

	return unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_RCVBUF, size)
}

func findUDPSocket(addr netip.Addr, port int) (int, error) {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return 0, err
	}

	for _, e := range entries {
		fd, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}

		if typ, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_TYPE); err != nil || typ != unix.SOCK_DGRAM {
			continue
		}

		sa, err := unix.Getsockname(fd)
		if err != nil {
			continue
		}

		if in4, ok := sa.(*unix.SockaddrInet4); ok && in4.Port == port && netip.AddrFrom4(in4.Addr) == addr {
			return fd, nil
		}
	}

	return 0, errors.New("the UDP socket of memberlist is not found")
}
