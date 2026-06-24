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

package takeover

import (
	"testing"
	"time"
)

func TestComputeVerifyReady(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	dsOK := &AgentDSStatus{NumberReady: 3, DesiredNumberScheduled: 3}
	stsOK := &CacheSTSStatus{ReadyReplicas: 3, Replicas: 3}
	leaseOK := &CacheLeaseStatus{Holder: "pod-0", RenewTime: now.Add(-5 * time.Second), LeaseDurationSeconds: 15}

	cases := []struct {
		name          string
		ds            *AgentDSStatus
		sts           *CacheSTSStatus
		lease         *CacheLeaseStatus
		cacheExpected bool
		airgap        bool
		storeSynced   bool
		want          bool
	}{
		// Existing cases — not air-gap, storeSynced irrelevant.
		{"cache on: all ready", dsOK, stsOK, leaseOK, true, false, false, true},
		{"cache off: agent only ready", dsOK, nil, nil, false, false, false, true},
		{"agent not fully ready", &AgentDSStatus{NumberReady: 2, DesiredNumberScheduled: 3}, stsOK, leaseOK, true, false, false, false},
		{"agent desired zero", &AgentDSStatus{NumberReady: 0, DesiredNumberScheduled: 0}, stsOK, leaseOK, true, false, false, false},
		{"agent nil", nil, stsOK, leaseOK, true, false, false, false},
		{"cache on but sts missing", dsOK, nil, leaseOK, true, false, false, false},
		{"cache on but replicas not all ready", dsOK, &CacheSTSStatus{ReadyReplicas: 2, Replicas: 3}, leaseOK, true, false, false, false},
		{"cache on but no leader", dsOK, stsOK, &CacheLeaseStatus{Holder: ""}, true, false, false, false},
		{"cache on but lease expired", dsOK, stsOK, &CacheLeaseStatus{Holder: "pod-0", RenewTime: now.Add(-60 * time.Second), LeaseDurationSeconds: 15}, true, false, false, false},
		{"cache off ignores stale cache state", dsOK, &CacheSTSStatus{ReadyReplicas: 0, Replicas: 3}, nil, false, false, false, true},
		// Air-gap cases.
		{"airgap: cache ready agent ready storeSynced false → blocked", dsOK, stsOK, leaseOK, true, true, false, false},
		{"airgap: all ready storeSynced true → allowed", dsOK, stsOK, leaseOK, true, true, true, true},
		// Non-air-gap (Proxy/Direct): storeSynced=false must not block.
		{"non-airgap: ready storeSynced false → allowed", dsOK, stsOK, leaseOK, true, false, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := computeVerifyReady(tc.ds, tc.sts, tc.lease, tc.cacheExpected, tc.airgap, tc.storeSynced, now); got != tc.want {
				t.Errorf("computeVerifyReady = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsAirgap(t *testing.T) {
	upstreamSet := &DerivedUpstream{Host: "registry.example.com"}
	cases := []struct {
		name    string
		derived *DerivedConfig
		want    bool
	}{
		{"nil derived → false", nil, false},
		{"Present false → false", &DerivedConfig{Present: false}, false},
		{"Present true Upstream nil → true (air-gap Local)", &DerivedConfig{Present: true, Upstream: nil}, true},
		{"Present true Upstream set → false (Proxy/Direct)", &DerivedConfig{Present: true, Upstream: upstreamSet}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAirgap(tc.derived); got != tc.want {
				t.Errorf("isAirgap = %v, want %v", got, tc.want)
			}
		})
	}
}
