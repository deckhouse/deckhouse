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

package webhook

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

type fakeRegistrar struct {
	called  bool
	manager ctrl.Manager
	err     error
}

func (f *fakeRegistrar) Register(manager ctrl.Manager) error {
	f.called = true
	f.manager = manager
	return f.err
}

func TestServerRegisterDelegatesToRegistrar(t *testing.T) {
	t.Parallel()

	server, err := newTestServer(t)
	if err != nil {
		t.Fatalf("newTestServer() error = %v", err)
	}

	registrar := &fakeRegistrar{}
	if err := server.Register(registrar); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if !registrar.called || registrar.manager == nil {
		t.Fatal("Register() did not call registrar")
	}
}

func TestServerRegisterReturnsRegistrarError(t *testing.T) {
	t.Parallel()

	server, err := newTestServer(t)
	if err != nil {
		t.Fatalf("newTestServer() error = %v", err)
	}

	want := errors.New("register failed")
	registrar := &fakeRegistrar{err: want}
	if err := server.Register(registrar); !errors.Is(err, want) {
		t.Fatalf("Register() error = %v, want %v", err, want)
	}
}

func TestServerClient(t *testing.T) {
	t.Parallel()

	server, err := newTestServer(t)
	if err != nil {
		t.Fatalf("newTestServer() error = %v", err)
	}

	if server.Client() == nil {
		t.Fatal("Client() = nil, want client")
	}
}

func newTestServer(t *testing.T) (*Server, error) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	cfg := DefaultServerConfig()
	cfg.MetricsBindAddress = ":0"
	cfg.HealthProbeBindAddress = ":0"

	return NewServer(&rest.Config{Host: "https://127.0.0.1:443"}, scheme, cfg)
}
