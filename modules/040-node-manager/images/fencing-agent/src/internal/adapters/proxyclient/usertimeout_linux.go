//go:build linux

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

package proxyclient

import (
	"net"
	"time"

	"golang.org/x/sys/unix"
)

// setUserTimeout sets TCP_USER_TIMEOUT of a TCP connection.
func setUserTimeout(c net.Conn, d time.Duration) error {
	tcp, ok := c.(*net.TCPConn)
	if !ok {
		return nil
	}

	raw, err := tcp.SyscallConn()
	if err != nil {
		return err
	}

	var serr error

	err = raw.Control(func(fd uintptr) {
		serr = unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_USER_TIMEOUT, int(d.Milliseconds()))
	})
	if err != nil {
		return err
	}

	return serr
}
