// Copyright 2021 Flant JSC
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

package maputil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExcludeKeys(t *testing.T) {
	cases := []struct {
		name     string
		mp       map[string]string
		excluded []string
		res      map[string]string
	}{
		{
			name:     "Empty map and empty keys returns empty map",
			mp:       make(map[string]string),
			excluded: make([]string, 0),
			res:      make(map[string]string),
		},

		{
			name:     "Not empty map and empty keys returns map with all keys",
			mp:       map[string]string{"k": "v"},
			excluded: make([]string, 0),
			res:      map[string]string{"k": "v"},
		},

		{
			name:     "Empty map and not empty keys return empty map",
			mp:       make(map[string]string),
			excluded: []string{"k"},
			res:      make(map[string]string),
		},

		{
			name: "Exclude one key",
			mp: map[string]string{
				"k1": "v1",
				"k2": "v2",
				"k3": "v3",
			},
			excluded: []string{"k2"},
			res: map[string]string{
				"k1": "v1",
				"k3": "v3",
			},
		},

		{
			name: "Exclude multiple keys, but one key is not in map. Must exclude all exists keys",
			mp: map[string]string{
				"k1": "v1",
				"k2": "v2",
				"k3": "v3",
			},
			excluded: []string{"k2", "m1"},
			res: map[string]string{
				"k1": "v1",
				"k3": "v3",
			},
		},

		{
			name: "Exclude all keys",
			mp: map[string]string{
				"k1": "v1",
				"k2": "v2",
				"k3": "v3",
			},
			excluded: []string{"k1", "k2", "k3", "m1"},
			res:      make(map[string]string),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := ExcludeKeys(c.mp, c.excluded...)

			require.Equal(t, res, c.res)
		})
	}
}
