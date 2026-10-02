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
	"encoding/base64"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dockerCfg(host, username, password string) string {
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return base64.StdEncoding.EncodeToString([]byte(`{"auths":{"` + host + `":{"auth":"` + auth + `"}}}`))
}

func TestAModuleIsKeptUnderItsSource(t *testing.T) {
	assert.Equal(t, "modules/console", ModuleRef{Name: "console", Source: ModuleSource{Name: "deckhouse"}}.StorePath())
	assert.Equal(t, "modules/stronghold", ModuleRef{Name: "stronghold"}.StorePath(),
		"a module whose source is unknown stays where it always was")
	assert.Equal(t, "module-sources/ce-public/dashboard",
		ModuleRef{Name: "dashboard", Source: ModuleSource{Name: "ce-public"}}.StorePath(),
		"apart from the platform's path, which the cache fills from the upstream")
}

func TestResolveOriginsReadsEachModuleWhereItLives(t *testing.T) {
	upstream := Registry{Address: "registry.deckhouse.io", Repository: "deckhouse/ee"}
	local := Registry{Address: "127.0.0.1:5003", Repository: "system/deckhouse"}

	modules, err := ResolveOrigins([]ModuleRef{
		// The platform's own, after the image address handover: from the upstream, as the platform.
		{Name: "console", Source: ModuleSource{Name: "deckhouse",
			Repository: "registry.d8-system.svc:5001/system/deckhouse/modules"}},
		// Another source on the upstream's own registry: read from the upstream with its license.
		{Name: "sds-node-configurator", Source: ModuleSource{Name: "sds",
			Repository: "Registry.Deckhouse.io/deckhouse/ee/modules/"}},
		// A registry of its own.
		{Name: "vendor-module", Source: ModuleSource{Name: "vendor",
			Repository: "https://vendor.example.com/catalog/modules", Scheme: "HTTPS",
			DockerCfg: dockerCfg("vendor.example.com", "robot", "s3cret")}},
		{Name: "vendor-other", Source: ModuleSource{Name: "vendor",
			Repository: "https://vendor.example.com/catalog/modules"}},
		// A source an operator repointed at its path in the store ahead of air-gap.
		{Name: "moved", Source: ModuleSource{Name: "moved",
			Repository: "registry.d8-system.svc:5001/system/deckhouse/module-sources/moved"}},
		// A source whose object could not be read.
		{Name: "unread", Source: ModuleSource{Name: "unread"}},
		// No source to go by.
		{Name: "stronghold"},
	}, upstream, local)
	require.NoError(t, err)

	byName := map[string]ModuleRef{}
	for _, module := range modules {
		byName[module.Name] = module
	}

	assert.Nil(t, byName["console"].Origin)
	assert.Nil(t, byName["stronghold"].Origin)

	for _, fromUpstream := range []string{"sds-node-configurator", "unread"} {
		origin := byName[fromUpstream].Origin
		require.NotNil(t, origin, fromUpstream)
		assert.Equal(t, "registry.deckhouse.io", origin.Address, fromUpstream)
		assert.Equal(t, "deckhouse/ee/modules", origin.Repository, fromUpstream)
	}

	vendor := byName["vendor-module"].Origin
	require.NotNil(t, vendor)
	assert.Equal(t, "vendor.example.com", vendor.Address)
	assert.Equal(t, "catalog/modules", vendor.Repository)
	assert.False(t, vendor.Insecure)
	assert.Same(t, vendor, byName["vendor-other"].Origin, "one registry per source, one client for it")

	moved := byName["moved"].Origin
	require.NotNil(t, moved)
	assert.Equal(t, "127.0.0.1:5003", moved.Address)
	assert.Equal(t, "system/deckhouse/module-sources/moved", moved.Repository)
}

