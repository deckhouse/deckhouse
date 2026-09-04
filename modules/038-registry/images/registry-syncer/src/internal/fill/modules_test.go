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

package fill

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAModuleAccountsForItsOwnImages is the hole this closes, and it is the dangerous kind: a store
// that reports itself complete while missing images the cluster runs.
//
// A release declares the platform's images and nothing else. The modules a cluster keeps are packaged
// and declared separately — so completeness judged on the release alone can be missing every one of
// them, and completeness is exactly the answer that authorizes cutting an air-gapped cluster off from
// its upstream. Measured on a bundle: 474 of 474 platform manifests present, 34 module images absent.
func TestAModuleAccountsForItsOwnImages(t *testing.T) {
	source := startRegistry(t)
	// Modules live under the platform's own repository, which is what the store is scoped to.
	source.Repository = "deckhouse/ee"

	// A module package declares its images flat — name to digest — where the platform's own account of
	// its set is nested by module. Same file name, different shape, which is why they are read apart.
	one := pushByDigest(t, source, "deckhouse/ee/modules/ingress-nginx")
	two := pushByDigest(t, source, "deckhouse/ee/modules/ingress-nginx")
	pushInstaller(t, source, "deckhouse/ee/modules/ingress-nginx:v1.2.3", map[string]any{
		"images_digests.json": map[string]string{
			"controller": one.String(),
			"kruise":     two.String(),
		},
	})

	puller, err := remote.NewPuller(source.Options...)
	require.NoError(t, err)

	references, err := ModuleReferences(t.Context(), source, puller,
		[]ModuleRef{{Name: "ingress-nginx", Version: "v1.2.3"}})
	require.NoError(t, err)

	var listed []string
	for _, reference := range references {
		listed = append(listed, reference.String())
	}

	assert.Contains(t, listed, source.Address+"/deckhouse/ee/modules/ingress-nginx:v1.2.3",
		"the package itself: a cluster that cannot pull it cannot reinstall the module")
	assert.Contains(t, listed, source.Address+"/deckhouse/ee/modules/ingress-nginx@"+one.String())
	assert.Contains(t, listed, source.Address+"/deckhouse/ee/modules/ingress-nginx@"+two.String())
	assert.Len(t, listed, 3)
}

// TestAModuleWithNoImagesOfItsOwnIsNotAFailure: several modules are templates and hooks that run in the
// platform's image and carry no images at all. Refusing them would make completeness unreachable on any
// cluster that keeps one.
func TestAModuleWithNoImagesOfItsOwnIsNotAFailure(t *testing.T) {
	source := startRegistry(t)
	source.Repository = "deckhouse/ee"
	pushInstaller(t, source, "deckhouse/ee/modules/pod-reloader:v0.1.0", map[string]any{
		"module.yaml": map[string]string{"name": "pod-reloader"},
	})

	puller, err := remote.NewPuller(source.Options...)
	require.NoError(t, err)

	references, err := ModuleReferences(t.Context(), source, puller,
		[]ModuleRef{{Name: "pod-reloader", Version: "v0.1.0"}})
	require.NoError(t, err)
	require.Len(t, references, 1, "the package, and nothing it does not declare")
}

// TestAModuleThatCannotBeReadIsAnError, rather than a module quietly left out.
//
// Leaving it out would lower the bar for completeness by exactly the images nobody could account for —
// silently, and in the direction that authorizes dropping the upstream. An error stops that, and says
// which module to look at.
func TestAModuleThatCannotBeReadIsAnError(t *testing.T) {
	source := startRegistry(t)

	puller, err := remote.NewPuller(source.Options...)
	require.NoError(t, err)

	_, err = ModuleReferences(t.Context(), source, puller,
		[]ModuleRef{{Name: "absent", Version: "v9.9.9"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "absent", "the message has to name the module")
}

// TestNoModulesIsNotAnEmptySet: a cluster with nothing but the platform's own modules is ordinary, and
// nothing about its accounting changes.
func TestNoModulesIsNotAnEmptySet(t *testing.T) {
	source := startRegistry(t)

	references, err := ModuleReferences(t.Context(), source, nil, nil)
	require.NoError(t, err)
	require.Empty(t, references)
}

// TestTheModuleCatalogueIsPartOfTheSet is what lets an air-gapped cluster still know which modules
// exist.
//
// The platform reads `GET /v2/<repository>/modules/tags/list` to enumerate what it can install, and a
// pull-through cache never holds that: a tag listing leaves nothing behind. Measured on `ly-mmc` after
// a clean transition — every replica full, every node pulling, and the whole catalogue answering
// `NAME_UNKNOWN` — so the catalogue belongs in the declared set, where a store missing it is not
// judged complete.
func TestTheModuleCatalogueIsPartOfTheSet(t *testing.T) {
	source := startRegistry(t)
	source.Repository = "deckhouse/ee"

	// Two modules offered by the source, only one of which this cluster keeps.
	pushImage(t, source, "deckhouse/ee/modules:prometheus")
	pushImage(t, source, "deckhouse/ee/modules:upmeter")

	references, err := ModuleCatalogue(context.Background(), source, nil)
	require.NoError(t, err)

	var tags []string
	for _, reference := range references {
		tags = append(tags, reference.Identifier())
	}
	assert.ElementsMatch(t, []string{"prometheus", "upmeter"}, tags,
		"every module the source offers has to be in the set, not only the ones the cluster runs")
}

// TestASourceThatWillNotListIsADegradationAndNotAFailure keeps a fill possible on a registry that
// withholds tag listing.
//
// Failing would be the worse answer: such a cluster could never complete a fill at all, so it could
// never drop its upstream, while the only thing it actually loses is the ability to enumerate modules
// once air-gapped. The reason is handed to the caller so it reaches a log rather than being swallowed.
func TestASourceThatWillNotListIsADegradationAndNotAFailure(t *testing.T) {
	refusing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/tags/list") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(refusing.Close)

	parsed, err := url.Parse(refusing.URL)
	require.NoError(t, err)

	var told error
	references, err := ModuleCatalogue(
		context.Background(),
		Registry{Address: parsed.Host, Insecure: true, Repository: "deckhouse/ee"},
		func(reason error) { told = reason },
	)
	require.NoError(t, err, "a source that will not list must not stop the fill")
	assert.Empty(t, references)
	require.Error(t, told, "the reason has to reach the caller, or nobody ever learns why")
	assert.Contains(t, told.Error(), "modules")
}
