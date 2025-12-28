// Copyright 2025 Flant JSC
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

package specs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/statusmapper"
)

func TestReadySpec(t *testing.T) {
	spec := ReadySpec()

	tests := []struct {
		name           string
		runtimeStatus  metav1.ConditionStatus
		expectedStatus metav1.ConditionStatus
	}{
		{"ready", metav1.ConditionTrue, metav1.ConditionTrue},
		{"not ready", metav1.ConditionFalse, metav1.ConditionFalse},
		{"unknown", metav1.ConditionUnknown, metav1.ConditionFalse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &statusmapper.Input{
				InternalConditions: map[status.ConditionName]status.Condition{
					"ReadyInRuntime": {Name: "ReadyInRuntime", Status: tt.runtimeStatus},
				},
				ExternalConditions: make(map[status.ConditionName]status.Condition),
			}

			result := spec.Map(input)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectedStatus, result.Status)
		})
	}
}
