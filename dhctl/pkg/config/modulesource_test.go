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

package config

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/digests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
)

// dockerCfg for registry.example.io with auth "user:pass".
const testModuleSourceDockerCfg = "eyJhdXRocyI6IHsicmVnaXN0cnkuZXhhbXBsZS5pbyI6IHsiYXV0aCI6ICJkWE5sY2pwd1lYTnoifX19"

func TestParseModuleDocs(t *testing.T) {
	docs := []string{
		`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: deckhouse
spec:
  registry:
    repo: registry.example.io/modules
    dockerCfg: ` + testModuleSourceDockerCfg + `
    scheme: HTTP
    ca: |
      -----BEGIN CERTIFICATE-----
`,
		`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  enabled: true
  version: 1
  source: deckhouse
  settings:
    layout: Standard
`,
		// Neither of these must be picked up: the deckhouse ModuleConfig is registry
		// configuration, not a provider, and the ClusterConfiguration is not a module at all.
		`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: EarlyAccess
`,
		`
apiVersion: deckhouse.io/v1alpha2
kind: ModulePullOverride
metadata:
  name: cloud-provider-dvp
spec:
  imageTag: mr1
`,
		`
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
`,
		"   ",
	}

	md, err := ParseModuleDocs(docs)
	require.NoError(t, err)

	require.Len(t, md.Sources, 1)
	require.Equal(t, "deckhouse", md.Sources[0].GetName())
	require.Equal(t, "registry.example.io/modules", md.Sources[0].Spec.Registry.Repo)
	require.Equal(t, "HTTP", md.Sources[0].Spec.Registry.Scheme)
	require.Contains(t, md.Sources[0].Spec.Registry.CA, "BEGIN CERTIFICATE")

	require.Len(t, md.ProviderConfigs, 1)
	require.Equal(t, "deckhouse", md.providerSource("cloud-provider-dvp"))
	require.Equal(t, "Standard", md.ProviderConfigs["cloud-provider-dvp"].Spec.Settings["layout"])

	require.Equal(t, "EarlyAccess", md.ReleaseChannel)
	require.Equal(t, "mr1", md.ImageTags["cloud-provider-dvp"])
	// The channel is kebab-cased the way release images are tagged.
	require.Equal(t, "early-access", md.releaseChannelTag())
}

func TestParseModuleDocsNothingFound(t *testing.T) {
	md, err := ParseModuleDocs([]string{"", "\n"})
	require.NoError(t, err)
	require.Empty(t, md.Sources)
	require.Empty(t, md.ProviderConfigs)
	// No deckhouse ModuleConfig at all: the controller's built-in policy is Stable.
	require.Equal(t, "stable", md.releaseChannelTag())
}

func TestResolveModuleProviderBundleNotAModule(t *testing.T) {
	// No ModuleConfig for the provider - the in-tree path, which must not be reported as
	// found and must not touch a registry.
	_, found, err := resolveModuleProviderBundle(context.Background(), "yandex", configModuleDocs([]string{ensureClusterConfigDoc("Yandex")}), testModuleOptions(t, "yandex"))
	require.NoError(t, err)
	require.False(t, found)
}

// A bare cloud-provider ModuleConfig - no spec.source, no ModulePullOverride - is enough for a
// module this image does not ship, and it is also the only shape an operator can write: the
// admission webhook rejects a spec.source that is not among the module's availableSources, which
// a module shipped with deckhouse does not have. images_digests.json keeps a section for the
// module until the build stops producing it, and that leftover must not win over the operator's
// ModuleConfig. The <Provider>ClusterConfiguration alongside it is the legacy infrastructure
// document, not a vote on where the module comes from.
// A CE installer handed the published OpenStack getting-started config.yml reaches this path:
// CE ships neither openstack candi nor the module, so the bare ModuleConfig sends dhctl at a
// repository CE never publishes. The registry's 404 is the only thing that can tell that apart
// from a provider genuinely published outside the repo, so the error has to offer both readings.
func TestResolveModuleProviderBundleGuessedRepoExplainsItself(t *testing.T) {
	stubModuleRegistry(t, &fakeRegistry{err: fmt.Errorf("NAME_UNKNOWN")})

	bareModuleConfig := `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-openstack
spec:
  enabled: true
  version: 1
`

	t.Run("no source named", func(t *testing.T) {
		_, found, err := resolveModuleProviderBundle(context.Background(), "openstack",
			configModuleDocs([]string{ensureRegistryMCDoc, bareModuleConfig}), testModuleOptions(t))
		require.True(t, found)
		require.ErrorContains(t, err, "NAME_UNKNOWN")
		require.ErrorContains(t, err, "this Deckhouse edition does not include that provider")
		require.ErrorContains(t, err, "ModuleSource publishing it is missing")
	})

	// The operator wrote the address that failed, so repeating the guess back at them is noise.
	t.Run("source named", func(t *testing.T) {
		_, found, err := resolveModuleProviderBundle(context.Background(), "openstack", configModuleDocs([]string{
			ensureRegistryMCDoc,
			testModuleSourceDoc(t, "registry.example.io/modules"),
			bareModuleConfig + "  source: dev\n",
		}), testModuleOptions(t))
		require.True(t, found)
		require.ErrorContains(t, err, "NAME_UNKNOWN")
		require.NotContains(t, err.Error(), "does not include that provider")
	})
}

