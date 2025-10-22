// Copyright 2023 Flant JSC
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

package registryclient

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadAuthConfig(t *testing.T) {
	authraw := `
{
  "auths": {
    "registry-1.deckhouse.io": {
      "auth": "YTpiCg=="
    },
    "registry-2.deckhouse.io": {
      "auth": "YTpiCg=="
    }
  }
}
`

	_, err := readAuthConfig("registry-1.deckhouse.io/module/foo/bar", authraw)
	require.NoError(t, err)

	_, err = readAuthConfig("registry-invalid.deckhouse.io/module/foo/bar", authraw)
	require.Error(t, err)
}
