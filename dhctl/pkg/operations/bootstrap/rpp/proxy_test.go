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

package rpp

import "testing"

func TestReverseTunnelCheckURL(t *testing.T) {
	cases := []struct {
		name string
		port string
		want string
	}{
		{
			name: "http healthz for registry packages proxy",
			port: "5444",
			want: "http://127.0.0.1:5444/healthz",
		},
		{
			name: "http reachable for rpp-get",
			port: "4282",
			want: "http://127.0.0.1:4282/healthz",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := reverseTunnelCheckURL(localhost, c.port)
			if got != c.want {
				t.Fatalf(
					"reverseTunnelCheckURL(%q, %q) = %q, want %q",
					localhost,
					c.port,
					got,
					c.want,
				)
			}
		})
	}
}