func TestResolveModuleProviderBundleBareModuleConfigModuleNotInImage(t *testing.T) {
	const moduleRepo = "r.example.com/test/modules/cloud-provider-dvp"

	stubEmbeddedDigests(t, `{"cloudProviderDvp": {"terraformManager": "sha256:embedded"}}`)

	reg := &fakeRegistry{images: map[string]crv1.Image{
		moduleRepo + "/release": testImage(t, map[string]string{"version.json": `{"version": "1.0.0"}`}),
		moduleRepo:              testImage(t, map[string]string{"images_digests.json": `{"terraformManager": "` + testBundleDigest + `"}`}),
	}}
	stubModuleRegistry(t, reg)

	ref, found, err := resolveModuleProviderBundle(context.Background(), "dvp", configModuleDocs([]string{
		ensureRegistryMCDoc,
		ensureClusterConfigDoc("DVP"), `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  enabled: true
  version: 1
  settings:
    layout: Standard
`}), testModuleOptions(t))
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, moduleRepo+"@"+testBundleDigest, ref.Image)
	require.NotEqual(t, "sha256:embedded", ref.Digest, "the leftover digests section must not win over the ModuleConfig")
}

// The OpenStack regression: the published getting-started configuration for OpenStack ships a
// ClusterConfiguration, a BARE cloud-provider-openstack ModuleConfig and an
// OpenStackClusterConfiguration in one file (docs/site/_includes/getting_started/
// openstack_selectel/partials/config.yml.standard.other.inc, and the VK and _ru_ copies of it).
// Every edition that carries the provider ships modules/030-cloud-provider-openstack, so that
// bare ModuleConfig must not send dhctl to a registry - doing so aborts LoadConfigFromFile on a
// fetch error before any preflight check runs, with no fallback.
func TestResolveModuleProviderBundleBareModuleConfigModuleInImageStaysInternal(t *testing.T) {
	reg := &fakeRegistry{err: fmt.Errorf("the registry must not be touched at all")}
	stubModuleRegistry(t, reg)

	_, found, err := resolveModuleProviderBundle(context.Background(), "openstack", configModuleDocs([]string{
		ensureRegistryMCDoc,
		ensureClusterConfigDoc("OpenStack"), `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-openstack
spec:
  enabled: true
  version: 1
`}), testModuleOptions(t, "openstack"))
	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, reg.tags)
}

// A modules directory that cannot be read at all - dhctl running somewhere the install tree was
// never unpacked - answers "shipped", so a bare ModuleConfig resolves from the embedded digests
// instead of turning the provider external on a filesystem that carries no evidence either way.
// The two explicit opt-ins are read off the documents and do not consult the directory at all.
func TestProviderIsExternalUnreadableModulesDir(t *testing.T) {
	missing := &options.GlobalOptions{ModulesDir: filepath.Join(t.TempDir(), "no-such-modules-dir")}

	for _, tc := range []struct {
		name     string
		doc      string
		external bool
	}{
		{"bare ModuleConfig", `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  enabled: true
  version: 1
`, false},
		{"spec.source", testProviderMCWithSourceDoc, true},
		{"ModulePullOverride", `
apiVersion: deckhouse.io/v1alpha2
kind: ModulePullOverride
metadata:
  name: cloud-provider-dvp
spec:
  imageTag: mr1
`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			md, err := ParseModuleDocs([]string{tc.doc})
			require.NoError(t, err)
			require.Equal(t, tc.external, md.providerIsExternal("dvp", missing))
		})
	}
}

// A document that does not decode is left for parseDocument, which names the kind and prints
// the numbered lines. The module documents around it still have to be read.
func TestParseModuleDocsSkipsMalformedDocument(t *testing.T) {
	const malformed = "kind: ModuleConfig\n\tname: broken\n"

	// Pinned so the case cannot quietly turn into a document that decodes fine.
	require.Error(t, yaml.Unmarshal([]byte(malformed), new(unstructured.Unstructured)))

	md, err := ParseModuleDocs([]string{malformed, testProviderMCWithSourceDoc})
	require.NoError(t, err)
	require.Equal(t, "dev", md.providerSource("cloud-provider-dvp"))
}

// Both explicit opt-ins outrank the modules directory. A provider whose module this image does
// ship can still be pointed at one specific module build - a dev tag, a ModuleSource of its own
// - and that is the only way to test such a build at bootstrap.
func TestResolveModuleProviderBundleExplicitOptInBeatsShippedModule(t *testing.T) {
	const moduleRepo = "r.example.com/test/modules/cloud-provider-openstack"

	for _, tc := range []struct {
		name string
		doc  string
		tag  string
	}{
		{"spec.source", `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-openstack
spec:
  enabled: true
  version: 1
  source: deckhouse
`, "v1.0.0"},
		{"ModulePullOverride", `
apiVersion: deckhouse.io/v1alpha2
kind: ModulePullOverride
metadata:
  name: cloud-provider-openstack
spec:
  imageTag: mr1
`, "mr1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := &fakeRegistry{images: map[string]crv1.Image{
				moduleRepo + "/release": testImage(t, map[string]string{"version.json": `{"version": "1.0.0"}`}),
				moduleRepo:              testImage(t, map[string]string{"images_digests.json": `{"terraformManager": "` + testBundleDigest + `"}`}),
			}}
			stubModuleRegistry(t, reg)

			ref, found, err := resolveModuleProviderBundle(context.Background(), "openstack",
				configModuleDocs([]string{ensureRegistryMCDoc, tc.doc}), testModuleOptions(t, "openstack"))
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, tc.tag, reg.tags[moduleRepo])
			require.Equal(t, moduleRepo+"@"+testBundleDigest, ref.Image)
		})
	}
}

