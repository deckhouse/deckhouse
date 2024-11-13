// Copyright 2024 Flant JSC
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

package time

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDuration_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		hasError bool
	}{
		{`"2h45m"`, 2*time.Hour + 45*time.Minute, false},
		{`"30m"`, 30 * time.Minute, false},
		{`"invalid"`, 0, true},
		{`""`, 0, true},
		{`"1h30m45s500ms"`, 1*time.Hour + 30*time.Minute + 45*time.Second + 500*time.Millisecond, false},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			var d Duration
			err := json.Unmarshal([]byte(test.input), &d)

			if test.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, d.Duration)
			}
		})
	}
}
