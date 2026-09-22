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

package main

import (
	"os"
	"testing"
)

func TestNodeName(t *testing.T) {
	t.Run("takes the name from the downward API", func(t *testing.T) {
		t.Setenv("NODE_NAME", "worker-rack3-07")

		got, err := nodeName()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "worker-rack3-07" {
			t.Fatalf("got %q, want the name from NODE_NAME", got)
		}
	})

	t.Run("trims what the downward API gives", func(t *testing.T) {
		t.Setenv("NODE_NAME", " worker-rack3-07\n")

		got, err := nodeName()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "worker-rack3-07" {
			t.Fatalf("got %q, want it trimmed", got)
		}
	})

	// A pod whose manifest predates the NODE_NAME env keeps working the way it
	// always did, on a cluster where the node name and the hostname agree.
	t.Run("falls back to the hostname when the env is absent", func(t *testing.T) {
		t.Setenv("NODE_NAME", "")

		hostname, err := os.Hostname()
		if err != nil {
			t.Skipf("no hostname to compare against: %v", err)
		}

		got, err := nodeName()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != hostname {
			t.Fatalf("got %q, want the hostname %q", got, hostname)
		}
	})
}