func TestResolveModuleProviderBundleUnknownSource(t *testing.T) {
	// A ModuleConfig naming a ModuleSource that is not in the configuration is a hard error:
	// falling back to the main registry would pull a different module than was asked for. The
	// one exception, "deckhouse", is covered by TestResolveModuleProviderBundleDefaultSource.
	_, found, err := resolveModuleProviderBundle(context.Background(), "dvp", configModuleDocs([]string{`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  enabled: true
  version: 1
  source: missing
`}), testModuleOptions(t))
	require.True(t, found, "a present ModuleConfig must not fall through to the embedded digests")
	require.ErrorContains(t, err, "missing")
}

func TestModuleSourceRegistryConfig(t *testing.T) {
	t.Run("credentials from dockerCfg", func(t *testing.T) {
		source := &ModuleSource{Spec: ModuleSourceSpec{Registry: ModuleSourceRegistry{
			Repo:      "registry.example.io/modules",
			DockerCfg: testModuleSourceDockerCfg,
			CA:        "ca-pem",
		}}}

		conf, err := source.RegistryConfig()
		require.NoError(t, err)
		require.Equal(t, "registry.example.io/modules", conf.GetRegistry())
		// Not set in the source, so the CRD default applies.
		require.Equal(t, "HTTPS", conf.GetScheme())
		require.Equal(t, "user", conf.GetUsername())
		require.Equal(t, "pass", conf.GetPassword())
		require.Equal(t, "ca-pem", conf.GetCA())
	})

	t.Run("anonymous access", func(t *testing.T) {
		source := &ModuleSource{Spec: ModuleSourceSpec{Registry: ModuleSourceRegistry{
			Repo:   "registry.example.io/modules",
			Scheme: "HTTP",
		}}}

		conf, err := source.RegistryConfig()
		require.NoError(t, err)
		require.Equal(t, "HTTP", conf.GetScheme())
		require.Empty(t, conf.GetUsername())
	})

	t.Run("no repo", func(t *testing.T) {
		_, err := (&ModuleSource{}).RegistryConfig()
		require.Error(t, err)
	})
}

// dhctl creates ModuleConfigs in the cluster out of this struct, so an unset source must not be
// serialised at all - otherwise the synthesised deckhouse/global configs carry source: "".
func TestModuleConfigSpecSourceOmitted(t *testing.T) {
	data, err := yaml.Marshal(ModuleConfig{Spec: ModuleConfigSpec{Version: 1}})
	require.NoError(t, err)
	require.NotContains(t, string(data), "source")
}

func TestModuleDocsDefaultModuleRepo(t *testing.T) {
	// No spec.source: the module rides along in the deckhouse registry under /modules, with the
	// credentials the deckhouse ModuleConfig already carries.
	docs := []string{ensureRegistryMCDoc, `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  version: 1
`}

	md, err := ParseModuleDocs(docs)
	require.NoError(t, err)

	repo, conf, err := md.moduleRepo("cloud-provider-dvp", md.providerSource("cloud-provider-dvp"))
	require.NoError(t, err)
	require.Equal(t, "r.example.com/test/modules", repo)
	require.Equal(t, "test-user", conf.GetUsername())
}

// fakeRegistry stands in for the OCI registry the module chain talks to. It answers per
// repository, because the chain's whole point is that the release image and the module image
// live in two different repositories and are addressed by two different tags.
type fakeRegistry struct {
	images map[string]crv1.Image
	err    error

	// tags records the tag each repository was actually asked for, which is where the
	// override-beats-channel decision becomes visible.
	tags  map[string]string
	confs map[string]*image.RegistryConfig
}

type fakeRegistryClient struct {
	reg  *fakeRegistry
	repo string
}

func (c fakeRegistryClient) Image(_ context.Context, tag string) (crv1.Image, error) {
	c.reg.tags[c.repo] = tag
	if c.reg.err != nil {
		return nil, c.reg.err
	}
	img, ok := c.reg.images[c.repo]
	if !ok {
		return nil, fmt.Errorf("no such repository %q", c.repo)
	}
	return img, nil
}

func (c fakeRegistryClient) Digest(_ context.Context, _ string) (string, error) { return "", nil }
func (c fakeRegistryClient) ListTags(_ context.Context) ([]string, error)       { return nil, nil }

// stubModuleRegistry routes the cr metadata lookups at reg instead of a real registry.
func stubModuleRegistry(t *testing.T, reg *fakeRegistry) {
	t.Helper()
	reg.tags = map[string]string{}
	reg.confs = map[string]*image.RegistryConfig{}

	orig := moduleRegistryClient
	moduleRegistryClient = func(conf *image.RegistryConfig, repo string) (cr.Client, error) {
		reg.confs[repo] = conf
		return fakeRegistryClient{reg: reg, repo: repo}, nil
	}

	t.Cleanup(func() { moduleRegistryClient = orig })
}

// testImage builds a single-layer image whose root holds the given files, the way a release
// image and a module image both carry their metadata.
func testImage(t *testing.T, files map[string]string) crv1.Image {
	t.Helper()

	buf := bytes.NewBuffer(nil)
	tw := tar.NewWriter(buf)

	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(content)),
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	})
	require.NoError(t, err)

	img, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	return img
}

