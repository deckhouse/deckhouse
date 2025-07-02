/*
Copyright 2025 Flant JSC

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
package experimental

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestFilter(t *testing.T) {
	truePtr, falsePtr := ptr.To(true), ptr.To(false)

	cases := []struct {
		name   string
		flag   bool
		labels map[string]string
		want   *bool
		err    bool
	}{
		{"skip-non-experimental", false, map[string]string{"stage": "Stable"}, nil, false},
		{"deny-when-flag-false", false, map[string]string{"stage": "Experimental"}, falsePtr, true},
		{"allow-when-flag-true", true, map[string]string{"stage": "Experimental"}, truePtr, false},
		{"no-labels", false, nil, nil, false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			e := Instance()
			_ = e.AddConstraint("allowExperimentalModules", strconv.FormatBool(tt.flag))

			got, err := e.Filter("foo", tt.labels)

			if tt.err {
				require.Error(t, err)
				require.Equal(t, *tt.want, *got)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			}
		})
	}
}
