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

package apiserver

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kube-openapi/pkg/util"
	"k8s.io/kube-openapi/pkg/validation/spec"

	registryimagev1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/registry.deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-controller/internal/images"
)

type answers struct {
	answers []images.Answer
	err     error
}

func (a answers) Collect(context.Context) ([]images.Answer, error) { return a.answers, a.err }

const (
	prometheus = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	pulled     = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
)

func cache() answers {
	at := metav1.Now()
	return answers{answers: []images.Answer{{Node: "master-0", Index: &registryimagev1alpha1.ReplicaIndex{
		Node: "master-0", Ready: true, SetKnown: true, IndexedAt: &at,
		Images: []registryimagev1alpha1.IndexedImage{
			{Digest: prometheus, State: registryimagev1alpha1.ImagePresent, InSet: true, Size: 3 << 20,
				Owners: []registryimagev1alpha1.ImageOwner{{Kind: registryimagev1alpha1.OwnerPlatform, Module: "prometheus", Image: "prometheus", Versions: []string{"v1.70.1"}}}},
			{Digest: pulled, State: registryimagev1alpha1.ImagePresent},
		},
	}}}}
}

func list(t *testing.T, storage *ImageStorage, options *metainternalversion.ListOptions) []registryimagev1alpha1.RegistryImage {
	t.Helper()
	object, err := storage.List(context.Background(), options)
	require.NoError(t, err)
	return object.(*registryimagev1alpha1.RegistryImageList).Items
}

func TestImagesAreListedBySelectors(t *testing.T) {
	storage := &ImageStorage{Collector: cache()}

	assert.Len(t, list(t, storage, nil), 2)

	selected := list(t, storage, &metainternalversion.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{registryimagev1alpha1.ModuleLabel: "prometheus"}),
	})
	require.Len(t, selected, 1)
	assert.Equal(t, prometheus, selected[0].Digest)

	selected = list(t, storage, &metainternalversion.ListOptions{FieldSelector: fields.OneTermEqualSelector("inSet", "false")})
	require.Len(t, selected, 1)
	assert.Equal(t, pulled, selected[0].Digest)

	// No continue token can be given, so a limit must not cut the list: kubectl would take the first
	// page for all of it.
	assert.Len(t, list(t, storage, &metainternalversion.ListOptions{Limit: 1}), 2)

	_, err := storage.List(context.Background(), &metainternalversion.ListOptions{FieldSelector: fields.OneTermEqualSelector("spec.size", "1")})
	assert.True(t, apierrors.IsBadRequest(err))
}

func TestAnImageIsReadByItsName(t *testing.T) {
	storage := &ImageStorage{Collector: cache()}

	object, err := storage.Get(context.Background(), registryimagev1alpha1.NameOf(prometheus), nil)
	require.NoError(t, err)
	assert.Equal(t, prometheus, object.(*registryimagev1alpha1.RegistryImage).Digest)

	_, err = storage.Get(context.Background(), "prometheus", nil)
	assert.True(t, apierrors.IsNotFound(err))
	_, err = storage.Get(context.Background(), registryimagev1alpha1.NameOf("sha256:00"), nil)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestWithoutACacheThereIsNothingAndWithoutAnAnswerTheServiceIsUnavailable(t *testing.T) {
	assert.Empty(t, list(t, &ImageStorage{Collector: answers{err: images.ErrNoCache}}, nil))

	_, err := (&ImageStorage{Collector: answers{err: errors.New("the API server is away")}}).List(context.Background(), nil)
	assert.True(t, apierrors.IsServiceUnavailable(err))
}

// An image two modules share carries the first of them in its label, and is found by either.
func TestAnImageIsSelectedByAnyOfItsModules(t *testing.T) {
	shared := cache()
	shared.answers[0].Index.Images[0].Owners = append(shared.answers[0].Index.Images[0].Owners,
		registryimagev1alpha1.ImageOwner{Kind: registryimagev1alpha1.OwnerPlatform, Module: "userAuthz", Image: "webhook"})
	storage := &ImageStorage{Collector: shared}

	for _, module := range []string{"prometheus", "userAuthz"} {
		selected := list(t, storage, &metainternalversion.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{registryimagev1alpha1.ModuleLabel: module}),
		})
		require.Len(t, selected, 1, module)
		assert.Equal(t, prometheus, selected[0].Digest)
	}
	assert.Empty(t, list(t, storage, &metainternalversion.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{registryimagev1alpha1.ModuleLabel: "istio"}),
	}))

	// Ruling a module out rules out every image it has a share in, whichever module the label names.
	// An image of no module, the pulled one, has no module to rule out.
	module := registryimagev1alpha1.ModuleLabel
	for selector, want := range map[string][]string{
		module + "!=userAuthz":           {pulled},
		module + "!=prometheus":          {pulled},
		module + " notin (istio)":        {prometheus, pulled},
		module + " in (userAuthz,istio)": {prometheus},
		"!" + module:                     {pulled},
		module + ",other!=value":         {prometheus},
	} {
		parsed, err := labels.Parse(selector)
		require.NoError(t, err, selector)
		var digests []string
		for _, image := range list(t, storage, &metainternalversion.ListOptions{LabelSelector: parsed}) {
			digests = append(digests, image.Digest)
		}
		assert.ElementsMatch(t, want, digests, selector)
	}
}

