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

package app

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"kubernetes-api-proxy/internal/upstream"
)

type fakeChecker struct{}

func (fakeChecker) Healthy() (bool, error) { return true, nil }

func (fakeChecker) Nodes() ([]upstream.ExportNode, error) {
	ago := int64(300)

	return []upstream.ExportNode{{Upstream: "a", LastReadyAgoMs: &ago}, {Upstream: "b"}}, nil
}

func (fakeChecker) Probe() (uint64, bool) { return 42, true }

func TestUpstreams_ProbeFields(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHealthServer("", fakeChecker{}).Handler.ServeHTTP(rec, httptest.NewRequest("GET", "/upstreams", nil))

	var body struct {
		IsHealthy bool             `json:"isHealthy"`
		ProbeSeq  uint64           `json:"probeSeq"`
		ProbeOK   bool             `json:"probeOK"`
		Nodes     []map[string]any `json:"nodes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}

	if !body.IsHealthy || body.ProbeSeq != 42 || !body.ProbeOK || len(body.Nodes) != 2 {
		t.Fatalf("body = %s, want isHealthy, probeSeq 42, probeOK and two nodes", rec.Body)
	}

	if body.Nodes[0]["lastReadyAgoMs"] != 300.0 {
		t.Errorf("node a = %v, want lastReadyAgoMs 300", body.Nodes[0])
	}

	if _, ok := body.Nodes[1]["lastReadyAgoMs"]; ok {
		t.Errorf("node b = %v, want no lastReadyAgoMs", body.Nodes[1])
	}
}

type unhealthyChecker struct{ fakeChecker }

func (unhealthyChecker) Healthy() (bool, error) { return false, nil }

func (unhealthyChecker) Probe() (uint64, bool) { return 42, false }

// /healthz, /readyz and /upstreams answer 200 also when no upstream is healthy.
func TestHealthEndpoints_AlwaysOK(t *testing.T) {
	for _, path := range []string{"/healthz", "/readyz", "/upstreams"} {
		rec := httptest.NewRecorder()
		NewHealthServer("", unhealthyChecker{}).Handler.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))

		if rec.Code != 200 {
			t.Errorf("%s = %d, want 200 with no healthy upstream", path, rec.Code)
		}
	}
}
