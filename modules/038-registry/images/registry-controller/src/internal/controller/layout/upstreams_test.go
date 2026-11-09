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

package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/go_lib/registry/apis/upstreams"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// The arbitration spells the in-cluster endpoint out, because its module keeps no
// dependency on go_lib/registry. This package imports both, so it is where they meet.
func TestArbitrationReservesTheInClusterEndpoint(t *testing.T) {
	assert.Equal(t, constant.Host, upstreams.InClusterHost)
}
