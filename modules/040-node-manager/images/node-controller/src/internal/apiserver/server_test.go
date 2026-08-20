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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	utilcompatibility "k8s.io/apiserver/pkg/util/compatibility"
	restclient "k8s.io/client-go/rest"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// stubStorage is the minimal read-only storage the installer accepts.
type stubStorage struct{}

func (stubStorage) New() runtime.Object { return &internalv1alpha1.NodeConfig{} }

func (stubStorage) Destroy() {}

func (stubStorage) NamespaceScoped() bool { return false }

func (stubStorage) GetSingularName() string { return "nodeconfigtemplate" }

func (stubStorage) Get(_ context.Context, name string, _ *metav1.GetOptions) (runtime.Object, error) {
	return nil, apierrors.NewNotFound(schema.GroupResource{
		Group:    internalv1alpha1.GroupVersion.Group,
		Resource: "nodeconfigtemplates",
	}, name)
}

func TestServesInternalDeckhouseGroup(t *testing.T) {
	cfg := genericapiserver.NewRecommendedConfig(Codecs)
	cfg.EffectiveVersion = utilcompatibility.DefaultBuildEffectiveVersion()
	cfg.ExternalAddress = "127.0.0.1:443"
	cfg.LoopbackClientConfig = &restclient.Config{Host: "127.0.0.1"}

	srv, err := newServer(cfg, map[string]rest.Storage{"nodeconfigtemplates": stubStorage{}})
	require.NoError(t, err)

	ts := httptest.NewServer(srv.Handler)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/apis/internal.deckhouse.io/v1alpha1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var list metav1.APIResourceList
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&list))
	require.Equal(t, "internal.deckhouse.io/v1alpha1", list.GroupVersion)

	names := make([]string, 0, len(list.APIResources))
	for _, r := range list.APIResources {
		names = append(names, r.Name)
	}
	require.Contains(t, names, "nodeconfigtemplates")
}
