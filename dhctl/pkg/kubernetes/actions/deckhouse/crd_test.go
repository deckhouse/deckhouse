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

package deckhouse

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const testModuleConfigCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: moduleconfigs.deckhouse.io
spec:
  group: deckhouse.io
  names:
    kind: ModuleConfig
    listKind: ModuleConfigList
    plural: moduleconfigs
    singular: moduleconfig
  scope: Cluster
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
`

var crdGVR = schema.GroupVersionResource{
	Group:    "apiextensions.k8s.io",
	Version:  "v1",
	Resource: "customresourcedefinitions",
}

func TestEnsureModuleConfigCRD(t *testing.T) {
	ctx := context.Background()
	log.InitLogger("json", false)

	t.Run("creates CRD with heritage label from existing file", func(t *testing.T) {
		crdPath := filepath.Join(t.TempDir(), "module-config.yaml")
		require.NoError(t, os.WriteFile(crdPath, []byte(testModuleConfigCRD), 0o600))

		fakeClient := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
			crdGVR: "CustomResourceDefinitionList",
		})

		err := EnsureModuleConfigCRD(ctx, fakeClient, crdPath)
		require.NoError(t, err)

		crd, err := fakeClient.Dynamic().Resource(crdGVR).
			Get(ctx, "moduleconfigs.deckhouse.io", metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "deckhouse", crd.GetLabels()["heritage"])
	})

	t.Run("missing file is not an error and creates nothing", func(t *testing.T) {
		fakeClient := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
			crdGVR: "CustomResourceDefinitionList",
		})

		err := EnsureModuleConfigCRD(ctx, fakeClient, filepath.Join(t.TempDir(), "absent.yaml"))
		require.NoError(t, err)

		_, err = fakeClient.Dynamic().Resource(crdGVR).
			Get(ctx, "moduleconfigs.deckhouse.io", metav1.GetOptions{})
		require.Error(t, err)
	})

	t.Run("empty path is not an error", func(t *testing.T) {
		fakeClient := client.NewFakeKubernetesClientWithListGVR(nil)

		require.NoError(t, EnsureModuleConfigCRD(ctx, fakeClient, ""))
	})

	t.Run("broken file returns error", func(t *testing.T) {
		crdPath := filepath.Join(t.TempDir(), "module-config.yaml")
		require.NoError(t, os.WriteFile(crdPath, []byte("{not yaml: ["), 0o600))

		fakeClient := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
			crdGVR: "CustomResourceDefinitionList",
		})

		require.Error(t, EnsureModuleConfigCRD(ctx, fakeClient, crdPath))
	})
}
