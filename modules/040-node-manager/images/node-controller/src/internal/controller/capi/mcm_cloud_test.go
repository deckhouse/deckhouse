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

package capi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func TestBlobZones(t *testing.T) {
	t.Run("extracts zones", func(t *testing.T) {
		blob := map[string]interface{}{
			"cloudInstances": map[string]interface{}{
				"zones": []interface{}{"eu-west-1a", "eu-west-1b"},
			},
		}
		assert.Equal(t, []string{"eu-west-1a", "eu-west-1b"}, blobZones(blob))
	})
	t.Run("no cloudInstances", func(t *testing.T) {
		assert.Nil(t, blobZones(map[string]interface{}{}))
	})
	t.Run("no zones key", func(t *testing.T) {
		assert.Nil(t, blobZones(map[string]interface{}{"cloudInstances": map[string]interface{}{}}))
	})
	t.Run("skips non-string entries", func(t *testing.T) {
		blob := map[string]interface{}{
			"cloudInstances": map[string]interface{}{
				"zones": []interface{}{"a", 5, "b"},
			},
		}
		assert.Equal(t, []string{"a", "b"}, blobZones(blob))
	})
}

func TestBlobInstanceClassSpot(t *testing.T) {
	t.Run("spot true", func(t *testing.T) {
		blob := map[string]interface{}{"instanceClass": map[string]interface{}{"spot": true}}
		assert.True(t, blobInstanceClassSpot(blob))
	})
	t.Run("spot false", func(t *testing.T) {
		blob := map[string]interface{}{"instanceClass": map[string]interface{}{"spot": false}}
		assert.False(t, blobInstanceClassSpot(blob))
	})
	t.Run("no spot key", func(t *testing.T) {
		blob := map[string]interface{}{"instanceClass": map[string]interface{}{}}
		assert.False(t, blobInstanceClassSpot(blob))
	})
	t.Run("no instanceClass", func(t *testing.T) {
		assert.False(t, blobInstanceClassSpot(map[string]interface{}{}))
	})
	t.Run("null instanceClass", func(t *testing.T) {
		assert.False(t, blobInstanceClassSpot(map[string]interface{}{"instanceClass": nil}))
	})
}

func TestDecodeCloudProviderSecret(t *testing.T) {
	data := map[string][]byte{
		"type":             []byte(`"aws"`),
		"region":           []byte(`"eu-west-1"`),
		"machineClassKind": []byte(`"AWSMachineClass"`),
		"aws":              []byte(`{"keyName":"kn","instances":{"ami":"ami-1"}}`),
		"plainString":      []byte(`not-json`),
	}
	tree := decodeCloudProviderSecret(data)
	assert.Equal(t, "aws", tree["type"])
	assert.Equal(t, "eu-west-1", tree["region"])
	assert.Equal(t, "AWSMachineClass", tree["machineClassKind"])
	aws, ok := tree["aws"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "kn", aws["keyName"])
	// Non-JSON values fall back to the raw string, matching decodeSecretData.
	assert.Equal(t, "not-json", tree["plainString"])
}

// TestReconcileCloudMCMs_NoCloudInstances exercises the earliest guard: a
// NodeGroup without cloudInstances returns nil before any kube access, so it runs
// without a client.
func TestReconcileCloudMCMs_NoCloudInstances(t *testing.T) {
	r := &MachineDeploymentReconciler{}
	ng := &deckhousev1.NodeGroup{}
	assert.NoError(t, r.reconcileCloudMCMs(context.Background(), ng))
}