func TestRoutesAreExactAndKeepEachSourceApart(t *testing.T) {
	upstream := Registry{Address: "registry.deckhouse.io", Repository: "deckhouse/ee"}
	destination := Registry{Address: "127.0.0.1:5003", Repository: "system/deckhouse"}
	modules, err := ResolveOrigins([]ModuleRef{
		{Name: "console", Source: ModuleSource{Name: "deckhouse"}},
		{Name: "sds-node-configurator", Source: ModuleSource{Name: "sds",
			Repository: "registry.deckhouse.io/deckhouse/ee/modules"}},
		{Name: "vendor-module", Source: ModuleSource{Name: "vendor", Repository: "vendor.example.com/catalog/modules"}},
		{Name: "vendor-other", Source: ModuleSource{Name: "vendor", Repository: "vendor.example.com/catalog/modules"}},
	}, upstream, destination)
	require.NoError(t, err)

	got := map[string]string{}
	for _, r := range Routes(modules, destination) {
		got[r.From.Address+"/"+r.From.Repository] = r.Into
	}
	assert.Equal(t, map[string]string{
		// Read from the upstream's own path, kept under the source's: a prefix route would have taken
		// every platform module along with it.
		"registry.deckhouse.io/deckhouse/ee/modules/sds-node-configurator": "system/deckhouse/module-sources/sds/sds-node-configurator",
		"registry.deckhouse.io/deckhouse/ee/modules":                       "system/deckhouse/module-sources/sds",
		"vendor.example.com/catalog/modules/vendor-module":                 "system/deckhouse/module-sources/vendor/vendor-module",
		"vendor.example.com/catalog/modules/vendor-other":                  "system/deckhouse/module-sources/vendor/vendor-other",
		"vendor.example.com/catalog/modules":                               "system/deckhouse/module-sources/vendor",
	}, got, "one catalogue per source, and nothing for the platform's own modules")
}

