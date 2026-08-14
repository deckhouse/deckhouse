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

package dhregistry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	dhregistry "github.com/deckhouse/deckhouse/pkg/deckhouse-registry"
)

func TestChannelIsValid(t *testing.T) {
	for _, c := range dhregistry.Channels() {
		assert.True(t, c.IsValid(), c)
	}

	assert.False(t, dhregistry.Channel("").IsValid())
	assert.False(t, dhregistry.Channel("v1.73.0").IsValid())
	assert.Equal(t, "early-access", dhregistry.EarlyAccessChannel.String())
	assert.Len(t, dhregistry.Channels(), 6)
}

// TestIsChannel covers telling channel tags apart from version tags, which is
// how a release repository's tag list is filtered.
func TestIsChannel(t *testing.T) {
	for _, tag := range []string{"alpha", "beta", "early-access", "stable", "rock-solid", "lts", "STABLE"} {
		assert.True(t, dhregistry.IsChannel(tag), tag)
	}

	for _, tag := range []string{"", "v1.73.0", "1.73", "latest", "sha256:abc"} {
		assert.False(t, dhregistry.IsChannel(tag), tag)
	}
}

func TestChannelsIsACopy(t *testing.T) {
	all := dhregistry.Channels()
	all[0] = "mutated"

	assert.Equal(t, dhregistry.AlphaChannel, dhregistry.Channels()[0])
}
