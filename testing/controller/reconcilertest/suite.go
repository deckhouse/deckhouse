// Copyright 2025 Flant JSC
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

// Package reconcilertest is a reusable test framework for controller-runtime
// reconcilers. It factors out the scaffolding that used to be copy-pasted across
// every controller test in deckhouse-controller:
//
//   - a shared -golden flag with write/compare logic (see golden.go);
//   - scheme-based YAML fixture decoding and seeding (see seed.go), which
//     replaces the hand-written `switch obj.Kind` blocks;
//   - golden snapshots of cluster state (see snapshot.go);
//   - registry/OCI and HTTP fakes (see regmock.go, httpmock.go);
//   - an embeddable testify Suite (this file) that ties the above together.
//
// A controller test embeds Suite, calls Init once with a Config describing which
// resources to seed and snapshot, and uses Seed/Client to drive its own private
// reconciler. The framework's TearDownSubTest compares the resulting cluster
// state against a golden file automatically.
package reconcilertest

import (
	"context"
	"flag"
	"path/filepath"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/go_lib/project"
)

// Config declares the static, per-suite behaviour of the framework.
type Config struct {
	// Scheme used to decode fixtures, build the client and snapshot results.
	// Defaults to the shared project.Scheme() (v1alpha1, v1alpha2, core, apps,
	// coordination, apiextensions), which covers every deckhouse-controller CRD.
	Scheme *runtime.Scheme

	// StatusSubresources are registered on the fake client so that status
	// updates from the reconciler are persisted (mirrors WithStatusSubresource).
	StatusSubresources []client.Object

	// SnapshotKinds lists the resource kinds dumped into the golden snapshot,
	// in order.
	SnapshotKinds []schema.GroupVersionKind

	// ObjectNormalizers / BytesNormalizers stabilise non-deterministic fields in
	// the snapshot before comparison.
	ObjectNormalizers []ObjectNormalizer
	BytesNormalizers  []BytesNormalizer

	// GoldenMode selects per-document or whole-document comparison.
	GoldenMode Mode

	// WithDynamic also builds a dynamic fake client and a static RESTMapper,
	// available via Dynamic() and RESTMapper(); needed by controllers that read
	// arbitrary resources (e.g. objectkeeper).
	WithDynamic bool

	// SeedViaCreate seeds the cluster by calling Create on an empty client
	// instead of pre-loading objects via WithObjects. This matches suites whose
	// golden files were generated that way (resourceVersion sequencing differs
	// between the two approaches).
	SeedViaCreate bool

	// TestdataDir is the fixtures directory (default "./testdata"). Golden files
	// live in <TestdataDir>/<GoldenSubdir>.
	TestdataDir  string
	GoldenSubdir string

	// SkipTestEnv disables setting D8_IS_TESTS_ENVIRONMENT=true during Init.
	SkipTestEnv bool
}

// Suite is an embeddable testify suite providing the framework's building blocks.
type Suite struct {
	suite.Suite

	cfg        Config
	scheme     *runtime.Scheme
	cl         client.Client
	dyn        dynamic.Interface
	restMapper meta.RESTMapper

	fixtureName string
}

// Init stores the configuration, applies defaults, parses test flags and (unless
// disabled) marks the process as a test environment. Call it once, typically from
// the embedding suite's SetupSuite.
func (s *Suite) Init(cfg Config) {
	if cfg.Scheme == nil {
		sc, err := project.Scheme()
		require.NoError(s.T(), err)
		cfg.Scheme = sc
	}
	if cfg.TestdataDir == "" {
		cfg.TestdataDir = "./testdata"
	}
	if cfg.GoldenSubdir == "" {
		cfg.GoldenSubdir = "golden"
	}

	s.cfg = cfg
	s.scheme = cfg.Scheme

	flag.Parse()
	if !cfg.SkipTestEnv {
		s.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
	}
}

// Scheme returns the scheme used by the suite.
func (s *Suite) Scheme() *runtime.Scheme { return s.scheme }

// Client returns the fake controller-runtime client seeded by Seed*.
func (s *Suite) Client() client.Client { return s.cl }

