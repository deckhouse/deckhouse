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

package capi

import (
	"testing"

	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/node-controller/internal/register"
)

func prefixReconciler(t *testing.T, objs ...runtime.Object) *MachineDeploymentReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	builder := fake.NewClientBuilder().WithScheme(scheme)
	for _, o := range objs {
		builder = builder.WithRuntimeObjects(o)
	}
	return &MachineDeploymentReconciler{BaseWithReader: BaseWithReader{Base: register.Base{Client: builder.Build()}}}
}

func clusterConfigSecret(data string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: clusterConfigSecretName, Namespace: clusterConfigSecretNamespace},
		Data:       map[string][]byte{"cluster-configuration.yaml": []byte(data)},
	}
}

// The instance prefix is part of every MachineDeployment name, and the name set it produces
// drives the stale-object prune. A read that fails open would make the prune consider every
// real "<prefix>-<ng>-<hash>" MachineDeployment stale and delete it, so an unreadable
// configuration must surface as an error instead of an empty prefix.
//
// Uses the fake client deliberately: the envtest suite creates the cluster-configuration Secret
// once in BeforeSuite for every spec, so "the Secret is absent" cannot be reached there without
// breaking the shared fixture. Only the reader's contract is asserted, never which calls it made.
func TestReadInstancePrefix_FailsClosed(t *testing.T) {
	t.Run("secret missing", func(t *testing.T) {
		r := prefixReconciler(t)
		got, err := r.readInstancePrefix(t.Context())
		require.ErrorContains(t, err, "get cluster-configuration secret", "prefix was %q", got)
	})

	t.Run("key missing", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: clusterConfigSecretName, Namespace: clusterConfigSecretNamespace},
			Data:       map[string][]byte{"other.yaml": []byte("{}")},
		}
		r := prefixReconciler(t, secret)
		got, err := r.readInstancePrefix(t.Context())
		require.ErrorContains(t, err, "no cluster-configuration.yaml key", "prefix was %q", got)
	})
}

func TestReadInstancePrefix_ReadsPrefix(t *testing.T) {
	r := prefixReconciler(t, clusterConfigSecret("cloud:\n  prefix: myprefix\n"))
	got, err := r.readInstancePrefix(t.Context())
	if err != nil {
		t.Fatalf("readInstancePrefix: %v", err)
	}
	if got != "myprefix" {
		t.Fatalf("prefix = %q, want myprefix", got)
	}
}

// A configuration that parsed and simply carries no cloud.prefix is legitimate — only read
// failures are errors.
func TestReadInstancePrefix_EmptyPrefixIsValid(t *testing.T) {
	r := prefixReconciler(t, clusterConfigSecret("clusterType: Static\n"))
	got, err := r.readInstancePrefix(t.Context())
	if err != nil {
		t.Fatalf("readInstancePrefix: %v", err)
	}
	if got != "" {
		t.Fatalf("prefix = %q, want empty", got)
	}
}
