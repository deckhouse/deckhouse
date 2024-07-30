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

package mirror

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_gostsum_HashCompatibility(t *testing.T) {
	input := "012345678901234567890123456789012345678901234567890123456789012"
	gostsumHash := "9d151eefd8590b89daa6ba6cb74af9275dd051026bb149a452fd84e5e57b5500"

	gogostHash, err := CalculateBlobGostDigest(strings.NewReader(input))
	require.NoError(t, err)
	require.Equal(t, gostsumHash, gogostHash)
}