// Dynamic returns the dynamic fake client (only when Config.WithDynamic is set).
func (s *Suite) Dynamic() dynamic.Interface { return s.dyn }

// RESTMapper returns the static RESTMapper (only when Config.WithDynamic is set).
func (s *Suite) RESTMapper() meta.RESTMapper { return s.restMapper }

// FixtureName returns the name of the currently loaded fixture; it is also the
// golden file name.
func (s *Suite) FixtureName() string { return s.fixtureName }

// Decode turns a YAML blob into typed objects using the suite scheme.
func (s *Suite) Decode(raw []byte) []client.Object {
	objs, err := Decode(s.scheme, raw)
	require.NoError(s.T(), err)
	return objs
}

// Seed loads <TestdataDir>/<name>, decodes it and builds the client. The name is
// also recorded as the golden file name. An empty or missing-content fixture
// results in an empty cluster, matching the legacy behaviour.
func (s *Suite) Seed(name string) {
	raw, err := LoadFixture(s.cfg.TestdataDir, name)
	require.NoError(s.T(), err)
	s.SeedRaw(name, raw)
}

// SeedRaw decodes an already-produced YAML blob (e.g. rendered from a template)
// and builds the client, recording name as the golden file name.
func (s *Suite) SeedRaw(name string, raw []byte) {
	s.fixtureName = name
	s.SeedObjects(name, s.Decode(raw)...)
}

// SeedObjects builds the client from pre-decoded objects, recording name as the
// golden file name.
func (s *Suite) SeedObjects(name string, objs ...client.Object) {
	s.fixtureName = name

	builder := fake.NewClientBuilder().WithScheme(s.scheme)
	if !s.cfg.SeedViaCreate {
		builder = builder.WithObjects(objs...)
	}
	if len(s.cfg.StatusSubresources) > 0 {
		builder = builder.WithStatusSubresource(s.cfg.StatusSubresources...)
	}

	if s.cfg.WithDynamic {
		s.restMapper = testrestmapper.TestOnlyStaticRESTMapper(s.scheme)
		builder = builder.WithRESTMapper(s.restMapper)
	}

	s.cl = builder.Build()

	if s.cfg.SeedViaCreate {
		for _, obj := range objs {
			require.NoError(s.T(), s.cl.Create(context.TODO(), obj))
		}
	}

	if s.cfg.WithDynamic {
		s.dyn = s.buildDynamic(objs)
	}
}

func (s *Suite) buildDynamic(objs []client.Object) dynamic.Interface {
	runtimeObjs := make([]runtime.Object, 0, len(objs))
	for _, obj := range objs {
		u := &unstructured.Unstructured{}
		if err := s.scheme.Convert(obj, u, nil); err == nil {
			runtimeObjs = append(runtimeObjs, u)
		}
	}

	return dynamicfake.NewSimpleDynamicClient(s.scheme, runtimeObjs...)
}

// Request builds a reconcile request for the given name/namespace.
func (s *Suite) Request(name, namespace string) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: namespace}}
}

// AssertGolden snapshots the current cluster state and compares it (or updates
// the golden file when -golden is set). It is a no-op for skipped subtests.
func (s *Suite) AssertGolden() {
	if s.T().Skipped() {
		return
	}

	got, err := Snapshot(context.TODO(), s.cl, s.scheme, SnapshotSpec{
		Kinds:             s.cfg.SnapshotKinds,
		ObjectNormalizers: s.cfg.ObjectNormalizers,
		BytesNormalizers:  s.cfg.BytesNormalizers,
	})
	require.NoError(s.T(), err)

	goldenPath := filepath.Join(s.cfg.TestdataDir, s.cfg.GoldenSubdir, s.fixtureName)
	CompareOrUpdate(s.T(), goldenPath, got, s.cfg.GoldenMode)
}

// TearDownSubTest runs the golden assertion after every subtest. Embedding suites
// that need a custom TearDownSubTest should call AssertGolden() themselves.
func (s *Suite) TearDownSubTest() {
	s.AssertGolden()
}

var _ suite.TearDownSubTest = (*Suite)(nil)
