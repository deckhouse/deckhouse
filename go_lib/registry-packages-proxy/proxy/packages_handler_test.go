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

package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePackagesPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		url         string
		wantPackage string
		wantAction  packagesAction
		wantVersion string
		wantErr     bool
	}{
		{
			url:         "/v1/packages/my-package/metadata/icon/",
			wantPackage: "my-package",
			wantAction:  packagesMetadataActionGetIcon,
		},
		{
			url:         "/v1/packages/my-package/metadata/icon",
			wantPackage: "my-package",
			wantAction:  packagesMetadataActionGetIcon,
		},
		{
			url:         "/v1/packages/my-package/metadata/icon/v1.0.1",
			wantPackage: "my-package",
			wantAction:  packagesMetadataActionGetIcon,
			wantVersion: "1.0.1",
		},
		{url: "/v1/packages/", wantErr: true},
		{url: "/v1/packages/my-package", wantErr: true},
		{url: "/v1/packages/foo/bar/metadata/icon", wantErr: true},
		{url: "/v1/packages/my-package/unknown/action", wantErr: true},
		{url: "/v1/packages/my-package/metadata/icon/not-semver", wantErr: true},
		{url: "/v1/packages/my-package/metadata/icon/v1/2/3", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.url, func(t *testing.T) {
			t.Parallel()

			action, pkg, version, err := parsePackagesPath(tc.url)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantAction, action)
			assert.Equal(t, tc.wantPackage, pkg)
			assert.Equal(t, tc.wantVersion, version)
		})
	}
}
