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
	"testing"

	"golang.org/x/sys/unix"
)

// F2: an accepted connection gives up on a peer that has not acknowledged what was sent for 30
// seconds, instead of retransmitting for about 15 minutes while heartbeats pile up in the buffer.
// The option is set on the listening socket; the test checks that the accepted one has it.
func TestListen_UserTimeout(t *testing.T) {
	t.Parallel()

	raw := acceptedConn(t)
	if v := sockoptInt(t, raw, unix.IPPROTO_TCP, unix.TCP_USER_TIMEOUT); v != int(userTimeout.Milliseconds()) {
		t.Errorf("TCP_USER_TIMEOUT = %d ms, want %d", v, userTimeout.Milliseconds())
	}
}
