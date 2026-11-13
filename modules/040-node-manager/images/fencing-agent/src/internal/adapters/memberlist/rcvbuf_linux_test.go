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
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The receive settings of the main network: the receive buffer is raised to net.core.rmem_max.
func TestApplyRaisesTheReceiveBufferToRmemMax(t *testing.T) {
	raw, err := os.ReadFile("/proc/sys/net/core/rmem_max")
	if err != nil {
		t.Skipf("rmem_max is not readable: %v", err)
	}

	rmemMax, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("parse rmem_max: %v", err)
	}

	c := newTestCluster(t)
	a := c.start("node-a", testLabel, timingsConfig(time.Hour), nil, Deps{})

	fd, err := findUDPSocket(loopback, a.net.cur.Load().port)
	if err != nil {
		t.Fatalf("find the socket: %v", err)
	}

	got, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_RCVBUF)
	if err != nil {
		t.Fatalf("read SO_RCVBUF: %v", err)
	}

	// The kernel caps the value at rmem_max and doubles it for its bookkeeping.
	if want := 2 * rmemMax; got != want {
		t.Errorf("SO_RCVBUF = %d, want %d (2 x rmem_max %d)", got, want, rmemMax)
	}
}