// With no replica able to answer, an empty list would read as an empty cache.
func TestWithNoReplicaAnsweringTheServiceIsUnavailable(t *testing.T) {
	for name, collector := range map[string]answers{
		"no replica yet": {answers: []images.Answer{}},
		"none answered": {answers: []images.Answer{
			{Node: "master-0", Err: errors.New("connection refused")},
			{Node: "master-1", Index: &registryimagev1alpha1.ReplicaIndex{Node: "master-1"}},
		}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := (&ImageStorage{Collector: collector}).List(context.Background(), nil)
			require.Error(t, err)
			assert.True(t, apierrors.IsServiceUnavailable(err))
		})
	}

	// One answer is enough: the silent replica is Unknown on every image.
	partial := cache()
	partial.answers = append(partial.answers, images.Answer{Node: "master-1", Err: errors.New("connection refused")})
	listed := list(t, &ImageStorage{Collector: partial}, nil)
	require.Len(t, listed, 2)
	assert.Equal(t, registryimagev1alpha1.ImageUnknown, listed[0].State)
}

func TestTheTableNamesTheComponentAndTheState(t *testing.T) {
	storage := &ImageStorage{Collector: cache()}
	object, err := storage.List(context.Background(), nil)
	require.NoError(t, err)

	table, err := storage.ConvertToTable(context.Background(), object, nil)
	require.NoError(t, err)
	require.Len(t, table.Rows, 2)
	assert.Equal(t, []interface{}{
		registryimagev1alpha1.NameOf(prometheus), "prometheus", "prometheus", "Present", "1/1", true,
		"3.0Mi", "0", "v1.70.1", "",
	}, table.Rows[0].Cells)
	assert.Len(t, table.ColumnDefinitions, len(table.Rows[0].Cells))
}

func TestTheSchemaSaysWhatTheTwoSizesMean(t *testing.T) {
	definitions := openAPIDefinitions()(func(path string) spec.Ref { return spec.MustCreateRef("#/definitions/" + path) })

	image, found := definitions[util.GetCanonicalTypeName(&registryimagev1alpha1.RegistryImage{})]
	require.True(t, found)
	properties := image.Schema.Properties
	assert.Contains(t, properties["size"].Description, "a layer several images share is stored once")
	assert.Contains(t, properties["reclaimableSize"].Description, "no other image references")
	assert.ElementsMatch(t, []interface{}{"Present", "Missing", "Incomplete", "Corrupted", "Unknown"}, properties["state"].Enum)

	// Every type the server carries has a definition, or the spec builder stops the process.
	for _, known := range Scheme.AllKnownTypes() {
		_, found := definitions[util.GetCanonicalTypeName(reflect.New(known).Interface())]
		assert.True(t, found, known.String())
	}
}

// The generic list handler converts every field selector through the scheme before List runs, and
// refuses what the scheme does not know. Calling List directly passes by that step, which is how a
// selector on state or inSet was refused on a cluster while every test of List passed.
func TestTheSchemeAcceptsEveryFieldListSelectsBy(t *testing.T) {
	kind := registryimagev1alpha1.GroupVersion.WithKind(registryimagev1alpha1.RegistryImageKind)
	for _, field := range []string{"metadata.name", "state", "inSet"} {
		label, value, err := Scheme.ConvertFieldLabel(kind, field, "x")
		require.NoError(t, err, field)
		assert.Equal(t, field, label)
		assert.Equal(t, "x", value)
	}
	_, _, err := Scheme.ConvertFieldLabel(kind, "spec.size", "1")
	assert.Error(t, err)
}
