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

package providerdata

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderDirLowercasesProvider(t *testing.T) {
	require.Equal(t, "/tmp/dl/dvp", ProviderDir("/tmp/dl", "DVP"))
	require.Equal(t, "/tmp/dl/yandex", ProviderDir("/tmp/dl", "yandex"))
}

func TestProviderDigestDir(t *testing.T) {
	require.Equal(t, "/tmp/dl/dvp@sha256:abc", ProviderDigestDir("/tmp/dl", "DVP", "sha256:abc"))
}

func TestValidatorPath(t *testing.T) {
	require.Equal(t, "/tmp/dl/dvp/validator", ValidatorPath("/tmp/dl", "Dvp"))
}
