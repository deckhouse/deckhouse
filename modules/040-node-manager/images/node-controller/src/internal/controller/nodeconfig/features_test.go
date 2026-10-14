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

package nodeconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The agent publishes status.features (nodelet, controllers.ActiveFeatures); a
// CRD without the field would prune it, and the console would be back to an
// empty spec.featureGates on every Engine node.
func TestShippedCRDKeepsTheFeaturesTheAgentReports(t *testing.T) {
	schema := nodeConfigCRDSchema(t)

	features := crdField(t, schema, "status", "features")
	require.Equal(t, "array", features["type"])
	require.Equal(t, "string", crdItems(t, features)["type"])
}
