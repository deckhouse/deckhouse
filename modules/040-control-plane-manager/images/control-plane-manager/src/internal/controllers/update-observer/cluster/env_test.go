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

package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAvailable(t *testing.T) {
	full := VersionSettings{Supported: []string{"1.33", "1.34", "1.35", "1.36", "1.37"}}
	// CSE: only the first and the last version, the ones in between are upgrade hops.
	cse := VersionSettings{Supported: []string{"1.34", "1.36"}}

	tests := []struct {
		name     string
		settings VersionSettings
		maxUsed  string
		want     []string
	}{
		{"full, maxUsed in the middle", full, "1.35", []string{"1.34", "1.35", "1.36", "1.37"}},
		{"full, maxUsed is the oldest", full, "1.33", []string{"1.33", "1.34", "1.35", "1.36", "1.37"}},
		{"full, maxUsed older than everything", full, "1.30", full.Supported},
		{"full, maxUsed newer than everything", full, "1.39", []string{}},
		{"cse, maxUsed is the first", cse, "1.34", []string{"1.34", "1.36"}},
		{"cse, maxUsed is the hop", cse, "1.35", []string{"1.34", "1.36"}},
		{"cse, maxUsed is the last", cse, "1.36", []string{"1.36"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.settings.Available(tt.maxUsed))
		})
	}
}