// stubEmbeddedDigests makes the compiled-in images_digests.json say what the test needs, so a
// fallback to it is visible instead of being an error that looks like any other.
func stubEmbeddedDigests(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "images_digests.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	t.Setenv(digests.ImagesDigestsFileEnv, path)
}

// testModuleOptions builds the options whose modules directory holds the chart of exactly the
// named providers, numeric prefix and all - the directory the build fills per edition, and the
// only thing that tells the bootstrap path which modules this image still ships.
func testModuleOptions(t *testing.T, shipped ...string) *options.GlobalOptions {
	t.Helper()

	dir := t.TempDir()
	for _, provider := range shipped {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "030-"+CloudProviderModuleName(provider)), 0o755))
	}

	return &options.GlobalOptions{ModulesDir: dir}
}

const testBundleDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000bbb"

// testDockerCfg encodes credentials for the registry host of repo, the way a ModuleSource
// carries them. The host has to match: the decoder looks the repository up by name.
func testDockerCfg(t *testing.T, repo string) string {
	t.Helper()
	host, _, _ := strings.Cut(repo, "/")
	return base64.StdEncoding.EncodeToString([]byte(
		`{"auths": {"` + host + `": {"auth": "` + base64.StdEncoding.EncodeToString([]byte("user:pass")) + `"}}}`))
}

func testModuleSourceDoc(t *testing.T, repo string) string {
	t.Helper()
	return `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: dev
spec:
  registry:
    repo: ` + repo + `
    dockerCfg: ` + testDockerCfg(t, repo) + `
    ca: test-ca-pem
`
}

const testProviderMCWithSourceDoc = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  enabled: true
  version: 1
  source: dev
`

// The whole chain, against the reference layout of the real external build: a release image
// carrying a NON-semver version.json, a module image carrying a flat images_digests.json, and
// the bundle addressed by the terraformManager digest inside the module's own repository.
func TestResolveModuleProviderBundleChain(t *testing.T) {
	const repo = "dev-registry.deckhouse.io/sys/deckhouse-oss/modules"
	const moduleRepo = repo + "/cloud-provider-dvp"

	// A fallback here would silently produce this installer's own bundle, so the embedded
	// digests are stocked with a different one to make that visible.
	stubEmbeddedDigests(t, `{"cloudProviderDvp": {"terraformManager": "sha256:embedded"}}`)

	reg := &fakeRegistry{images: map[string]crv1.Image{
		moduleRepo + "/release": testImage(t, map[string]string{
			"version.json": `{"version": "mr1"}`,
			"module.yaml":  "name: cloud-provider-dvp\n",
		}),
		moduleRepo: testImage(t, map[string]string{
			"images_digests.json": `{"validator": "sha256:aaa", "terraformManager": "` + testBundleDigest + `"}`,
		}),
	}}
	stubModuleRegistry(t, reg)

	docs := []string{
		ensureRegistryMCDoc,
		testModuleSourceDoc(t, repo),
		testProviderMCWithSourceDoc,
	}

	ref, err := resolveProviderBundleRef(context.Background(), "dvp", configModuleDocs(docs), testModuleOptions(t))
	require.NoError(t, err)

	// A dev build's version is not a semver, and "v"+version is what the controller tags the
	// module image with, so the module image must be asked for by "vmr1".
	require.Equal(t, "stable", reg.tags[moduleRepo+"/release"])
	require.Equal(t, "vmr1", reg.tags[moduleRepo])

	require.Equal(t, moduleRepo+"@"+testBundleDigest, ref.Image)
	require.Equal(t, testBundleDigest, ref.Digest)

	// The ModuleSource repo overrides <imagesRepo>/modules, and its credentials and CA are
	// what both requests and the bundle download authenticate with.
	require.Equal(t, repo, ref.Registry.GetRegistry())
	require.Equal(t, "user", ref.Registry.GetUsername())
	require.Equal(t, "pass", ref.Registry.GetPassword())
	require.Equal(t, "test-ca-pem", ref.Registry.GetCA())
	require.Equal(t, ref.Registry, reg.confs[moduleRepo+"/release"])
	require.Equal(t, ref.Registry, reg.confs[moduleRepo])
}

// Which tag the release image is asked for: an override pins the module and beats any
// channel, a channel is kebab-cased the way the release images are tagged, and with neither
// the module follows the controller's built-in Stable policy.
func TestResolveModuleProviderBundleReleaseTag(t *testing.T) {
	const moduleRepo = "r.example.com/test/modules/cloud-provider-dvp"

	// The registry settings and the release channel live in the one deckhouse ModuleConfig,
	// so the channel cases carry their own copy of it rather than a second document.
	deckhouseMC := func(channel string) string {
		return strings.Replace(ensureRegistryMCDoc, "  settings:\n", "  settings:\n    releaseChannel: "+channel+"\n", 1)
	}
	// source: deckhouse is what marks the module external without shipping a ModuleSource -
	// helm creates that source in-cluster, so it can never be in config.yml.
	providerMC := `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  version: 1
  source: deckhouse
