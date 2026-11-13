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

package accesslog

import (
	"context"
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestParse(t *testing.T) {
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
		module, version, ok := parse(tt.line)
		if module != tt.module || version != tt.version || ok != tt.ok {
			t.Errorf("%q: got %q %q %v", tt.line, module, version, ok)
		}
	}
}

// recorder keeps the accesses it is told of.
type recorder struct {
	mu       sync.Mutex
	accesses []string
}

func (r *recorder) Touch(module, version string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.accesses = append(r.accesses, module+"/"+version)
}

func (r *recorder) touched(k string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Contains(r.accesses, k)
}

func TestListenTouchesVersions(t *testing.T) {
	r := &recorder{}

	// Pick a free UDP port for the listener.
	probe, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.LocalAddr().String()
	probe.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Listen(ctx, addr, r, log.NewNop()) }()

	conn, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	deadline := time.Now().Add(5 * time.Second)
	for !r.touched("echo/v1.0.0") {
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
