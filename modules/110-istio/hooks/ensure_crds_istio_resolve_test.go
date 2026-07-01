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

package hooks

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"
)

func TestResolveCRDBundleVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		localMax            string
		multiclusterEnabled bool
		want                string
	}{
		{
			name:                "multicluster off keeps base bundle",
			localMax:            "1.21",
			multiclusterEnabled: false,
			want:                "1.21",
		},
		{
			name:                "multicluster on legacy version uses compat bundle",
			localMax:            "1.21",
			multiclusterEnabled: true,
			want:                "1.21-mesh-compat",
		},
		{
			name:                "multicluster on modern version keeps base bundle",
			localMax:            "1.25",
			multiclusterEnabled: true,
			want:                "1.25",
		},
		{
			name:                "multicluster on 1.27 keeps base bundle",
			localMax:            "1.27",
			multiclusterEnabled: true,
			want:                "1.27",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			localMax := semver.MustParse(tt.localMax)
			got := resolveCRDBundleVersion(localMax, tt.multiclusterEnabled)
			require.Equal(t, tt.want, got)
		})
	}
}
