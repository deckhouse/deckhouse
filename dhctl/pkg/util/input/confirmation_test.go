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

package input

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInCI(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{value: "", want: false},
		{value: "false", want: false},
		{value: "0", want: false},
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "woodpecker", want: true},
	} {
		t.Run("CI="+tc.value, func(t *testing.T) {
			t.Setenv("CI", tc.value)
			require.Equal(t, tc.want, inCI())
		})
	}
}
