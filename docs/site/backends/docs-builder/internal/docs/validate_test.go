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

package docs

import (
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

func TestValidateModuleName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"stronghold", false},
		{"cert-manager", false},
		{"user-authn", false},
		{"node-local-dns", false},
		{"m0dule1", false},
		{"", true},
		{"-leading", true},
		{"trailing-", true},
		{"Upper", true},
		{"with.dot", true},
		{"with/slash", true},
		{"..", true},
		{"../../../../app", true},
		{"../etc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateModuleName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateModuleName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestValidateChannels(t *testing.T) {
	tests := []struct {
		name     string
		channels []string
		wantErr  bool
	}{
		{"default stable", []string{"stable"}, false},
		{"all release channels", []string{"alpha", "beta", "early-access", "rock-solid", "stable"}, false},
		{"empty list", []string{}, true},
		{"nil list", nil, true},
		{"one empty channel", []string{"stable", ""}, true},
		{"traversal channel", []string{"../../../../../../app/hugo/content"}, true},
		{"dot in channel", []string{"a.b"}, true},
		{"slash in channel", []string{"a/b"}, true},
		{"one bad among good", []string{"stable", "../evil"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateChannels(tt.channels)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateChannels(%v) error = %v, wantErr %v", tt.channels, err, tt.wantErr)
			}
		})
	}
}

func TestEnsureWithinBase(t *testing.T) {
	const base = "/app/hugo"

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"base itself", "/app/hugo", false},
		{"child", "/app/hugo/content/modules/foo/stable", false},
		{"sibling with shared prefix", "/app/hugo-init/x", true},
		{"parent", "/app", true},
		{"escape via join", "/app/hugo/content/modules/../../../../tmp/evil", true},
		{"unrelated", "/tmp/evil", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureWithinBase(base, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ensureWithinBase(%q, %q) error = %v, wantErr %v", base, tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestUploadRejectsTraversal(t *testing.T) {
	svc := NewService("/app/hugo/", "", false, log.NewNop(), metricsstorage.NewMetricStorage())

	if err := svc.Upload(nil, "../../../../app", "v1.0.0", []string{"stable"}); err == nil {
		t.Error("Upload with traversal moduleName: expected error, got nil")
	}

	if err := svc.Upload(nil, "pwn", "v1.0.0", []string{"../../../../../../app/hugo/content"}); err == nil {
		t.Error("Upload with traversal channel: expected error, got nil")
	}
}

func TestDeleteRejectsTraversal(t *testing.T) {
	svc := NewService("/app/hugo/", "", false, log.NewNop(), metricsstorage.NewMetricStorage())

	if err := svc.Delete("../../../../app", []string{"stable"}); err == nil {
		t.Error("Delete with traversal moduleName: expected error, got nil")
	}

	if err := svc.Delete("pwn", []string{"../../../../../../app/hugo/content"}); err == nil {
		t.Error("Delete with traversal channel: expected error, got nil")
	}
}
