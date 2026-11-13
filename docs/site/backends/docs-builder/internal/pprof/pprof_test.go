// Copyright 2023 Flant JSC
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

//go:build pprof

package pprof

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// A development build serves the profiles inside the pod only.
func TestServerServesProfiles(t *testing.T) {
	srv := NewServer(log.NewNop())
	if srv.Addr != "127.0.0.1:6060" {
		t.Fatalf("address %q, want 127.0.0.1:6060", srv.Addr)
	}

	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/debug/pprof/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/debug/pprof/: %d, want 200", resp.StatusCode)
	}
}
