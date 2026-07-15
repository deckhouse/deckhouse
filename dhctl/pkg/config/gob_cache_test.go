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

package config

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/stretchr/testify/require"

	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
)

// The state cache gob-encodes MetaConfig via SaveStruct. For mc-flow / DVP its
// CloudProviderVars carries a credential Secret's data as a map[string]string
// nested in interface{}, plus map[string]interface{} / []interface{} from
// JSON-decoded provider resources. gob refuses unregistered concrete types
// inside an interface, so destroy/converge previously failed with
// "gob: type not registered for interface: map[string]string". The config
// init() registers these; this round-trips the exact failing shape.
func TestGobEncodeCloudProviderVars(t *testing.T) {
	cv := &proto.CloudProviderVars{
		Settings: map[string]interface{}{
			"nested": map[string]interface{}{"list": []interface{}{"a", float64(1)}},
		},
		Secrets: map[string]map[string]interface{}{
			"d8-credentials": {
				"data": map[string]string{"secret": "eyJ", "authScheme": "kubeconfig"},
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(cv),
		"gob must encode CloudProviderVars carrying a map[string]string secret value")

	var out proto.CloudProviderVars
	require.NoError(t, gob.NewDecoder(&buf).Decode(&out))
	require.Equal(t, cv.Secrets, out.Secrets)
	require.Equal(t, cv.Settings, out.Settings)
}
