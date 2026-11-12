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

package objectpatcher

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

// The readiness hook of an older module-sdk patches the IsReady condition of its v1alpha1 Module:
// the patch never reaches the API, and IsReady=False becomes the error of the run.
func TestModuleStatusPatchIsAVerdict(t *testing.T) {
	module := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "Module",
		"metadata":   map[string]any{"name": "hello"},
	}}
	cli := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{v1alpha1.ModuleGVR: "ModuleList"}, module)
	modules := moduleStatusDynamic{Interface: cli}.Resource(v1alpha1.ModuleGVR).Namespace("")

	patch := func(body string, subresources ...string) error {
		_, err := modules.Patch(context.Background(), "hello", types.MergePatchType, []byte(body), metav1.PatchOptions{}, subresources...)
		return err
	}

	err := patch(`{"status":{"phase":"Reconciling","conditions":[{"type":"IsReady","status":"False","message":"database is down"}]}}`, "/status")
	var failed *hooks.ProbeFailedError
	require.ErrorAs(t, err, &failed)
	assert.Equal(t, "database is down", failed.Message)

	require.NoError(t, patch(`{"status":{"phase":"Ready","conditions":[{"type":"IsReady","status":"True"}]}}`, "/status"))
	assert.Empty(t, cli.Actions(), "a status patch never reaches the API")

	require.NoError(t, patch(`{"metadata":{"labels":{"a":"b"}}}`))
	assert.Len(t, cli.Actions(), 1, "a patch of anything but the status goes through")
}
