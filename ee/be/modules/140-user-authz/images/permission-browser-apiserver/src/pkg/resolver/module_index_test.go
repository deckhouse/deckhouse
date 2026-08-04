/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	metadatafake "k8s.io/client-go/metadata/fake"
)

func crdMetadata(name string, labels map[string]string) *metav1.PartialObjectMetadata {
	return &metav1.PartialObjectMetadata{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
	}
}

func newTestModuleIndex(objects ...runtime.Object) *ModuleIndex {
	scheme := metadatafake.NewTestScheme()
	_ = metav1.AddMetaToScheme(scheme)

	return NewModuleIndex(metadatafake.NewSimpleMetadataClient(scheme, objects...))
}

// The API group does not name the module: operator-trivy ships
// aquasecurity.github.io, and grouping a coverage report by group would file it
// under a vendor nobody in the cluster is responsible for.
func TestModuleIndex_ReadsTheModuleFromTheCRD(t *testing.T) {
	t.Parallel()

	index := newTestModuleIndex(
		crdMetadata("vulnerabilityreports.aquasecurity.github.io", map[string]string{"heritage": "deckhouse", "module": "operator-trivy"}),
		crdMetadata("projects.deckhouse.io", map[string]string{"heritage": "deckhouse", "module": "multitenancy-manager"}),
	)

	origin, known := index.Origin("aquasecurity.github.io", "vulnerabilityreports")

	assert.True(t, known)
	assert.Equal(t, "operator-trivy", origin.Module)
	assert.False(t, origin.Custom)
}

// A subresource is served by whoever ships its parent, and a coverage report
// that files pods/exec apart from pods splits one module in two.
func TestModuleIndex_SubresourceBelongsToItsParent(t *testing.T) {
	t.Parallel()

	index := newTestModuleIndex(
		crdMetadata("virtualmachines.virtualization.deckhouse.io", map[string]string{"heritage": "deckhouse", "module": "virtualization"}),
	)

	origin, known := index.Origin("virtualization.deckhouse.io", "virtualmachines/console")

	assert.True(t, known)
	assert.Equal(t, "virtualization", origin.Module)
}

// The checkbox that leaves out platform resources needs a criterion that does
// not depend on the API group: a customer CRD can live under any of them.
func TestModuleIndex_CustomIsWhatThePlatformDoesNotInstall(t *testing.T) {
	t.Parallel()

	index := newTestModuleIndex(
		crdMetadata("widgets.example.com", nil),
		crdMetadata("projects.deckhouse.io", map[string]string{"heritage": "deckhouse", "module": "multitenancy-manager"}),
	)

	custom, known := index.Origin("example.com", "widgets")
	assert.True(t, known)
	assert.True(t, custom.Custom)
	assert.Empty(t, custom.Module)

	platform, known := index.Origin("deckhouse.io", "projects")
	assert.True(t, known)
	assert.False(t, platform.Custom)
}

// Built-in and aggregated APIs have no CRD at all: the index has to say it does
// not know them rather than call them custom.
func TestModuleIndex_KnowsNothingAboutBuiltInResources(t *testing.T) {
	t.Parallel()

	index := newTestModuleIndex(crdMetadata("projects.deckhouse.io", map[string]string{"heritage": "deckhouse"}))

	_, known := index.Origin("", "secrets")

	assert.False(t, known)
}
