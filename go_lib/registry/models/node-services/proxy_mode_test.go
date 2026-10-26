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

package nodeservices

import (
	"net/url"
	"testing"
)

func TestUpstreamRegistryURLHost(t *testing.T) {
	for _, tc := range []struct {
		host  string
		valid bool
	}{
		{"::A", false},
		{"::1", false},
		{"[::A]", true},
		{"[::A]:5000", true},
		{"registry.example.com", true},
		{"192.0.2.1:5000", true},
	} {
		t.Run(tc.host, func(t *testing.T) {
			upstream := UpstreamRegistry{Scheme: "http", Host: tc.host, Path: "/"}
			err := upstream.Validate()
			if !tc.valid {
				if err == nil {
					t.Fatal("accepted a host that cannot be safely used in remoteurl")
				}
				return
			}
			if err != nil {
				t.Fatalf("rejected a valid upstream: %v", err)
			}
			parsed, err := url.Parse(upstream.Scheme + "://" + upstream.Host)
			if err != nil || parsed.Host != tc.host || parsed.Hostname() == "" {
				t.Fatalf("remoteurl did not preserve the host: %v, %v", parsed, err)
			}
		})
	}
}
