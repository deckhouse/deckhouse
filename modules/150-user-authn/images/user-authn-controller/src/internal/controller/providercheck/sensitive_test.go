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

package providercheck

import (
	"context"
	"net/http"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
)

func TestReconcile_LDAPBindFromMaskedCache(t *testing.T) {
	t.Parallel()

	const secret = "dh-test-secret-0123456789abcdef"
	const bindDN = "cn=admin,dc=example,dc=com"
	provider := ldapProvider("ldap", 1, map[string]any{
		"host": "ldap.example.com", "insecureNoSSL": true,
		"bindDN": bindDN, "bindPW": controller.MaskedValue,
	})
	unmasked := provider.DeepCopy()
	if err := unstructured.SetNestedField(unmasked.Object, secret, "spec", "ldap", "bindPW"); err != nil {
		t.Fatal(err)
	}
	httpFake := newFakeHTTP()
	httpFake.set(dexDiscoveryURL, http.StatusOK, `{"issuer":"https://dex.d8-user-authn"}`)
	ldap := &recordingLDAP{}
	r := newTestReconciler(t, time.Now(), httpFake, ldap, provider, newCheck("ldap", "ldap"))
	r.apiReader = fake.NewClientBuilder().WithScheme(r.client.Scheme()).WithObjects(unmasked).Build()

	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "ldap"}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if ldap.bindCalls != 1 || ldap.bindDN != bindDN || ldap.bindPW != secret {
		t.Errorf("LDAP bind = (%q, %q), calls = %d; want the unmasked API credentials once",
			ldap.bindDN, ldap.bindPW, ldap.bindCalls)
	}
	assertStep(t, getCheck(t, r, "ldap"), "ldapBind", stepSucceeded)
}

type recordingLDAP struct {
	fakeLDAPConn
	bindDN    string
	bindPW    string
	bindCalls int
}

func (c *recordingLDAP) Dial(context.Context, *DexProviderLDAPForCheck) (LDAPConn, error) {
	return c, nil
}

func (c *recordingLDAP) Bind(dn, password string) error {
	c.bindCalls++
	c.bindDN, c.bindPW = dn, password
	return nil
}
