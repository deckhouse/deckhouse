// Copyright 2024 Flant JSC
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

package source

import (
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/releaseutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

func TestGetReleasePolicy(t *testing.T) {
	embeddedDeckhousePolicy := &v1alpha1.ModuleUpdatePolicySpec{
		Update: v1alpha1.ModuleUpdatePolicySpecUpdate{
			Mode: "Auto",
		},
		ReleaseChannel: "Stable",
	}
	sc := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(sc)
	cl := fake.NewClientBuilder().WithScheme(sc).WithStatusSubresource(&v1alpha1.ModuleSource{}).Build()

	c := &moduleSourceReconciler{
		client:                  cl,
		externalModulesDir:      os.Getenv("EXTERNAL_MODULES_DIR"),
		dc:                      dependency.NewDependencyContainer(),
		deckhouseEmbeddedPolicy: embeddedDeckhousePolicy,
		logger:                  log.New(),

		moduleSourcesChecksum: make(sourceChecksum),
	}

	t.Run("Exact match policy", func(t *testing.T) {
		policy := v1alpha1.ModuleUpdatePolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ModuleUpdatePolicy",
				APIVersion: "deckhouse.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-1",
			},
			Spec: v1alpha1.ModuleUpdatePolicySpec{
				ModuleReleaseSelector: v1alpha1.ModuleUpdatePolicySpecReleaseSelector{
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"source": "test-1", "module": "test-module-1"}},
				},
			},
		}
		mup, err := c.getReleasePolicy("test-1", "test-module-1", []v1alpha1.ModuleUpdatePolicy{policy})
		require.NoError(t, err)
		assert.Equal(t, "test-1", mup.Name)
	})

	t.Run("Only module match policy", func(t *testing.T) {
		policy := v1alpha1.ModuleUpdatePolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ModuleUpdatePolicy",
				APIVersion: "deckhouse.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-2",
			},
			Spec: v1alpha1.ModuleUpdatePolicySpec{
				ModuleReleaseSelector: v1alpha1.ModuleUpdatePolicySpecReleaseSelector{
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"module": "test-module-2"}},
				},
			},
		}
		mup, err := c.getReleasePolicy("test-2", "test-module-2", []v1alpha1.ModuleUpdatePolicy{policy})
		require.NoError(t, err)
		assert.Equal(t, "test-2", mup.Name)
	})

	t.Run("Only source match policy", func(t *testing.T) {
		policy := v1alpha1.ModuleUpdatePolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ModuleUpdatePolicy",
				APIVersion: "deckhouse.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-3",
			},
			Spec: v1alpha1.ModuleUpdatePolicySpec{
				ModuleReleaseSelector: v1alpha1.ModuleUpdatePolicySpecReleaseSelector{
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"source": "test-3"}},
				},
			},
		}
		mup, err := c.getReleasePolicy("test-3", "test-module-3", []v1alpha1.ModuleUpdatePolicy{policy})
		require.NoError(t, err)
		assert.Equal(t, "test-3", mup.Name)
	})

	t.Run("Except module policy", func(t *testing.T) {
		policy := v1alpha1.ModuleUpdatePolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ModuleUpdatePolicy",
				APIVersion: "deckhouse.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-4",
			},
			Spec: v1alpha1.ModuleUpdatePolicySpec{
				ModuleReleaseSelector: v1alpha1.ModuleUpdatePolicySpecReleaseSelector{
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"source": "test-4", "module": "foobar"}},
				},
			},
		}
		mup, err := c.getReleasePolicy("test-4", "test-module-4", []v1alpha1.ModuleUpdatePolicy{policy})
		require.NoError(t, err)
		assert.Equal(t, "", mup.Name)
	})

	t.Run("No policy set", func(t *testing.T) {
		mup, err := c.getReleasePolicy("test-5", "test-module-5", []v1alpha1.ModuleUpdatePolicy{})
		require.NoError(t, err)
		assert.Equal(t, "", mup.Name)
	})

	t.Run("multiply policies", func(t *testing.T) {
		data, err := os.ReadFile("./testdata/policies/multi.yaml")
		require.NoError(t, err)
		manifests := releaseutil.SplitManifests(string(data))
		res := make([]v1alpha1.ModuleUpdatePolicy, 0, len(manifests))

		for _, manifest := range manifests {
			var p v1alpha1.ModuleUpdatePolicy
			_ = yaml.Unmarshal([]byte(manifest), &p)
			res = append(res, p)
		}
		mup, err := c.getReleasePolicy("foxtrot", "parca", res)
		require.NoError(t, err)
		assert.Equal(t, "foxtrot-alpha", mup.Name)

		mup, err = c.getReleasePolicy("deckhouse-prod", "deckhouse-admin", res)
		require.NoError(t, err)
		assert.Equal(t, "deckhouse-prod", mup.Name)
	})
}
