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

package moduleloader

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// newEnsureLoader builds a Loader with everything ensureModule reads: a fake
// client, the embedded update policy (for the release channel) and a version.
func newEnsureLoader(t *testing.T, objects ...client.Object) *Loader {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.SchemeBuilder.AddToScheme(scheme))
	require.NoError(t, v1alpha2.SchemeBuilder.AddToScheme(scheme))

	return &Loader{
		client:  fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build(),
		logger:  log.NewNop(),
		version: "v1.73.0",
		embeddedPolicy: helpers.NewModuleUpdatePolicySpecContainer(&v1alpha2.ModuleUpdatePolicySpec{
			ReleaseChannel: "Stable",
		}),
	}
}

func ensureTestModule(name, source string) *v1alpha1.Module {
	return &v1alpha1.Module{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.ModuleGVK.Kind,
			APIVersion: v1alpha1.ModuleGVK.GroupVersion().String(),
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Properties: v1alpha1.ModuleProperties{Source: source},
	}
}

func getEnsuredModule(t *testing.T, l *Loader, name string) *v1alpha1.Module {
	t.Helper()

	module := new(v1alpha1.Module)
	require.NoError(t, l.client.Get(context.Background(), client.ObjectKey{Name: name}, module))

	return module
}

// TestEnsureModuleEmbeddedSource verifies that ensureModule keeps the invariant
// "a physically embedded module always reports Source == Embedded". This is the
// reconciliation point that heals a stale external source (e.g. deckhouse) left
// on an embedded module by an erroneous flip after a registry migration - a
// value that otherwise stuck until the Module was deleted by hand.
func TestEnsureModuleEmbeddedSource(t *testing.T) {
	embeddedDef := func() *moduletypes.Definition {
		return &moduletypes.Definition{
			Name:   "ingress-nginx",
			Weight: 380,
			// parsed from the embedded modules dir - i.e. shipped on the filesystem
			Path: embeddedModulesDir + "/380-ingress-nginx",
		}
	}

	t.Run("stale external source on an embedded module is reset to Embedded", func(t *testing.T) {
		l := newEnsureLoader(t, ensureTestModule("ingress-nginx", "deckhouse"))

		require.NoError(t, l.ensureModule(context.Background(), embeddedDef(), true))

		module := getEnsuredModule(t, l, "ingress-nginx")
		assert.Equal(t, v1alpha1.ModuleSourceEmbedded, module.Properties.Source,
			"embedded module must be pinned back to the Embedded source")
	})

	t.Run("empty source on an embedded module is set to Embedded", func(t *testing.T) {
		l := newEnsureLoader(t, ensureTestModule("ingress-nginx", ""))

		require.NoError(t, l.ensureModule(context.Background(), embeddedDef(), true))

		module := getEnsuredModule(t, l, "ingress-nginx")
		assert.Equal(t, v1alpha1.ModuleSourceEmbedded, module.Properties.Source)
	})

	t.Run("embedded source is left untouched", func(t *testing.T) {
		l := newEnsureLoader(t, ensureTestModule("ingress-nginx", v1alpha1.ModuleSourceEmbedded))

		require.NoError(t, l.ensureModule(context.Background(), embeddedDef(), true))

		module := getEnsuredModule(t, l, "ingress-nginx")
		assert.Equal(t, v1alpha1.ModuleSourceEmbedded, module.Properties.Source)
	})

	t.Run("downloaded (non-embedded) module keeps its external source", func(t *testing.T) {
		l := newEnsureLoader(t, ensureTestModule("echo", "example"))
		def := &moduletypes.Definition{Name: "echo", Weight: 900, Path: "/deckhouse/downloaded/modules/echo"}

		require.NoError(t, l.ensureModule(context.Background(), def, false))

		module := getEnsuredModule(t, l, "echo")
		assert.Equal(t, "example", module.Properties.Source,
			"a downloaded module must not be forced to the Embedded source")
	})
}
