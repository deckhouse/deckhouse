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

package loadbalancer

import (
	"testing"
	"time"
)

func TestServeRoute_RejectsNegativeTimeouts(t *testing.T) {
	tests := []struct {
		name string
		b    Balancer
	}{
		{name: "first byte", b: Balancer{FirstByteTimeout: -time.Second}},
		{name: "dial", b: Balancer{DialTimeout: -time.Second}},
		{name: "keepalive", b: Balancer{KeepAlivePeriod: -time.Second}},
		{name: "tcp user timeout", b: Balancer{TCPUserTimeout: -time.Second}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.b.ServeRoute("127.0.0.1:0"); err == nil {
				t.Error("ServeRoute() accepted a negative timeout")
			}
		})
	}
}

func TestNewTarget_ZeroBalancer(t *testing.T) {
	target := (&Balancer{}).newTarget("route")

	if target.list != nil {
		t.Errorf("list = %#v, want a nil interface", target.list)
	}

	if target.logger == nil {
		t.Error("logger is nil, want a no-op logger")
	}

	if target.firstByte != nil {
		t.Error("first-byte budget set with FirstByteTimeout 0")
	}
}

func TestNewTarget_FirstByteBudget(t *testing.T) {
	target := (&Balancer{FirstByteTimeout: 3 * time.Second}).newTarget("route")

	if target.firstByte == nil || target.firstByte.Wait("a") != 3*time.Second {
		t.Errorf("first-byte budget = %v, want a budget with a 3s floor", target.firstByte)
	}
}

func TestPick_ZeroBalancerFallsToEnv(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "1")

	backend, addr := (&Balancer{}).newTarget("route").pick()
	if backend != nil || addr != "127.0.0.1:1" {
		t.Errorf("pick() = %v, %q; want nil, 127.0.0.1:1", backend, addr)
	}
}