`

	for _, tc := range []struct {
		name string
		docs []string
		tag  string
	}{
		{"no channel, no override", []string{ensureRegistryMCDoc, providerMC}, "stable"},
		{"channel", []string{deckhouseMC("EarlyAccess"), providerMC}, "early-access"},
		{"rock solid", []string{deckhouseMC("RockSolid"), providerMC}, "rock-solid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := &fakeRegistry{images: map[string]crv1.Image{
				moduleRepo + "/release": testImage(t, map[string]string{"version.json": `{"version": "1.2.3"}`}),
				moduleRepo:              testImage(t, map[string]string{"images_digests.json": `{"terraformManager": "` + testBundleDigest + `"}`}),
			}}
			stubModuleRegistry(t, reg)

			ref, _, err := resolveModuleProviderBundle(context.Background(), "dvp", configModuleDocs(tc.docs), testModuleOptions(t))
			require.NoError(t, err)
			require.Equal(t, tc.tag, reg.tags[moduleRepo+"/release"])
			// A semver version is tagged the same way, just with the "v" the build adds.
			require.Equal(t, "v1.2.3", reg.tags[moduleRepo])
			require.Equal(t, moduleRepo+"@"+testBundleDigest, ref.Image)
		})
	}
}

// No ModuleConfig for the provider means an in-tree provider, and that path must keep working
// exactly as it did: the digest comes from this installer image, with no repository and no
// registry of its own attached to it.
func TestResolveProviderBundleRefFallsBackToEmbeddedDigests(t *testing.T) {
	stubEmbeddedDigests(t, `{"cloudProviderYandex": {"terraformManager": "sha256:embedded"}}`)

	reg := &fakeRegistry{err: fmt.Errorf("the registry must not be touched at all")}
	stubModuleRegistry(t, reg)

	ref, err := resolveProviderBundleRef(context.Background(), "yandex", configModuleDocs([]string{ensureRegistryMCDoc, ensureClusterConfigDoc("Yandex")}), testModuleOptions(t, "yandex"))
	require.NoError(t, err)
	require.Equal(t, "sha256:embedded", ref.Digest)
	require.Empty(t, ref.Image)
	require.Nil(t, ref.Registry)
	require.Empty(t, reg.tags)
}

// No ModuleConfig at all is how DVP bootstraps today, out of this installer's own
// images_digests.json. Nothing in the documents asked for a module, so nothing may reach for a
// registry.
func TestResolveModuleProviderBundleNoModuleConfig(t *testing.T) {
	reg := &fakeRegistry{err: fmt.Errorf("the registry must not be touched at all")}
	stubModuleRegistry(t, reg)

	_, found, err := resolveModuleProviderBundle(context.Background(), "dvp",
		configModuleDocs([]string{ensureRegistryMCDoc, ensureClusterConfigDoc("DVP")}), testModuleOptions(t))
	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, reg.tags)
}

// Once the operator has asked for an external provider, a registry failure is the answer.
// Falling back to the embedded digests would validate the configuration against whatever
// provider schema this installer image happens to carry, which is worse than refusing.
func TestResolveProviderBundleRefRegistryFailureIsNotAFallback(t *testing.T) {
	// Stocked so that a fallback would succeed and the test would pass by accident.
	stubEmbeddedDigests(t, `{"cloudProviderDvp": {"terraformManager": "sha256:embedded"}}`)

	stubModuleRegistry(t, &fakeRegistry{err: fmt.Errorf("401 Unauthorized")})

	_, err := resolveProviderBundleRef(context.Background(), "dvp", configModuleDocs([]string{ensureRegistryMCDoc, testProviderMCWithSourceDoc, testModuleSourceDoc(t, "registry.example.io/modules")}), testModuleOptions(t))
	require.ErrorContains(t, err, "401 Unauthorized")
}

// A module image that lists no terraformManager digest has no bundle to unpack, and saying so
// beats pulling <repo>/<module>@"" and letting the registry phrase the error.
func TestResolveModuleProviderBundleNoBundleDigest(t *testing.T) {
	const moduleRepo = "r.example.com/test/modules/cloud-provider-dvp"

	stubModuleRegistry(t, &fakeRegistry{images: map[string]crv1.Image{
		moduleRepo + "/release": testImage(t, map[string]string{"version.json": `{"version": "1.0.0"}`}),
		moduleRepo:              testImage(t, map[string]string{"images_digests.json": `{"validator": "sha256:aaa"}`}),
	}})

	_, found, err := resolveModuleProviderBundle(context.Background(), "dvp", configModuleDocs([]string{ensureRegistryMCDoc, `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  version: 1
  source: deckhouse
`}), testModuleOptions(t))
	require.True(t, found)
	require.ErrorContains(t, err, "terraformManager")
}

// A ModulePullOverride pins the module to a tag its dev build actually published, and the
// controller pulls <repo>/<module>:<imageTag> straight, with no release image in between. The
// verified DVP dev artifact is tagged "mr1", so asking for ".../release:mr1" or for ":vmr1"
// would both miss - which is the whole case this path exists for.
func TestResolveModuleProviderBundlePullOverrideSkipsReleaseImage(t *testing.T) {
	const moduleRepo = "r.example.com/test/modules/cloud-provider-dvp"

	reg := &fakeRegistry{images: map[string]crv1.Image{
		moduleRepo: testImage(t, map[string]string{
			"images_digests.json": `{"terraformManager": "` + testBundleDigest + `"}`,
		}),
	}}
	stubModuleRegistry(t, reg)

	ref, found, err := resolveModuleProviderBundle(context.Background(), "dvp", configModuleDocs([]string{ensureRegistryMCDoc, `
apiVersion: deckhouse.io/v1alpha2
kind: ModulePullOverride
metadata:
  name: cloud-provider-dvp
spec:
  imageTag: mr1
