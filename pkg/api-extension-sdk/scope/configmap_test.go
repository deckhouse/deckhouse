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

package scope

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/deckhouse/deckhouse/pkg/api-extension-sdk/internal/testcerts"
)

func authConfigMap(ca *testcerts.CA, allowed, usernames string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: AuthenticationConfigMapNamespace, Name: AuthenticationConfigMapName},
		Data: map[string]string{
			"client-ca-file":                     "ignored",
			keyRequestHeaderCA:                   string(ca.PEM()),
			keyRequestHeaderAllowedNames:         allowed,
			keyRequestHeaderUsernameHeaders:      usernames,
			"requestheader-group-headers":        `["X-Remote-Group"]`,
			"requestheader-extra-headers-prefix": `["X-Remote-Extra-"]`,
		},
	}
}

func TestConfigMapTrustSource(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ca1, ca2 := testcerts.NewCA(t, "ca1"), testcerts.NewCA(t, "ca2")
	cert1, cert2 := ca1.ClientCert(t, proxyName), ca2.ClientCert(t, proxyName)

	client := fake.NewClientset()
	cms := client.CoreV1().ConfigMaps(AuthenticationConfigMapNamespace)
	src := NewConfigMapTrustSource(client, 0)
	go src.Run(ctx)
	if err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 5*time.Second, true, func(context.Context) (bool, error) {
		return src.HasSynced(), nil
	}); err != nil {
		t.Fatal("not synced")
	}

	var got seen
	handler := NewTrustedHandler(recorder(&got), src)
	h := hdr("X-Remote-User", "alice", HeaderMode, "filtered", HeaderNamespaces, "a")
	trusted := func(cert *testcerts.Leaf) bool {
		got = seen{}
		handler.ServeHTTP(httptest.NewRecorder(), req{cert, h}.build())
		return got.hasDecision
	}
	eventually := func(what string, cond func() bool) {
		t.Helper()
		if err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 5*time.Second, true, func(context.Context) (bool, error) {
			return cond(), nil
		}); err != nil {
			t.Fatalf("timed out waiting for: %s", what)
		}
	}

	if trusted(cert1) {
		t.Fatal("trusted before the ConfigMap exists")
	}

	if _, err := cms.Create(ctx, authConfigMap(ca1, `["front-proxy-client"]`, `["X-Remote-User"]`), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	eventually("ca1 trusted", func() bool { return trusted(cert1) && !trusted(cert2) })

	if _, err := cms.Update(ctx, authConfigMap(ca2, `["front-proxy-client"]`, `["X-Remote-User"]`), metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	eventually("rotated to ca2", func() bool { return trusted(cert2) && !trusted(cert1) })

	if _, err := cms.Update(ctx, authConfigMap(ca2, `[]`, `["X-Remote-User"]`), metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	eventually("empty allowed names trust nothing", func() bool { return !trusted(cert2) })

	if _, err := cms.Update(ctx, authConfigMap(ca2, `["front-proxy-client"]`, `["X-Remote-User"]`), metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	eventually("ca2 trusted again", func() bool { return trusted(cert2) })

	if _, err := cms.Update(ctx, authConfigMap(ca2, `not json`, `["X-Remote-User"]`), metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	eventually("malformed ConfigMap trusts nothing", func() bool { return !trusted(cert2) })

	if _, err := cms.Update(ctx, authConfigMap(ca2, `["front-proxy-client"]`, `["X-Remote-User"]`), metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	eventually("ca2 trusted again", func() bool { return trusted(cert2) })

	if err := cms.Delete(ctx, AuthenticationConfigMapName, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	eventually("deleted ConfigMap trusts nothing", func() bool { return !trusted(cert2) })
}

func TestParseAuthenticationConfigMap(t *testing.T) {
	ca := testcerts.NewCA(t, "ca")
	cfg, err := ParseAuthenticationConfigMap(authConfigMap(ca, `["front-proxy-client","other"]`, `["X-Remote-User"]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.AllowedNames) != 2 || cfg.UsernameHeaders[0] != "X-Remote-User" || cfg.Roots == nil {
		t.Fatalf("%+v", cfg)
	}
	bad := authConfigMap(ca, `[]`, `[]`)
	bad.Data[keyRequestHeaderCA] = "garbage"
	if _, err := ParseAuthenticationConfigMap(bad); err == nil {
		t.Fatal("garbage CA accepted")
	}
	delete(bad.Data, keyRequestHeaderCA)
	if _, err := ParseAuthenticationConfigMap(bad); err == nil {
		t.Fatal("missing CA accepted")
	}
}
