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

package kube

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestRPPEndpoints(t *testing.T) {
	const (
		ready       = `{"status":{"phase":"Running","podIP":"%s","conditions":[{"type":"Ready","status":"True"}]}}`
		notReady    = `{"status":{"phase":"Running","podIP":"%s","conditions":[{"type":"Ready","status":"False"}]}}`
		terminating = `{"metadata":{"deletionTimestamp":"2026-10-06T17:44:00Z"},"status":{"phase":"Running","podIP":"%s","conditions":[{"type":"Ready","status":"True"}]}}`
		pending     = `{"status":{"phase":"Pending","podIP":"%s"}}`
	)
	pod := func(format, ip string) string { return fmt.Sprintf(format, ip) }

	for name, tc := range map[string]struct {
		pods []string
		want []string
	}{
		"the ready ones while the rest start": {
			pods: []string{pod(ready, "10.0.0.1"), pod(notReady, "10.0.0.2")},
			want: []string{"10.0.0.1:4219"},
		},
		"running ones when none is ready": {
			pods: []string{pod(notReady, "10.0.0.2"), pod(notReady, "10.0.0.3")},
			want: []string{"10.0.0.2:4219", "10.0.0.3:4219"},
		},
		"never one being deleted": {
			pods: []string{pod(terminating, "10.0.0.1"), pod(notReady, "10.0.0.3")},
			want: []string{"10.0.0.3:4219"},
		},
		"nothing while the only ones are gone or not started": {
			pods: []string{pod(terminating, "10.0.0.1"), pod(pending, "10.0.0.2")},
			want: nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var response podListResponse
			if err := json.Unmarshal([]byte(`{"items":[`+strings.Join(tc.pods, ",")+`]}`), &response); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := rppEndpoints(response.Items); !slices.Equal(got, tc.want) {
				t.Fatalf("rppEndpoints() = %v, want %v", got, tc.want)
			}
		})
	}
}