`}), testModuleOptions(t))
	require.NoError(t, err)
	require.True(t, found, "an override is enough on its own to mark the module external")
	require.Equal(t, "mr1", reg.tags[moduleRepo])
	require.NotContains(t, reg.tags, moduleRepo+"/release", "the release image must not be requested")
	require.Equal(t, moduleRepo+"@"+testBundleDigest, ref.Image)
}

// "deckhouse" is the ModuleSource helm creates inside the cluster; it cannot be in config.yml,
// so naming it must resolve to <imagesRepo>/modules - what that source points at - instead of
// failing on a document the operator had no way to write.
func TestResolveModuleProviderBundleDefaultSource(t *testing.T) {
	const moduleRepo = "r.example.com/test/modules/cloud-provider-dvp"

	reg := &fakeRegistry{images: map[string]crv1.Image{
		moduleRepo + "/release": testImage(t, map[string]string{"version.json": `{"version": "1.0.0"}`}),
		moduleRepo:              testImage(t, map[string]string{"images_digests.json": `{"terraformManager": "` + testBundleDigest + `"}`}),
	}}
	stubModuleRegistry(t, reg)

	ref, found, err := resolveModuleProviderBundle(context.Background(), "dvp", configModuleDocs([]string{ensureRegistryMCDoc, `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  version: 1
  source: deckhouse
`}), testModuleOptions(t))
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, moduleRepo+"@"+testBundleDigest, ref.Image)
	require.Equal(t, "test-user", ref.Registry.GetUsername())
}

// The source lives on one ModuleConfig and the settings on another, which is a shape dhctl
// explicitly supports. Last-wins would drop the source and send the chain to the wrong registry.
func TestParseModuleDocsKeepsSourceAcrossModuleConfigs(t *testing.T) {
	md, err := ParseModuleDocs([]string{`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  enabled: true
  source: dev
`, `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  version: 1
  settings:
    zone: z1
`})
	require.NoError(t, err)
	require.Equal(t, "dev", md.providerSource("cloud-provider-dvp"))
	require.NotEmpty(t, md.ProviderConfigs["cloud-provider-dvp"].Spec.Settings, "the settings overlay must still win")
}

// An external provider's module is not in the installer image, so parseDocument files its
// ModuleConfig under ResourcesYAML rather than ModuleConfigs. Everything downstream - the
// provider settings, the layout, the provider namespace, the discovery data - reads it through
// findProviderModuleConfig, so it has to be recovered from there.
func TestRecoverExternalProviderModuleConfig(t *testing.T) {
	m := &MetaConfig{ProviderName: "dvp", ResourcesYAML: `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  version: 1
  source: deckhouse
  settings:
    nodes:
      parameters:
        layout: Standard
