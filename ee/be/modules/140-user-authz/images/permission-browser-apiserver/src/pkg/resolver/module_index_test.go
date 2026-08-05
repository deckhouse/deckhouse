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

func apiServiceMetadata(name string, labels map[string]string) *metav1.PartialObjectMetadata {
	return &metav1.PartialObjectMetadata{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apiregistration.k8s.io/v1", Kind: "APIService"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
	}
}

func newTestModuleIndex(t *testing.T, objects ...runtime.Object) *ModuleIndex {
	t.Helper()

	scheme := metadatafake.NewTestScheme()
	_ = metav1.AddMetaToScheme(scheme)

	return NewModuleIndex(t.Context(), metadatafake.NewSimpleMetadataClient(scheme, objects...))
}

// The API group does not name the module: operator-trivy ships
// aquasecurity.github.io, and grouping a coverage report by group would file it
// under a vendor nobody in the cluster is responsible for.
func TestModuleIndex_ReadsTheModuleFromTheCRD(t *testing.T) {
	t.Parallel()

	index := newTestModuleIndex(t, crdMetadata("vulnerabilityreports.aquasecurity.github.io", map[string]string{"heritage": "deckhouse", "module": "operator-trivy"}),
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

	index := newTestModuleIndex(t, crdMetadata("virtualmachines.virtualization.deckhouse.io", map[string]string{"heritage": "deckhouse", "module": "virtualization"}))

	origin, known := index.Origin("virtualization.deckhouse.io", "virtualmachines/console")

	assert.True(t, known)
	assert.Equal(t, "virtualization", origin.Module)
}

// The checkbox that leaves out platform resources needs a criterion that does
// not depend on the API group: a customer CRD can live under any of them.
func TestModuleIndex_CustomIsWhatThePlatformDoesNotInstall(t *testing.T) {
	t.Parallel()

	index := newTestModuleIndex(t, crdMetadata("widgets.example.com", nil),
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

// Dex keeps its sessions and passwords in resources of its own, and nobody is
// meant to be granted them. A coverage review has nothing to look for there, so
// the report has to say which kinds are like that -- and which, on the contrary,
// hold the configuration a human writes.
func TestModuleIndex_TellsServiceKindsFromConfigurationOnes(t *testing.T) {
	t.Parallel()

	index := newTestModuleIndex(t,
		crdMetadata("dexauthenticators.deckhouse.io", map[string]string{
			"backup.deckhouse.io/cluster-config": "true",
			"heritage":                           "deckhouse",
			"module":                             "user-authn",
		}),
		crdMetadata("passwords.dex.coreos.com", map[string]string{"deckhouse.io/system-resource": "true", "heritage": "deckhouse"}),
		crdMetadata("offlinesessionses.dex.coreos.com", map[string]string{"heritage": "deckhouse"}),
	)

	config, _ := index.Origin("deckhouse.io", "dexauthenticators")
	assert.True(t, config.ClusterConfig)
	assert.False(t, config.System)

	service, _ := index.Origin("dex.coreos.com", "passwords")
	assert.True(t, service.System)
	assert.False(t, service.ClusterConfig)

	// Ничем не помеченный ресурс остаётся обычным: отчёт не догадывается за платформу.
	plain, known := index.Origin("dex.coreos.com", "offlinesessionses")
	assert.True(t, known)
	assert.False(t, plain.System)
	assert.False(t, plain.ClusterConfig)
}

// Built-in APIs have no CRD and no APIService of a module: the index has to say
// it does not know them rather than call them custom or invent a module.
func TestModuleIndex_KnowsNothingAboutBuiltInResources(t *testing.T) {
	t.Parallel()

	index := newTestModuleIndex(t, crdMetadata("projects.deckhouse.io", map[string]string{"heritage": "deckhouse"}))

	_, known := index.Origin("", "secrets")

	assert.False(t, known)
}

// An aggregated API has no CRD, so without the APIService its group would be
// filed under a made-up module: authorization.deckhouse.io comes from user-authz,
// and "authorization" is not a module.
func TestModuleIndex_AggregatedAPIComesFromItsAPIService(t *testing.T) {
	t.Parallel()

	index := newTestModuleIndex(t, apiServiceMetadata("v1alpha1.authorization.deckhouse.io", map[string]string{"heritage": "deckhouse", "module": "user-authz"}),
		// The local APIServices of the built-in APIs carry no group and no module.
		apiServiceMetadata("v1.", nil),
	)

	origin, known := index.Origin("authorization.deckhouse.io", "roleaccessreports")

	assert.True(t, known)
	assert.Equal(t, "user-authz", origin.Module)
	assert.False(t, origin.Custom, "an aggregated API of a module is not a resource of the cluster owner")
}

// A CRD wins over the APIService of its group: the module that owns the CRD is
// the one responsible for the resource.
func TestModuleIndex_CRDWinsOverTheGroup(t *testing.T) {
	t.Parallel()

	index := newTestModuleIndex(t, crdMetadata("projects.deckhouse.io", map[string]string{"heritage": "deckhouse", "module": "multitenancy-manager"}),
		apiServiceMetadata("v1alpha1.deckhouse.io", map[string]string{"module": "deckhouse"}),
	)

	origin, _ := index.Origin("deckhouse.io", "projects")

	assert.Equal(t, "multitenancy-manager", origin.Module)
}