func TestResolveOriginsRefusesCredentialsItCannotRead(t *testing.T) {
	_, err := ResolveOrigins([]ModuleRef{{Name: "vendor-module", Source: ModuleSource{
		Name: "vendor", Repository: "vendor.example.com/modules", DockerCfg: "not base64!",
	}}}, Registry{Address: "registry.deckhouse.io", Repository: "deckhouse/ee"}, Registry{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vendor-module")
	assert.Contains(t, err.Error(), "vendor")
}

func TestDockerCfgCredentials(t *testing.T) {
	raw, err := base64.StdEncoding.DecodeString(dockerCfg("https://Vendor.Example.com", "robot", "pa:ss"))
	require.NoError(t, err)
	username, password, err := dockerCfgCredentials(raw, "vendor.example.com")
	require.NoError(t, err)
	assert.Equal(t, "robot", username)
	assert.Equal(t, "pa:ss", password, "only the first colon separates the two")

	username, password, err = dockerCfgCredentials([]byte(`{"auths":{"other.io":{"username":"u","password":"p"}}}`), "vendor.example.com")
	require.NoError(t, err)
	assert.Empty(t, username+password, "no entry for the host: anonymous")
}

type discoverFunc func(ctx context.Context, source Registry, puller *remote.Puller) ([]name.Reference, error)

func (f discoverFunc) Discover(ctx context.Context, source Registry, puller *remote.Puller) ([]name.Reference, error) {
	return f(ctx, source, puller)
}

// modulesAndCatalogues is the part of Release.Discover this file is about.
func modulesAndCatalogues(modules []ModuleRef) Discoverer {
	return discoverFunc(func(ctx context.Context, source Registry, puller *remote.Puller) ([]name.Reference, error) {
		references, err := ModuleReferences(ctx, source, puller, modules)
		if err != nil {
			return nil, err
		}
		catalogues, err := SourceCatalogues(ctx, source, modules, nil)
		if err != nil {
			return nil, err
		}
		return append(references, catalogues...), nil
	})
}

// TestAModuleOfAnotherSourceSurvivesTheUpstreamsOfTheSameName is what a live cluster found: the
// platform ships `dashboard` too, a different build, and while the store kept the source's copy on the
// platform's path the cache put the upstream's back over it on every read — the store never held what
// the cluster ran. Kept apart, both are there, each where its reader looks.
func TestAModuleOfAnotherSourceSurvivesTheUpstreamsOfTheSameName(t *testing.T) {
	upstream := startRegistry(t)
	upstream.Repository = "deckhouse/ee"
	vendor := startRegistry(t)
	destination := startRegistry(t)
	destination.Repository = "system/deckhouse"

	// The same module name at both, with different content.
	platformBuild := pushByDigest(t, upstream, "deckhouse/ee/modules/dashboard")
	pushInstaller(t, upstream, "deckhouse/ee/modules/dashboard:v1.0.0", map[string]any{
		"images_digests.json": map[string]string{"web": platformBuild.String()},
	})
	sourceBuild := pushByDigest(t, vendor, "catalog/modules/dashboard")
	pushInstaller(t, vendor, "catalog/modules/dashboard:v1.0.0", map[string]any{
		"images_digests.json": map[string]string{"web": sourceBuild.String()},
	})
	// The source's catalogue: its modules as the tags of its repository.
	pushInstaller(t, vendor, "catalog/modules:dashboard", map[string]any{"module.yaml": map[string]string{"name": "dashboard"}})

	modules, err := ResolveOrigins([]ModuleRef{{Name: "dashboard", Version: "v1.0.0",
		Source: ModuleSource{Name: "vendor", Repository: vendor.Address + "/catalog/modules", Scheme: "HTTP"}}},
		upstream, destination)
	require.NoError(t, err)
	require.NotNil(t, modules[0].Origin)
	modules[0].Origin.Insecure = true

	copier := &Copier{
		Source:      upstream,
		Destination: destination,
		Routes:      Routes(modules, destination),
		Discover:    modulesAndCatalogues(modules),
	}
	report, err := copier.Run(context.Background())
	require.NoError(t, err)
	assert.Empty(t, report.Failed)
	assert.EqualValues(t, 3, report.Written, "the package, its image, and the source's catalogue")

	sourcePackage, err := digestOf(t, destination, "system/deckhouse/module-sources/vendor/dashboard:v1.0.0")
	require.NoError(t, err, "the source's package, under the source's own path")
	vendorPackage, err := digestOf(t, vendor, "catalog/modules/dashboard:v1.0.0")
	require.NoError(t, err)
	assert.Equal(t, vendorPackage, sourcePackage, "the source's build, not the upstream's")

	image, err := name.NewDigest(destination.Address+"/system/deckhouse/module-sources/vendor/dashboard@"+sourceBuild.String(), name.Insecure)
	require.NoError(t, err)
	_, err = remote.Get(image)
	require.NoError(t, err, "and the image the source's package declares")

	_, err = digestOf(t, destination, "system/deckhouse/module-sources/vendor:dashboard")
	require.NoError(t, err, "and the catalogue a repointed ModuleSource lists")

	_, err = digestOf(t, destination, "system/deckhouse/modules/dashboard:v1.0.0")
	assert.Error(t, err, "nothing was written to the platform's path")
}

// TestAReplicaReadsASourcesModuleFromItsPath: a follower replicating, or a count of the set, reads the
// store — where the leader kept the module under its source, whatever the source's registry is.
func TestAReplicaReadsASourcesModuleFromItsPath(t *testing.T) {
	store := startRegistry(t)
	store.Repository = "system/deckhouse"

	image := pushByDigest(t, store, "system/deckhouse/module-sources/vendor/dashboard")
	pushInstaller(t, store, "system/deckhouse/module-sources/vendor/dashboard:v1.0.0", map[string]any{
		"images_digests.json": map[string]string{"web": image.String()},
	})
	pushInstaller(t, store, "system/deckhouse/module-sources/vendor:dashboard", map[string]any{"module.yaml": map[string]string{}})

	modules := []ModuleRef{{Name: "dashboard", Version: "v1.0.0",
		Source: ModuleSource{Name: "vendor", Repository: "vendor.example.com/catalog/modules"}}}
	puller, err := remote.NewPuller(store.Options...)
	require.NoError(t, err)

	references, err := modulesAndCatalogues(modules).Discover(context.Background(), store, puller)
	require.NoError(t, err)

	var listed []string
	for _, reference := range references {
		listed = append(listed, reference.String())
	}
	assert.ElementsMatch(t, []string{
		store.Address + "/system/deckhouse/module-sources/vendor/dashboard:v1.0.0",
		store.Address + "/system/deckhouse/module-sources/vendor/dashboard@" + image.String(),
		store.Address + "/system/deckhouse/module-sources/vendor:dashboard",
	}, listed)
}

// TestTheFlantSourceIsReadFromTheUpstream: on FE the platform renders the Flant ModuleSource from the
// cluster's registry address, and once the cluster pulls through the agent that address is the
// in-cluster one — `registry.d8-system.svc:5001/flant/modules`. Dialled as written it is the store,
// under an authority this client does not trust: x509, and the whole fill stopped before its first
// copy. It is the upstream's registry at the same path, as the node agent serves it.
func TestTheFlantSourceIsReadFromTheUpstream(t *testing.T) {
	upstream := Registry{Address: "registry.deckhouse.io", Repository: "deckhouse/fe"}
	destination := Registry{Address: "127.0.0.1:5003", Repository: "system/deckhouse"}

	modules, err := ResolveOrigins([]ModuleRef{
		{Name: "console", Source: ModuleSource{Name: "flant", Repository: "registry.d8-system.svc:5001/flant/modules"}},
		{Name: "sds", Source: ModuleSource{Name: "own", Repository: "registry.d8-system.svc:5001/system/deckhouse/module-sources/own"}},
	}, upstream, destination)
	require.NoError(t, err)

	require.NotNil(t, modules[0].Origin)
	assert.Equal(t, "registry.deckhouse.io", modules[0].Origin.Address)
	assert.Equal(t, "flant/modules", modules[0].Origin.Repository,
		"beside the edition's image set on the upstream's registry, not under it")

	require.NotNil(t, modules[1].Origin)
	assert.Equal(t, "127.0.0.1:5003", modules[1].Origin.Address, "a source on its store path is still read from the store")

	// Air-gapped: no upstream, and nothing to translate the address into.
	airGapped, err := ResolveOrigins([]ModuleRef{
		{Name: "console", Source: ModuleSource{Name: "flant", Repository: "registry.d8-system.svc:5001/flant/modules"}},
	}, Registry{}, destination)
	require.NoError(t, err)
	assert.Equal(t, "registry.d8-system.svc:5001", airGapped[0].Origin.Address)
}

// TestThePlatformsCatalogueStaysWhereThePlatformReadsIt: a second ModuleSource on the upstream's own
// `modules` repository reads the platform's very catalogue, and a route keyed by that repository took
// every platform tag along to the source's path. The platform's path stayed empty, and once the
// upstream was gone deckhouse-controller listed it as NAME_UNKNOWN. Both get it now.
func TestThePlatformsCatalogueStaysWhereThePlatformReadsIt(t *testing.T) {
	upstream := startRegistry(t)
	upstream.Repository = "deckhouse/ee"
	destination := startRegistry(t)
	destination.Repository = "system/deckhouse"

	pushInstaller(t, upstream, "deckhouse/ee/modules:console", map[string]any{"module.yaml": map[string]string{"name": "console"}})
	pushInstaller(t, upstream, "deckhouse/ee/modules:sds-node-configurator",
		map[string]any{"module.yaml": map[string]string{"name": "sds-node-configurator"}})

	modules, err := ResolveOrigins([]ModuleRef{
		{Name: "sds-node-configurator", Source: ModuleSource{Name: "sds", Repository: upstream.Address + "/deckhouse/ee/modules"}},
	}, upstream, destination)
	require.NoError(t, err)

	copier := &Copier{
		Source:      upstream,
		Destination: destination,
		Routes:      Routes(modules, destination),
		Discover: explicit{
			upstream.Address + "/deckhouse/ee/modules:console",
			upstream.Address + "/deckhouse/ee/modules:sds-node-configurator",
		},
	}
	report, err := copier.Run(context.Background())
	require.NoError(t, err)
	assert.Empty(t, report.Failed)

	for _, tag := range []string{"console", "sds-node-configurator"} {
		_, err := digestOf(t, destination, "system/deckhouse/modules:"+tag)
		require.NoError(t, err, "the platform's catalogue, where the platform's ModuleSource lists it: %s", tag)
		_, err = digestOf(t, destination, "system/deckhouse/module-sources/sds:"+tag)
		require.NoError(t, err, "and the source's, where a repointed source lists it: %s", tag)
	}
}
