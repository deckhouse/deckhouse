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

package vmresource

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		name string
		rule *Rule
		rc   plan.ResourceChange
		want bool
	}{
		{
			name: "type match without fieldEquals",
			rule: &Rule{Type: "yandex_compute_instance"},
			rc:   plan.ResourceChange{Type: "yandex_compute_instance"},
			want: true,
		},
		{
			name: "type mismatch without fieldEquals",
			rule: &Rule{Type: "yandex_compute_instance"},
			rc:   plan.ResourceChange{Type: "yandex_compute_disk"},
			want: false,
		},
		{
			name: "type match plus fieldEquals match",
			rule: &Rule{
				Type:        "kubernetes_manifest",
				FieldEquals: &FieldEquals{Path: "manifest.kind", Value: "VirtualMachine"},
			},
			rc: plan.ResourceChange{
				Type: "kubernetes_manifest",
				Change: plan.ChangeOp{After: map[string]interface{}{
					"manifest": map[string]interface{}{"kind": "VirtualMachine"},
				}},
			},
			want: true,
		},
		{
			name: "type match but fieldEquals value differs",
			rule: &Rule{
				Type:        "kubernetes_manifest",
				FieldEquals: &FieldEquals{Path: "manifest.kind", Value: "VirtualMachine"},
			},
			rc: plan.ResourceChange{
				Type: "kubernetes_manifest",
				Change: plan.ChangeOp{After: map[string]interface{}{
					"manifest": map[string]interface{}{"kind": "Service"},
				}},
			},
			want: false,
		},
		{
			name: "type match but fieldEquals path missing",
			rule: &Rule{
				Type:        "kubernetes_manifest",
				FieldEquals: &FieldEquals{Path: "manifest.kind", Value: "VirtualMachine"},
			},
			rc: plan.ResourceChange{
				Type:   "kubernetes_manifest",
				Change: plan.ChangeOp{After: map[string]interface{}{"spec": "anything"}},
			},
			want: false,
		},
		{
			name: "type mismatch ignores fieldEquals",
			rule: &Rule{
				Type:        "kubernetes_manifest",
				FieldEquals: &FieldEquals{Path: "manifest.kind", Value: "VirtualMachine"},
			},
			rc: plan.ResourceChange{
				Type: "yandex_compute_instance",
				Change: plan.ChangeOp{After: map[string]interface{}{
					"manifest": map[string]interface{}{"kind": "VirtualMachine"},
				}},
			},
			want: false,
		},
		{
			name: "fieldEquals path value is not a string",
			rule: &Rule{
				Type:        "kubernetes_manifest",
				FieldEquals: &FieldEquals{Path: "manifest.kind", Value: "VirtualMachine"},
			},
			rc: plan.ResourceChange{
				Type: "kubernetes_manifest",
				Change: plan.ChangeOp{After: map[string]interface{}{
					"manifest": map[string]interface{}{"kind": 42},
				}},
			},
			want: false,
		},
		{
			name: "fieldEquals path traverses non-map intermediate",
			rule: &Rule{
				Type:        "kubernetes_manifest",
				FieldEquals: &FieldEquals{Path: "manifest.kind", Value: "VirtualMachine"},
			},
			rc: plan.ResourceChange{
				Type:   "kubernetes_manifest",
				Change: plan.ChangeOp{After: map[string]interface{}{"manifest": "not-a-map"}},
			},
			want: false,
		},
		{
			name: "nil rule returns false",
			rule: nil,
			rc:   plan.ResourceChange{Type: "yandex_compute_instance"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, Match(tt.rc, tt.rule))
		})
	}
}