`}

	require.False(t, m.HasProviderModuleConfig())
	require.NoError(t, m.recoverExternalProviderModuleConfig())
	require.True(t, m.HasProviderModuleConfig())

	require.NoError(t, m.applyCloudProviderModuleSettings())
	require.Equal(t, "standard", m.Layout)
	require.NotEmpty(t, m.CloudProviderVars.Settings)

	// A validated ModuleConfig in ModuleConfigs (the in-tree path) still wins.
	inTree := &MetaConfig{
		ProviderName:  "dvp",
		ModuleConfigs: []*ModuleConfig{{ObjectMeta: metav1.ObjectMeta{Name: "cloud-provider-dvp"}}},
		ResourcesYAML: m.ResourcesYAML,
	}
	require.NoError(t, inTree.recoverExternalProviderModuleConfig())
	require.Nil(t, inTree.externalProviderModuleConfig)
}

// --- the in-cluster half of the chain -------------------------------------------------------
//
// Converge, destroy and commander have no configuration documents to read the module out of, so
// the Module object the controller maintains is the source of truth: whether the provider came
// from a ModuleSource at all, and which image of it the cluster actually runs.

const testClusterModuleRepo = "registry.example.io/modules"

type testClusterObject struct {
	gvr schema.GroupVersionResource
	obj map[string]any
}

func testModuleObject(source, version string) testClusterObject {
	return testClusterObject{ModuleGVR, map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "Module",
		"metadata":   map[string]any{"name": "cloud-provider-dvp"},
		"properties": map[string]any{"source": source, "version": version},
	}}
}

func testModuleSourceObject(t *testing.T, repo string) testClusterObject {
	t.Helper()
	return testClusterObject{ModuleSourceGVR, map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleSource",
		"metadata":   map[string]any{"name": "dev"},
		"spec": map[string]any{"registry": map[string]any{
			"repo":      repo,
			"dockerCfg": testDockerCfg(t, repo),
			"ca":        "test-ca-pem",
		}},
	}}
}

func testPullOverrideObject(tag string) testClusterObject {
	return testPullOverrideObjectWithStatus(tag, modulePullOverrideReady)
}

func testPullOverrideObjectWithStatus(tag, message string) testClusterObject {
	return testClusterObject{ModulePullOverrideGVR, map[string]any{
		"apiVersion": "deckhouse.io/v1alpha2",
		"kind":       "ModulePullOverride",
		"metadata":   map[string]any{"name": "cloud-provider-dvp"},
		"spec":       map[string]any{"imageTag": tag},
		"status":     map[string]any{"message": message},
	}}
}

// testClusterModuleLookup is the lookup the in-cluster callers build, against a fake cluster
// holding exactly the given objects.
func testClusterModuleLookup(t *testing.T, objs ...testClusterObject) providerModuleLookup {
	t.Helper()

	kubeCl := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
		ModuleGVR:             "ModuleList",
		ModuleSourceGVR:       "ModuleSourceList",
		ModulePullOverrideGVR: "ModulePullOverrideList",
	})
	for _, o := range objs {
		_, err := kubeCl.Dynamic().Resource(o.gvr).Create(t.Context(), &unstructured.Unstructured{Object: o.obj}, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	return clusterModuleDocs(func(context.Context) (*client.KubernetesClient, error) { return kubeCl, nil }, "dvp", false)
}

// An embedded module reports a blank properties.source ("Source the module was downloaded from
// (otherwise will be blank)"), and a provider that was never downloaded has no Module object at
// all. Both are the in-tree case: the embedded digests answer and no registry is touched.
func TestResolveModuleProviderBundleFromClusterNotAModule(t *testing.T) {
	for _, tc := range []struct {
		name string
		objs []testClusterObject
	}{
		// The CRD says the source of an embedded module "will be blank"; the controller
		// writes the sentinel instead, and rewrites it on every start. Both are in-tree.
		{"embedded module", []testClusterObject{testModuleObject("Embedded", "1.2.3")}},
		{"blank source", []testClusterObject{testModuleObject("", "1.2.3")}},
		{"no Module object", nil},
		// The Module keeps properties.source after its ModuleSource is deleted, and the
		// cluster must stay convergeable over a state the controller itself tolerates.
		{"source object gone", []testClusterObject{testModuleObject("dev", "1.2.3")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := &fakeRegistry{err: fmt.Errorf("the registry must not be touched at all")}
			stubModuleRegistry(t, reg)

			_, found, err := resolveModuleProviderBundle(t.Context(), "dvp", testClusterModuleLookup(t, tc.objs...), testModuleOptions(t))
			require.NoError(t, err)
			require.False(t, found)
			require.Empty(t, reg.tags)
		})
	}
}

// The whole in-cluster chain. properties.version pins the module image, so the release image is
// never asked for - the channel may have moved since the module was installed, and converge has
// to unpack the bundle this cluster is running.
func TestResolveModuleProviderBundleFromClusterVersion(t *testing.T) {
	const moduleRepo = testClusterModuleRepo + "/cloud-provider-dvp"

	// A fallback to the embedded digests would produce this installer's own bundle, so they are
	// stocked with a different one to make that visible.
	stubEmbeddedDigests(t, `{"cloudProviderDvp": {"terraformManager": "sha256:embedded"}}`)

	reg := &fakeRegistry{images: map[string]crv1.Image{
		moduleRepo: testImage(t, map[string]string{
			"images_digests.json": `{"terraformManager": "` + testBundleDigest + `"}`,
		}),
	}}
	stubModuleRegistry(t, reg)

	ref, found, err := resolveModuleProviderBundle(t.Context(), "dvp", testClusterModuleLookup(t,
		testModuleObject("dev", "1.2.3"),
		testModuleSourceObject(t, testClusterModuleRepo),
	), testModuleOptions(t))
	require.NoError(t, err)
	require.True(t, found)

	require.Equal(t, "v1.2.3", reg.tags[moduleRepo])
	require.NotContains(t, reg.tags, moduleRepo+"/release", "the installed version pins the image, so the release image must not be requested")
	require.Equal(t, moduleRepo+"@"+testBundleDigest, ref.Image)
	require.Equal(t, testBundleDigest, ref.Digest)

	// The ModuleSource's own registry - credentials and CA included - is what the metadata
	// requests and the bundle download authenticate with, not the cluster's deckhouse registry.
	require.Equal(t, testClusterModuleRepo, ref.Registry.GetRegistry())
	require.Equal(t, "user", ref.Registry.GetUsername())
	require.Equal(t, "pass", ref.Registry.GetPassword())
	require.Equal(t, "test-ca-pem", ref.Registry.GetCA())
	require.Equal(t, ref.Registry, reg.confs[moduleRepo])
}

// A ModulePullOverride in the cluster keeps the controller pulling its tag, so it outranks the
// version the Module object reports - exactly as it outranks the channel at bootstrap.
func TestResolveModuleProviderBundleFromClusterPullOverride(t *testing.T) {
	const moduleRepo = testClusterModuleRepo + "/cloud-provider-dvp"

	reg := &fakeRegistry{images: map[string]crv1.Image{
		moduleRepo: testImage(t, map[string]string{
			"images_digests.json": `{"terraformManager": "` + testBundleDigest + `"}`,
		}),
	}}
	stubModuleRegistry(t, reg)

	ref, found, err := resolveModuleProviderBundle(t.Context(), "dvp", testClusterModuleLookup(t,
		testModuleObject("dev", "1.2.3"),
		testModuleSourceObject(t, testClusterModuleRepo),
		testPullOverrideObject("mr1"),
	), testModuleOptions(t))
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "mr1", reg.tags[moduleRepo])
	require.Equal(t, moduleRepo+"@"+testBundleDigest, ref.Image)
}

// Every cloud-provider-* ModuleConfig used to land in one slot, so a source written for one
// provider was grafted onto the next one's bare ModuleConfig - and the provider that declared
// it was dropped. Leftovers from the cloud-provider ModuleConfig migration make two of them in
// one configuration a normal shape.
func TestParseModuleDocsSourceDoesNotCrossModules(t *testing.T) {
	md, err := ParseModuleDocs([]string{`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  enabled: true
  source: ext-src
`, `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-openstack
spec:
  enabled: true
