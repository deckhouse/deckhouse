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
	"github.com/deckhouse/node-controller/internal/cloudprovider"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
)

func TestInstanceClassSpot(t *testing.T) {
	t.Run("spot true", func(t *testing.T) {
		resolved := derived_status.ResolvedNodeGroup{InstanceClass: map[string]interface{}{"spot": true}}
		assert.True(t, instanceClassSpot(resolved))
	})
	t.Run("spot false", func(t *testing.T) {
		resolved := derived_status.ResolvedNodeGroup{InstanceClass: map[string]interface{}{"spot": false}}
		assert.False(t, instanceClassSpot(resolved))
	})
	t.Run("no spot key", func(t *testing.T) {
		resolved := derived_status.ResolvedNodeGroup{InstanceClass: map[string]interface{}{}}
		assert.False(t, instanceClassSpot(resolved))
	})
	t.Run("no instanceClass", func(t *testing.T) {
		assert.False(t, instanceClassSpot(derived_status.ResolvedNodeGroup{}))
	})
	t.Run("instanceClass resolved to null", func(t *testing.T) {
		assert.False(t, instanceClassSpot(derived_status.ResolvedNodeGroup{CloudProcessed: true, InstanceClass: nil}))
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
	tree := cloudprovider.Decode(data).Data
	assert.Equal(t, "aws", tree["type"])
	assert.Equal(t, "eu-west-1", tree["region"])
	assert.Equal(t, "AWSMachineClass", tree["machineClassKind"])
	aws, ok := tree["aws"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "kn", aws["keyName"])
	// Non-JSON values fall back to the raw string, matching decodeSecretData.
	assert.Equal(t, "not-json", tree["plainString"])
}

func TestReconcileCloudMCMs_NoCloudInstances(t *testing.T) {
	r := &MachineDeploymentReconciler{}
	ng := &deckhousev1.NodeGroup{}
	assert.NoError(t, r.reconcileCloudMCMs(context.Background(), ng, cloudprovider.Providers{}, cloudprovider.Provider{}))
}
