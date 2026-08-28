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

package imagefs

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafeJoin(t *testing.T) {
	const root = "/deckhouse/downloaded/modules/some-module/v1.0.0"

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "plain entry", path: "openapi/values.yaml"},
		{name: "leading dot slash", path: "./openapi/values.yaml"},
		{name: "dots inside a filename", path: "..gitkeep"},
		{name: "cleans back into the root", path: "charts/../values.yaml"},
		{name: "absolute name is re-rooted", path: "/etc/passwd"},
		{name: "escapes one level up", path: "..", wantErr: true},
		{name: "escapes the root", path: "../../victim.txt", wantErr: true},
		{name: "escapes after a descent", path: "charts/../../../victim.txt", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SafeJoin(root, tt.path)
			if tt.wantErr {
				require.ErrorContains(t, err, "path traversal detected")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, filepath.Join(root, tt.path), got)
		})
	}
}
