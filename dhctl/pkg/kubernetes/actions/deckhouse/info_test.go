// Copyright 2021 Flant CJSC
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
	"testing"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func TestGetClusterInfo(t *testing.T) {
	log.InitLogger("simple")

	tests := []struct {
		name           string
		configMapData  string
		expectedResult string
	}{
		{
			"With proper data",
			`
global:
  clusterName: test
  project: projectTest
`,
			"Cluster:\ttest\nProject:\tprojectTest\n",
		},
		{
			"Without global",
			`
testModule:
  test: test
`,
			"Cluster:\t\nProject:\t\n",
		},
		{
			"Without ConfigMap",
			``,
			"",
		},
	}

	for _, tc := range tests {
		fakeClient := client.NewFakeKubernetesClient()

		var data map[string]interface{}
		err := yaml.Unmarshal([]byte(tc.configMapData), &data)
		if err != nil {
			t.Fatalf("%s: Unexpected error: %v", tc.name, err)
		}

		task := actions.ManifestTask{
			Name: `ConfigMap "deckhouse"`,
			Manifest: func() interface{} {
				if tc.configMapData == "" {
					return &apiv1.ConfigMap{}
				}
				return manifests.DeckhouseConfigMap(data)
			},
			CreateFunc: func(manifest interface{}) error {
				_, err := fakeClient.CoreV1().ConfigMaps("d8-system").Create(context.TODO(), manifest.(*apiv1.ConfigMap), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := fakeClient.CoreV1().ConfigMaps("d8-system").Update(context.TODO(), manifest.(*apiv1.ConfigMap), metav1.UpdateOptions{})
				return err
			},
		}
		if err := task.CreateOrUpdate(); err != nil {
			t.Fatalf("%s: Unexpected error: %v", tc.name, err)
		}

		result := GetClusterInfo(fakeClient)
		if tc.expectedResult != result {
			t.Fatalf("%s: %s\n!=\n%s", tc.name, tc.expectedResult, result)
		}
	}
}
