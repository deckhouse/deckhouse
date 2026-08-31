// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pkgsync

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func getPackage(t *testing.T, cl client.Client, name string) *v1alpha1.ModulePackage {
	t.Helper()

	pkg := new(v1alpha1.ModulePackage)
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: name}, pkg))

	return pkg
}

func TestSyncModulePackages(t *testing.T) {
	ctx := context.Background()

	t.Run("creates an empty package for an embedded module", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		syncOK(t, s)

		pkg := getPackage(t, cl, "echo")
		assert.Equal(t, map[string]string{"heritage": "deckhouse"}, pkg.Labels)
		assert.Empty(t, pkg.OwnerReferences, "no owner: no repository offers an embedded package")
		assert.Empty(t, pkg.Status.AvailableRepositories, "the entry stays empty until a scan fills it")
	})

	t.Run("leaves an existing package untouched", func(t *testing.T) {
		existing := &v1alpha1.ModulePackage{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "echo",
				Labels: map[string]string{"user": "label"},
			},
			Status: v1alpha1.ModulePackageStatus{
				AvailableRepositories: []string{"deckhouse-modules"},
			},
		}

		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

		s, cl := newTestSyncer(t, "v1.80.0", dir, existing)
		syncOK(t, s)

		pkg := getPackage(t, cl, "echo")
		assert.Equal(t, map[string]string{"user": "label"}, pkg.Labels)
		assert.Equal(t, []string{"deckhouse-modules"}, pkg.Status.AvailableRepositories)
	})

	t.Run("a release stub creates no package", func(t *testing.T) {
		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(),
			testRelease("parca", "deckhouse", "1.4.3", v1alpha1.ModuleReleasePhaseDeployed),
		)

		syncOK(t, s)

		packages := new(v1alpha1.ModulePackageList)
		require.NoError(t, cl.List(ctx, packages))
		assert.Empty(t, packages.Items, "the repository scan builds the catalog of sourced packages")
	})
}
