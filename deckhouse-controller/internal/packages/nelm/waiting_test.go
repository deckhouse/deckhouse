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

package nelm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/werf/nelm/pkg/legacy/progrep"
)

func TestWaitingFor(t *testing.T) {
	resource := func(category progrep.OperationCategory, kind, name string, status progrep.OperationStatus) progrep.Operation {
		op := progrep.Operation{Category: category, Status: status}
		op.Kind = kind
		op.Name = name

		return op
	}

	// Stage boundaries and release record updates carry no resource.
	noResource := func(category progrep.OperationCategory, status progrep.OperationStatus) progrep.Operation {
		return progrep.Operation{Category: category, Status: status}
	}

	tests := []struct {
		name     string
		ops      []progrep.Operation
		expected []string
	}{
		{
			name:     "no operations",
			expected: []string{},
		},
		{
			name: "started operations hide the queued ones",
			ops: []progrep.Operation{
				resource(progrep.OperationCategoryResource, "ConfigMap", "done", progrep.OperationStatusCompleted),
				resource(progrep.OperationCategoryTrack, "Deployment", "app", progrep.OperationStatusProgressing),
				resource(progrep.OperationCategoryTrack, "Job", "migrate", progrep.OperationStatusFailed),
				resource(progrep.OperationCategoryResource, "Service", "app", progrep.OperationStatusPending),
			},
			expected: []string{"Deployment/app", "Job/migrate"},
		},
		{
			name: "queued operations are named when nothing has started",
			ops: []progrep.Operation{
				resource(progrep.OperationCategoryResource, "ConfigMap", "done", progrep.OperationStatusCompleted),
				resource(progrep.OperationCategoryResource, "Service", "app", progrep.OperationStatusPending),
			},
			expected: []string{"Service/app"},
		},
		{
			name: "a resource with several operations is named once",
			ops: []progrep.Operation{
				resource(progrep.OperationCategoryResource, "Deployment", "app", progrep.OperationStatusProgressing),
				resource(progrep.OperationCategoryTrack, "Deployment", "app", progrep.OperationStatusProgressing),
			},
			expected: []string{"Deployment/app"},
		},
		{
			name: "operations without a resource are not named",
			ops: []progrep.Operation{
				noResource(progrep.OperationCategoryMeta, progrep.OperationStatusProgressing),
				noResource(progrep.OperationCategoryRelease, progrep.OperationStatusPending),
				resource(progrep.OperationCategoryTrack, "Deployment", "app", progrep.OperationStatusProgressing),
				noResource(progrep.OperationCategoryMeta, progrep.OperationStatusPending),
			},
			expected: []string{"Deployment/app"},
		},
		{
			name: "only operations without a resource are left",
			ops: []progrep.Operation{
				noResource(progrep.OperationCategoryMeta, progrep.OperationStatusPending),
				noResource(progrep.OperationCategoryRelease, progrep.OperationStatusPending),
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, waitingFor(tt.ops))
		})
	}
}
