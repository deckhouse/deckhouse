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

package impersonate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// apiServer answers every read with a ConfigMap and records the user each request impersonates and
// the token it authenticates with.
type apiServer struct {
	mu            sync.Mutex
	impersonated  []string
	authorization []string
}

func (s *apiServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.impersonated = append(s.impersonated, r.Header.Get("Impersonate-User"))
	s.authorization = append(s.authorization, r.Header.Get("Authorization"))
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(&corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "proj", Name: "settings"},
	})
}

func (s *apiServer) requests() ([]string, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.impersonated...), append([]string(nil), s.authorization...)
}

func newClients(t *testing.T, server *httptest.Server) (*Clients, *rest.Config) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)

	config := &rest.Config{Host: server.URL, BearerToken: "controller-token"}
	return NewClients(config, scheme, mapper), config
}

// A client of For acts as the user on every request, with the credentials of the controller, while
// the config it was made from, and every client made from that, stay the controller.
func TestClientsActAsTheUser(t *testing.T) {
	recorder := new(apiServer)
	server := httptest.NewServer(recorder)
	t.Cleanup(server.Close)
	clients, config := newClients(t, server)
	ctx := context.Background()
	key := client.ObjectKey{Namespace: "proj", Name: "settings"}

	impersonating, err := clients.For("system:multitenancy-manager:project:proj")
	require.NoError(t, err)
	require.NoError(t, impersonating.Get(ctx, key, new(corev1.ConfigMap)))

	assert.Empty(t, config.Impersonate, "the config of the controller does not change")
	controller, err := client.New(config, client.Options{Scheme: clients.scheme, Mapper: clients.mapper})
	require.NoError(t, err)
	require.NoError(t, controller.Get(ctx, key, new(corev1.ConfigMap)))

	other, err := clients.For("system:multitenancy-manager:project:other")
	require.NoError(t, err)
	require.NoError(t, other.Get(ctx, key, new(corev1.ConfigMap)))

	impersonated, authorization := recorder.requests()
	assert.Equal(t, []string{
		"system:multitenancy-manager:project:proj",
		"",
		"system:multitenancy-manager:project:other",
	}, impersonated, "each client acts as its own user, and the client of the controller as nobody else")
	for _, header := range authorization {
		assert.Equal(t, "Bearer controller-token", header, "the controller authenticates every request")
	}
}

// A client without a user to act as would be the controller; For refuses to make one.
func TestClientsNeedAUser(t *testing.T) {
	server := httptest.NewServer(new(apiServer))
	t.Cleanup(server.Close)
	clients, _ := newClients(t, server)

	_, err := clients.For("")
	require.ErrorIs(t, err, errNoUser)
}
