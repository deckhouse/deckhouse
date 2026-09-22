// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package providerinitializer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/lib-connection/pkg/settings"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
)

// A --kubeconfig run must reach the API server as the same user the kubectl proxy
// on a master impersonates, or Deckhouse denies it what it allows over SSH - the
// ValidatingAdmissionPolicy on CAPI Cluster deletion, for one.
func TestAKubeconfigConnectionActsAsDhctl(t *testing.T) {
	apiserver := newFakeAPIServer(t, false)
	kubeconfig := writeKubeconfig(t, apiserver.URL)

	connectToAPIServer(t, kubeconfig)

	users, groups := apiserver.impersonated()
	require.NotEmpty(t, users)
	for i, user := range users {
		require.Equal(t, global.ImpersonateUser, user, "request %d must be made as %q", i, global.ImpersonateUser)
		// Impersonation replaces the identity whole, so the group comes along.
		require.Equal(t, global.ImpersonateGroup, groups[i], "request %d must be made in %q", i, global.ImpersonateGroup)
	}
}

// A kubeconfig whose user may not impersonate must keep working as that user: the
// operations it can do are the ones it could do before, and none of them regress
// into a blanket 403.
func TestAKubeconfigThatMayNotImpersonateFallsBackToItsOwnIdentity(t *testing.T) {
	apiserver := newFakeAPIServer(t, true)
	kubeconfig := writeKubeconfig(t, apiserver.URL)

	connectToAPIServer(t, kubeconfig)

	users, _ := apiserver.impersonated()
	require.Equal(t, global.ImpersonateUser, users[0], "impersonation is asked for once, and refused")
	require.NotEmpty(t, users[1:])
	for i, user := range users[1:] {
		require.Empty(t, user, "request %d must carry the kubeconfig's own identity", i+1)
	}
}

// connectToAPIServer drives the whole path a dhctl command takes with --kubeconfig:
// the resolved kube config, the client built from it and the first request it makes.
func connectToAPIServer(t *testing.T, kubeconfig string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	params := settings.ProviderParams{
		Logger: dhlog.FromContext(ctx),
		TmpDir: t.TempDir(),
	}

	sshProviderInitializer, kubeProvider, err := GetProviders(ctx, params,
		WithConnectionConfigOnly(),
		WithKubeConfig(kubeconfig, "", false),
	)
	require.NoError(t, err)
	require.Nil(t, sshProviderInitializer, "a kubeconfig connection needs no SSH provider")
	require.NotNil(t, kubeProvider)

	_, err = kubeProvider.Client(ctx)
	require.NoError(t, err)
	require.NoError(t, kubeProvider.Cleanup(ctx))
}

type fakeAPIServer struct {
	*httptest.Server

	refuseImpersonation bool

	mu     sync.Mutex
	users  []string
	groups []string
}

// newFakeAPIServer answers /version, which is all a client does on connect. With
// refuseImpersonation it answers an impersonated request the way an API server does
// when the caller holds no impersonate verb.
func newFakeAPIServer(t *testing.T, refuseImpersonation bool) *fakeAPIServer {
	t.Helper()

	apiserver := &fakeAPIServer{refuseImpersonation: refuseImpersonation}
	apiserver.Server = httptest.NewServer(apiserver)
	t.Cleanup(apiserver.Close)

	return apiserver
}

func (s *fakeAPIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user := r.Header.Get("Impersonate-User")

	s.mu.Lock()
	s.users = append(s.users, user)
	s.groups = append(s.groups, r.Header.Get("Impersonate-Group"))
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")

	if s.refuseImpersonation && user != "" {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(metav1.Status{
			TypeMeta: metav1.TypeMeta{Kind: "Status", APIVersion: "v1"},
			Status:   metav1.StatusFailure,
			Code:     http.StatusForbidden,
			Reason:   metav1.StatusReasonForbidden,
			Message: fmt.Sprintf(`users %q is forbidden: User "tester" cannot impersonate resource "users" `+
				`in API group "" at the cluster scope`, user),
		})
		return
	}

	_, _ = w.Write([]byte(`{"major":"1","minor":"30","gitVersion":"v1.30.0","platform":"linux/amd64"}`))
}

// impersonated returns the identity of every request the server has seen, in order.
func (s *fakeAPIServer) impersonated() ([]string, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.users, s.groups
}

func writeKubeconfig(t *testing.T, server string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "admin.kubeconfig")
	content := fmt.Sprintf(`apiVersion: v1
kind: Config
current-context: ctx
clusters:
- name: cluster
  cluster:
    server: %s
contexts:
- name: ctx
  context:
    cluster: cluster
    user: tester
users:
- name: tester
  user:
    token: token
`, server)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}
