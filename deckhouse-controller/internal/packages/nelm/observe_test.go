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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/werf/nelm/pkg/legacy/progrep"
)

func TestTracking(t *testing.T) {
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

	t.Run("an empty report does not replace the progress", func(t *testing.T) {
		_, ok := tracking(progrep.ProgressReport{})
		assert.False(t, ok)
	})

	t.Run("the raw report is kept and only resource operations are counted", func(t *testing.T) {
		report := progrep.ProgressReport{Operations: []progrep.Operation{
			resource(progrep.OperationCategoryResource, "ConfigMap", "done", progrep.OperationStatusCompleted),
			resource(progrep.OperationCategoryTrack, "Deployment", "app", progrep.OperationStatusProgressing),
			resource(progrep.OperationCategoryResource, "Service", "app", progrep.OperationStatusPending),
			noResource(progrep.OperationCategoryMeta, progrep.OperationStatusCompleted),
		}}

		got, ok := tracking(report)
		require.True(t, ok)

		raw, err := json.Marshal(report)
		require.NoError(t, err)

		assert.Equal(t, 1, got.Completed)
		assert.Equal(t, 2, got.Remaining)
		assert.JSONEq(t, string(raw), string(got.Report))
	})

	t.Run("a report of non-resource operations is still recorded", func(t *testing.T) {
		got, ok := tracking(progrep.ProgressReport{Operations: []progrep.Operation{
			noResource(progrep.OperationCategoryRelease, progrep.OperationStatusProgressing),
		}})
		require.True(t, ok)

		assert.Zero(t, got.Completed)
		assert.Zero(t, got.Remaining)
		assert.NotEmpty(t, got.Report)
	})
}
