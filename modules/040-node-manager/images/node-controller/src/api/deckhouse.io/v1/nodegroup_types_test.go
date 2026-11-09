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

package v1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsDeckhouseEngine(t *testing.T) {
	for _, systemType := range []SystemType{"DeckhouseEngine", "Immutable"} {
		require.True(t, IsDeckhouseEngine(systemType), "%q names a Deckhouse Engine node", systemType)
	}
	for _, systemType := range []SystemType{"GenericOS", "Mutable", "", "Other"} {
		require.False(t, IsDeckhouseEngine(systemType), "%q does not name a Deckhouse Engine node", systemType)
	}
}