`})
	require.NoError(t, err)

	require.Equal(t, "ext-src", md.providerSource("cloud-provider-dvp"))
	require.Empty(t, md.providerSource("cloud-provider-openstack"), "a bare ModuleConfig must not inherit another provider's source")
}

// The in-cluster dhctl (terraform-state-exporter, terraform-auto-converger) runs under a
// ClusterRole that enumerates what it may read, and an existing cluster keeps the old role
// until its module is upgraded. A denied read says the same thing a missing object does: ask
// the mounted digests instead, which is what that pod did before the module chain existed.
func TestResolveModuleProviderBundleFromClusterDenied(t *testing.T) {
	reg := &fakeRegistry{err: fmt.Errorf("the registry must not be touched at all")}
	stubModuleRegistry(t, reg)

	kubeCl := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
		ModuleGVR: "ModuleList",
	})
	kubeCl.Dynamic().(*dynamicfake.FakeDynamicClient).PrependReactor("get", "modules",
		func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "deckhouse.io", Resource: "modules"}, "cloud-provider-dvp", fmt.Errorf("no"))
		})

	_, found, err := resolveModuleProviderBundle(t.Context(), "dvp",
		clusterModuleDocs(func(context.Context) (*client.KubernetesClient, error) { return kubeCl, nil }, "dvp", true), testModuleOptions(t))
	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, reg.tags)
}

// properties.version is a release version for a module installed from a release, and the
// override's raw imageTag for one restored from a ModulePullOverride - which deleting the
// override never resets. Only the former is tagged "v"+version.
func TestModuleVersionTag(t *testing.T) {
	require.Equal(t, "v1.2.3", moduleVersionTag("1.2.3"))
	require.Equal(t, "v1.2.3", moduleVersionTag("v1.2.3"))
	require.Equal(t, "mr1", moduleVersionTag("mr1"))
	require.Empty(t, moduleVersionTag(""))
}

// The controller follows an override only while it is Ready, so an override whose tag does not
// resolve leaves the cluster on the version it had - and that is the bundle converge validates
// against.
func TestResolveModuleProviderBundleFromClusterUnreadyPullOverride(t *testing.T) {
	const moduleRepo = testClusterModuleRepo + "/cloud-provider-dvp"

	reg := &fakeRegistry{images: map[string]crv1.Image{
		moduleRepo: testImage(t, map[string]string{
			"images_digests.json": `{"terraformManager": "` + testBundleDigest + `"}`,
		}),
	}}
	stubModuleRegistry(t, reg)

	_, found, err := resolveModuleProviderBundle(t.Context(), "dvp", testClusterModuleLookup(t,
		testModuleObject("dev", "1.2.3"),
		testModuleSourceObject(t, testClusterModuleRepo),
		testPullOverrideObjectWithStatus("typo", "Failed to pull module"),
	), testModuleOptions(t))
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "v1.2.3", reg.tags[moduleRepo], "an override the cluster is not running must not pin the bundle")
}

// Defaulting to Stable here would unpack whatever that channel points at today and validate the
// configuration against schemas from a release this cluster is not on.
func TestResolveModuleProviderBundleFromClusterNoVersion(t *testing.T) {
	reg := &fakeRegistry{err: fmt.Errorf("the registry must not be touched at all")}
	stubModuleRegistry(t, reg)

	_, _, err := resolveModuleProviderBundle(t.Context(), "dvp", testClusterModuleLookup(t,
		testModuleObject("dev", ""),
		testModuleSourceObject(t, testClusterModuleRepo),
	), testModuleOptions(t))
	require.ErrorContains(t, err, "neither properties.version nor properties.releaseChannel")
}

// On a cluster running the in-cluster registry the built-in ModuleSource points at
// registry.d8-system.svc, which only resolves inside the cluster. An out-of-cluster caller has
// to pull the same path from the upstream registry instead.
func TestModuleDocsFromClusterSubstitutesUpstreamRegistry(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
		ModuleGVR:       "ModuleList",
		ModuleSourceGVR: "ModuleSourceList",
	})
	for _, o := range []testClusterObject{
		testModuleObject("dev", "1.2.3"),
		testModuleSourceObject(t, registry_const.HostWithPath+"/modules"),
	} {
		_, err := kubeCl.Dynamic().Resource(o.gvr).Create(t.Context(), &unstructured.Unstructured{Object: o.obj}, metav1.CreateOptions{})
		require.NoError(t, err)
	}
	_, err := kubeCl.CoreV1().Secrets("d8-system").Create(t.Context(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "registry-config", Namespace: "d8-system"},
		Data: map[string][]byte{
			"imagesRepo": []byte("upstream.example.io/deckhouse/ee"),
			"scheme":     []byte("HTTPS"),
			"username":   []byte("user"),
			"password":   []byte("pass"),
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	md, err := moduleDocsFromCluster(t.Context(), kubeCl, "dvp", false)
	require.NoError(t, err)
	require.Len(t, md.Sources, 1)
	require.Equal(t, "upstream.example.io/deckhouse/ee/modules", md.Sources[0].Spec.Registry.Repo)

	conf, err := md.Sources[0].RegistryConfig()
	require.NoError(t, err)
	require.Equal(t, "upstream.example.io/deckhouse/ee/modules", conf.GetRegistry())
	require.Equal(t, "user", conf.GetUsername())

	// In the cluster the mirror is the fast local path and stays untouched.
	md, err = moduleDocsFromCluster(t.Context(), kubeCl, "dvp", true)
	require.NoError(t, err)
	require.Equal(t, registry_const.HostWithPath+"/modules", md.Sources[0].Spec.Registry.Repo)
}
