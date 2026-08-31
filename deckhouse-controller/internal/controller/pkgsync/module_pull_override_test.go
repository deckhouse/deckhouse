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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
)

func testPackageVersionStub(repository, module, version string) *v1alpha1.ModulePackageVersion {
	return &v1alpha1.ModulePackageVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.MakeModulePackageVersionName(repository, module, version),
			Labels: map[string]string{
				v1alpha1.ModulePackageVersionLabelRepository: repository,
				v1alpha1.ModulePackageVersionLabelPackage:    module,
			},
		},
		Spec: v1alpha1.ModulePackageVersionSpec{
			PackageName:           module,
			PackageRepositoryName: repository,
			PackageVersion:        version,
		},
	}
}

func testModuleSourceOffering(name string, modules ...string) *v1alpha1.ModuleSource {
	source := testModuleSource(name, "registry.example.io/"+name)

	for _, module := range modules {
		source.Status.AvailableModules = append(source.Status.AvailableModules, v1alpha1.AvailableModule{Name: module})
	}

	return source
}

func TestDevModuleRepositoryDerivation(t *testing.T) {
	t.Run("config source names the repository", func(t *testing.T) {
		conf := testModuleConfig("echo")
		conf.Spec.Source = "deckhouse"

		s, cl := newTestSyncer(t, testVersion, t.TempDir(),
			testReadyPullOverride("echo", "dev-tag"),
			conf,
		)

		syncOK(t, s)

		module := getV2Module(t, cl, "echo")
		assert.Equal(t, "deckhouse-modules", module.Spec.PackageRepositoryName)
		assert.Equal(t, "dev-tag", module.Spec.PackageVersion)
		assert.True(t, module.IsDev())
	})

	t.Run("the module spec keeps its repository", func(t *testing.T) {
		existing := &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{Name: "echo"},
			Spec:       v1alpha2.ModuleSpec{PackageRepositoryName: "example", PackageVersion: "v1.0.0"},
		}

		s, cl := newTestSyncer(t, testVersion, t.TempDir(),
			existing,
			testReadyPullOverride("echo", "dev-tag"),
		)

		syncOK(t, s)

		module := getV2Module(t, cl, "echo")
		assert.Equal(t, "example", module.Spec.PackageRepositoryName)
		assert.Equal(t, "dev-tag", module.Spec.PackageVersion)
	})

	t.Run("a single-repository package catalog names the repository", func(t *testing.T) {
		s, cl := newTestSyncer(t, testVersion, t.TempDir(),
			testReadyPullOverride("echo", "dev-tag"),
			testPackageVersionStub("example", "echo", "v1.0.0"),
			testPackageVersionStub("example", "echo", "v1.1.0"),
		)

		syncOK(t, s)

		module := getV2Module(t, cl, "echo")
		assert.Equal(t, "example", module.Spec.PackageRepositoryName)
	})

	t.Run("package versions of several repositories name none", func(t *testing.T) {
		s, cl := newTestSyncer(t, testVersion, t.TempDir(),
			testReadyPullOverride("echo", "dev-tag"),
			testPackageVersionStub("example", "echo", "v1.0.0"),
			testPackageVersionStub("other", "echo", "v1.0.0"),
		)

		syncOK(t, s)

		module := new(v1alpha2.Module)
		err := cl.Get(t.Context(), client.ObjectKey{Name: "echo"}, module)
		assert.True(t, err != nil, "an ambiguous repository must not place the module")
	})

	t.Run("the single module source offering the module names the repository", func(t *testing.T) {
		s, cl := newTestSyncer(t, testVersion, t.TempDir(),
			testReadyPullOverride("echo", "dev-tag"),
			testModuleSourceOffering("deckhouse", "echo", "parca"),
			testModuleSourceOffering("other", "parca"),
		)

		syncOK(t, s)

		module := getV2Module(t, cl, "echo")
		assert.Equal(t, "deckhouse-modules", module.Spec.PackageRepositoryName)
	})

	t.Run("no synced resource names the repository, the override is skipped", func(t *testing.T) {
		s, cl := newTestSyncer(t, testVersion, t.TempDir(),
			testReadyPullOverride("echo", "dev-tag"),
		)

		syncOK(t, s)

		module := new(v1alpha2.Module)
		err := cl.Get(t.Context(), client.ObjectKey{Name: "echo"}, module)
		assert.True(t, err != nil, "a module with no repository trace must not be placed")
	})
}
