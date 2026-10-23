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

package revocationfeed

import (
	"fmt"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// userTimeout is how long sent data may stay unacknowledged before the kernel drops the
// connection: three heartbeats.
const userTimeout = 30 * time.Second

// setUserTimeout sets TCP_USER_TIMEOUT on a listening socket; Linux copies it to every connection
// the socket accepts.
func setUserTimeout(_, _ string, c syscall.RawConn) error {
	var setErr error
	err := c.Control(func(fd uintptr) {
		setErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_USER_TIMEOUT, int(userTimeout.Milliseconds()))
	})
	if err != nil {
		return fmt.Errorf("set the TCP user timeout: %w", err)
	}
	if setErr != nil {
		return fmt.Errorf("set the TCP user timeout: %w", setErr)
	}
	return nil
}
