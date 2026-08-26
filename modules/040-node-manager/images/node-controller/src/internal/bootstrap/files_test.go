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

package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLoadFilesResolvesBothSpellings(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "d8-cloud-instance-manager", Name: TemplatesConfigMapName},
		Data: map[string]string{
			"lib.sh.tpl":                       "LIB",
			"bootstrap-networks-yandex.sh.tpl": "YANDEX",
		},
		BinaryData: map[string][]byte{"minget": {0x7f, 'E', 'L', 'F'}},
	}).Build()

	f, err := LoadFiles(t.Context(), c)
	require.NoError(t, err)

	// Templates try the absolute path and fall back to the relative one:
	// `.Files.Get "/deckhouse/candi/..." | default (.Files.Get "candi/...")`.
	assert.Equal(t, "LIB", f.Get("/deckhouse/candi/bashible/lib.sh.tpl"))
	assert.Equal(t, "LIB", f.Get("candi/bashible/lib.sh.tpl"))

	// A provider path resolves to the flat provider-prefixed key, in both
	// spellings 01-bootstrap-prerequisites.sh.tpl uses.
	assert.Equal(t, "YANDEX", f.Get("deckhouse/candi/cloud-providers/yandex/bashible/bootstrap-networks.sh.tpl"))
	assert.Equal(t, "YANDEX", f.Get("candi/cloud-providers/yandex/bashible/bootstrap-networks.sh.tpl"))
	assert.Equal(t, "", f.Get("candi/cloud-providers/aws/bashible/bootstrap-networks.sh.tpl"))

	// A missing file yields an empty string, not an error: `default` relies on it.
	assert.Equal(t, "", f.Get("candi/bashible/nope.tpl"))

	assert.Equal(t, []byte{0x7f, 'E', 'L', 'F'}, f.Binary("minget"))
}

func TestLoadFilesFailsWithoutConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	// A missing ConfigMap is an error, not an empty set: otherwise the render
	// silently emits a script without the library and the node never comes up.
	_, err := LoadFiles(t.Context(), c)
	require.Error(t, err)
}

func TestLoadFilesFailsWithoutLibrary(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "d8-cloud-instance-manager", Name: TemplatesConfigMapName},
		Data:       map[string]string{"bb_node_ip.sh.tpl": "IP"},
	}).Build()

	// A ConfigMap without lib.sh.tpl is as fatal as a missing one: every other
	// template is rendered on top of the library.
	_, err := LoadFiles(t.Context(), c)
	require.Error(t, err)
}
