// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ondemand

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestParseAccess(t *testing.T) {
	tests := []struct {
		line            string
		module, version string
		ok              bool
	}{
		// As nginx sends it: access_log syslog:...,nohostname,tag=docs_ondemand
		{"<190>Nov  2 13:22:22 docs_ondemand: /en/modules/echo/v1.2.3/readme.html", "echo", "v1.2.3", true},
		{"<190>Nov 12 01:02:03 docs_ondemand: /ru/modules/virtualization/v1.13.0-rc.1/faq/readme.html\n", "virtualization", "v1.13.0-rc.1", true},
		{"", "", "", false},
		{"<190>Nov  2 13:22:22 docs_ondemand: /en/modules/echo/", "", "", false},
		{"<190>Nov  2 13:22:22 docs_ondemand: /en/other/echo/v1.2.3/x.html", "", "", false},
	}

	for _, tt := range tests {
		module, version, ok := parseAccess(tt.line)
		if module != tt.module || version != tt.version || ok != tt.ok {
			t.Errorf("%q: got %q %q %v", tt.line, module, version, ok)
		}
	}
}

func TestListenAccessLogTouchesVersions(t *testing.T) {
	f := &fakeRender{size: 10}
	s, c := newTestStore(t, f, 1000)

	renderDone(t, s, "echo", "v1.0.0")
	c.add(time.Minute)

	// Pick a free UDP port for the listener.
	probe, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.LocalAddr().String()
	probe.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- ListenAccessLog(ctx, addr, s, log.NewNop()) }()

	conn, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	touched := func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.versions["echo/v1.0.0"].lastAccess.Equal(c.now())
	}

	deadline := time.Now().Add(5 * time.Second)
	for !touched() {
		if time.Now().After(deadline) {
			t.Fatal("the access was not recorded")
		}
		// The listener may not be up yet: datagrams sent before it are lost
		// and the write may report the refused port.
		_, _ = conn.Write([]byte("<190>Nov  2 13:22:22 docs_ondemand: /en/modules/echo/v1.0.0/readme.html"))
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("listener stopped with %v", err)
	}
}
