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
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	genericapiserver "k8s.io/apiserver/pkg/server"
	utilcompatibility "k8s.io/apiserver/pkg/util/compatibility"
	restclient "k8s.io/client-go/rest"
	certutil "k8s.io/client-go/util/cert"

	templatesv1alpha1 "github.com/deckhouse/node-controller/api/templates.internal.deckhouse.io/v1alpha1"
)

// stubStorage is the minimal read-only storage the installer accepts.
type stubStorage struct{}

func (stubStorage) New() runtime.Object { return &templatesv1alpha1.NodeConfigTemplate{} }

func (stubStorage) Destroy() {}

func (stubStorage) NamespaceScoped() bool { return false }

func (stubStorage) GetSingularName() string { return "nodeconfigtemplate" }

func (stubStorage) Get(_ context.Context, name string, _ *metav1.GetOptions) (runtime.Object, error) {
	return nil, apierrors.NewNotFound(schema.GroupResource{
		Group:    templatesv1alpha1.GroupVersion.Group,
		Resource: templatesv1alpha1.NodeConfigTemplateResource,
	}, name)
}

// newPreparedServer builds the aggregated server the way Run does, minus the
// delegated authentication, and takes the startup step that killed the process:
// with an OpenAPI config set, PrepareRun builds the spec for every route the
// generic server carries and calls klog.Fatal on a model it has no definition
// for. Nothing before that line notices, so a server that cannot start looks
// healthy to a test that only calls newServer.
func newPreparedServer(t *testing.T) *genericapiserver.GenericAPIServer {
	t.Helper()

	cfg := genericapiserver.NewRecommendedConfig(Codecs)
	cfg.EffectiveVersion = utilcompatibility.DefaultBuildEffectiveVersion()
	cfg.ExternalAddress = "127.0.0.1:443"
	cfg.LoopbackClientConfig = &restclient.Config{Host: "127.0.0.1"}

	srv, err := newServer(cfg, stubStorage{})
	require.NoError(t, err)
	require.NotNil(t, srv.PrepareRun())
	return srv
}

func TestServesInternalDeckhouseGroup(t *testing.T) {
	srv := newPreparedServer(t)

	ts := httptest.NewServer(srv.Handler)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/apis/templates.internal.deckhouse.io/v1alpha1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var list metav1.APIResourceList
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&list))
	require.Equal(t, "templates.internal.deckhouse.io/v1alpha1", list.GroupVersion)

	names := make([]string, 0, len(list.APIResources))
	for _, r := range list.APIResources {
		names = append(names, r.Name)
	}
	require.Contains(t, names, templatesv1alpha1.NodeConfigTemplateResource)
}

// The group's OpenAPI v3 document is built from the definitions this package
// hands the generic server. A model without one leaves the document empty: the
// group serves "null" and the server logs the missing models on every start.
func TestServesAnOpenAPIV3DocumentForTheGroup(t *testing.T) {
	srv := newPreparedServer(t)

	ts := httptest.NewServer(srv.Handler)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/openapi/v3/apis/templates.internal.deckhouse.io/v1alpha1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NotEqual(t, "null", strings.TrimSpace(string(body)), "the group serves no schema at all")
	require.Contains(t, string(body), templatesv1alpha1.NodeConfigTemplateResource)
}

// The delegated-authentication lookup dials the kube-apiserver once, and the pod
// starts before it answers — including the bootstrap window in which the client
// points at 127.0.0.1:6445. Returning there takes the whole module down:
// cmd/main.go exits the process, and the CRD conversion webhook of NodeGroup and
// Instance is served by this very pod.
func TestNewConfigRetriesTheStartupLookup(t *testing.T) {
	// In-cluster config with no service account token on disk: every attempt
	// fails, the way it does while the kube-apiserver refuses connections.
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "1")

	interval := configRetryInterval
	configRetryInterval = 20 * time.Millisecond
	t.Cleanup(func() { configRetryInterval = interval })

	certFile, keyFile := writeSelfSignedCert(t)
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	started := time.Now()
	_, err := newConfig(ctx, Options{BindPort: freePort(t), CertFile: certFile, KeyFile: keyFile})

	require.Error(t, err)
	require.GreaterOrEqual(t, time.Since(started), 400*time.Millisecond,
		"newConfig gave up on the first failed lookup instead of waiting for the kube-apiserver")
	// Every attempt must fail on the lookup: a listener left behind by the
	// previous one would turn this into "address already in use" forever.
	require.Contains(t, err.Error(), "apply authentication options")
}

// The signal readiness reads must mean "answering", not "started": newConfig
// opens the listener on its first attempt and keeps it across every retry, so a
// port that is bound proves nothing while the startup lookup is still failing.
func TestServingIsNotSignalledWhileTheStartupLookupRetries(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "1")

	interval := configRetryInterval
	configRetryInterval = 20 * time.Millisecond
	t.Cleanup(func() { configRetryInterval = interval })

	certFile, keyFile := writeSelfSignedCert(t)
	port := freePort(t)
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	serving := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{
			BindPort: port,
			CertFile: certFile,
			KeyFile:  keyFile,
			Storage:  stubStorage{},
			Serving:  serving,
		})
	}()

	require.Error(t, <-done)
	select {
	case <-serving:
		t.Fatal("the aggregated API server reported itself answering while it never started serving")
	default:
	}
}

func writeSelfSignedCert(t *testing.T) (string, string) {
	t.Helper()

	cert, key, err := certutil.GenerateSelfSignedCertKey("localhost", []net.IP{net.ParseIP("127.0.0.1")}, nil)
	require.NoError(t, err)

	dir := t.TempDir()
	certFile := filepath.Join(dir, "tls.crt")
	keyFile := filepath.Join(dir, "tls.key")
	require.NoError(t, os.WriteFile(certFile, cert, 0o600))
	require.NoError(t, os.WriteFile(keyFile, key, 0o600))
	return certFile, keyFile
}

func freePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	number, err := strconv.Atoi(port)
	require.NoError(t, err)
	return number
}
