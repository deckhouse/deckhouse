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

package upstream

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestList_NewList(t *testing.T) {
	tests := []struct {
		name    string
		options []ListOption
		wantErr bool
	}{
		{
			name:    "valid options",
			options: []ListOption{WithHealthcheckInterval(2 * time.Second)},
			wantErr: false,
		},
		{
			name:    "invalid jitter",
			options: []ListOption{WithHealthCheckJitter(1.5)},
			wantErr: true,
		},
		{
			name:    "invalid jitter range",
			options: []ListOption{WithHealthCheckJitter(-0.5)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewList([]*Upstream{}, tt.options...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewList() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestList_Pick(t *testing.T) {
	// Create a test server for health checks
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/readyz" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	t.Run("Pick with no upstreams", func(t *testing.T) {
		list, err := NewList([]*Upstream{})
		if err != nil {
			t.Fatalf("NewList() error = %v", err)
		}
		defer list.Shutdown()

		backend, err := list.Pick()
		if err == nil {
			t.Error("Expected error for no upstreams")
		}
		if backend != nil {
			t.Error("Expected nil backend")
		}
	})
}
