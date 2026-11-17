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

package dexauthenticator

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

func TestDecode(t *testing.T) {
	t.Parallel()

	obj := authenticatorObject("d8-monitoring", "grafana", 0, "grafana.example.com")
	obj.SetLabels(map[string]string{"heritage": "deckhouse"})
	obj.SetAnnotations(map[string]string{controller.DexAuthenticatorAllowAccessToKubernetesAnnotation: "true"})
	obj.Object["spec"] = map[string]any{
		"applicationDomain":                       "grafana.example.com",
		"applicationIngressClassName":             "nginx",
		"applicationIngressCertificateSecretName": "ingress-tls",
		"signOutURL":                              "/logout",
		"whitelistSourceRanges":                   []any{"10.0.0.0/8"},
		"gatewayAPI": map[string]any{
			"applicationHTTPRouteListenerSetName":        "public",
			"applicationHTTPRouteListenerSetNamespace":   "d8-alb",
			"applicationHTTPRouteListenerSetSectionName": "https",
		},
		"additionalApplications": []any{
			map[string]any{
				"domain": "prometheus.example.com", "ingressClassName": "internal", "ingressSecretName": "other-tls",
				"signOutURL": "/out", "whitelistSourceRanges": []any{"192.168.0.0/16"},
				"gatewayAPI": map[string]any{"httpRouteListenerSetName": "shared"},
			},
			// The conversion from v2alpha1 leaves the keys of what an application does not set empty.
			map[string]any{"domain": "bare.example.com", "ingressSecretName": nil},
		},
		"sendAuthorizationHeader": true,
		"keepUsersLoggedInFor":    "24h",
		"allowedEmails":           []any{"john.doe@example.com"},
		"allowedGroups":           []any{"admins"},
		"allowedProviders":        []any{"local"},
		"highAvailability":        false,
	}

	got, err := Decode(obj)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	no := false
	want := Authenticator{
		Name: "grafana", Namespace: "d8-monitoring", UID: "uid-d8-monitoring-grafana", Created: testNow, Generation: 1,
		Module: true, AllowAccessToKubernetes: true,
		Applications: []Application{
			{
				Domain: "grafana.example.com", IngressClassName: "nginx", IngressSecretName: "ingress-tls", SignOutURL: "/logout",
				WhitelistSourceRanges: []string{"10.0.0.0/8"},
				ListenerSet:           ListenerSet{Name: "public", Namespace: "d8-alb", SectionName: "https"},
			},
			{
				Domain: "prometheus.example.com", IngressClassName: "internal", IngressSecretName: "other-tls", SignOutURL: "/out",
				WhitelistSourceRanges: []string{"192.168.0.0/16"},
				ListenerSet:           ListenerSet{Name: "shared"},
			},
			{Domain: "bare.example.com"},
		},
		SendAuthorizationHeader: true, KeepUsersLoggedInFor: "24h",
		AllowedEmails: []string{"john.doe@example.com"}, AllowedGroups: []string{"admins"}, AllowedProviders: []string{"local"},
		HighAvailability: &no,
	}
	if !got.Created.Equal(want.Created) {
		t.Errorf("created %v, want %v", got.Created, want.Created)
	}
	// The same moment may come back in another time zone.
	want.Created = got.Created
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Decode =\n%+v\nwant\n%+v", got, want)
	}
	if got.Instance() != naming.SystemAuthenticatorInstance || got.ClientID() != "dex-authenticator-grafana@d8-monitoring" {
		t.Errorf("instance %s, client %s", got.Instance(), got.ClientID())
	}
}

func TestDecode_DefaultsAndRefusals(t *testing.T) {
	t.Parallel()

	plain, err := Decode(authenticatorObject("shop", "app", 0, "app.example.com"))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if plain.KeepUsersLoggedInFor != "168h" || plain.Module || plain.AllowAccessToKubernetes || plain.HighAvailability != nil {
		t.Errorf("defaults: %+v", plain)
	}

	// A value of the annotation that is not a boolean gives no access to the Kubernetes API.
	obj := authenticatorObject("shop", "app", 0, "app.example.com")
	obj.SetAnnotations(map[string]string{controller.DexAuthenticatorAllowAccessToKubernetesAnnotation: "yes please"})
	if a, err := Decode(obj); err != nil || a.AllowAccessToKubernetes {
		t.Errorf("an annotation that is not a boolean: access %v, error %v", a.AllowAccessToKubernetes, err)
	}

	for name, spec := range map[string]map[string]any{
		"no domain":              {"applicationIngressClassName": "nginx"},
		"an address, not a name": {"applicationDomain": "https://app.example.com/"},
		"a name with a port":     {"applicationDomain": "app.example.com:8443"},
		"an application without a domain": {
			"applicationDomain":      "app.example.com",
			"additionalApplications": []any{map[string]any{"ingressClassName": "nginx"}},
		},
		"a lifetime that is not a duration": {"applicationDomain": "app.example.com", "keepUsersLoggedInFor": "a week"},
		"a spec of another shape":           {"applicationDomain": []any{"app.example.com"}},
	} {
		obj := authenticatorObject("shop", "app", 0, "app.example.com")
		obj.Object["spec"] = spec
		if _, err := Decode(obj); err == nil || !strings.Contains(err.Error(), "shop/app") {
			t.Errorf("%s: error %v, want one that names the object", name, err)
		}
	}
}

func TestParseSettings(t *testing.T) {
	t.Parallel()

	parse := func(data map[string]string) (Settings, error) {
		return ParseSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: SettingsConfigMapName}, Data: data})
	}

	got, err := parse(map[string]string{SettingsKey: `{"clusterDomain":"cluster.local","httpsMode":"CertManager","ingressEnabled":true,` +
		`"ingressNginxHSTSAnnotation":true,"dexSessions":true,"platformSSO":true,"idTokenTTL":"30m","addedLater":1}`})
	want := Settings{
		ClusterDomain: "cluster.local", HTTPSMode: "CertManager", IngressEnabled: true, IngressNginxHSTSAnnotation: true,
		DexSessions: true, PlatformSSO: true, IDTokenTTL: "30m",
	}
	if err != nil || got != want {
		t.Errorf("settings = %+v, %v; want %+v", got, err, want)
	}
	if !got.ingressTLS() || (Settings{HTTPSMode: "OnlyInURI"}).ingressTLS() || (Settings{HTTPSMode: "Disabled"}).ingressTLS() {
		t.Error("ingressTLS")
	}

	for name, data := range map[string]map[string]string{
		"no key":            {},
		"not JSON":          {SettingsKey: "clusterDomain: cluster.local"},
		"no cluster domain": {SettingsKey: `{"httpsMode":"CertManager"}`},
		"a bad lifetime":    {SettingsKey: `{"clusterDomain":"cluster.local","idTokenTTL":"soon"}`},
		"a zero lifetime":   {SettingsKey: `{"clusterDomain":"cluster.local","idTokenTTL":"0s"}`},
	} {
		if _, err := parse(data); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestState(t *testing.T) {
	t.Parallel()

	state := NewState()
	if _, known, ready := state.Entry(shopApp); known || ready {
		t.Error("a new state knows an object")
	}

	console, grafana := keyOf("d8-console", "console"), keyOf("d8-monitoring", "grafana")
	record := &Record{Name: "app", Namespace: "shop", Hosts: []string{"app.example.com"}}
	entries := func(keys ...types.NamespacedName) map[types.NamespacedName]Entry {
		out := map[types.NamespacedName]Entry{}
		for _, key := range keys {
			out[key] = Entry{UID: types.UID(key.String()), Instance: naming.AuthenticatorInstance(key.Namespace), Record: record, Generation: 1}
		}
		return out
	}

	// The first decisions are all new, even for a reconciler that has just started and changed nothing.
	if changed := state.set(entries(shopApp, console)); !slices.Equal(changed, []types.NamespacedName{console, shopApp}) {
		t.Errorf("first set: %v", changed)
	}
	if entry, known, ready := state.Entry(shopApp); !known || !ready || entry.Record != record {
		t.Errorf("entry = %+v, %v, %v", entry, known, ready)
	}
	if changed := state.set(entries(shopApp, console)); len(changed) != 0 {
		t.Errorf("the same decisions changed %v", changed)
	}

	// A new object of a d8-* namespace changes the Dex client of every other one.
	if changed := state.set(entries(shopApp, console, grafana)); !slices.Equal(changed, []types.NamespacedName{console, grafana}) {
		t.Errorf("a new platform object changed %v", changed)
	}
	if got := state.Platform(); !slices.Equal(got, []types.NamespacedName{console, grafana}) {
		t.Errorf("platform = %v", got)
	}

	// Another decision about one object changes it alone; an object that is gone is announced too.
	next := entries(console, grafana)
	next[shopApp] = Entry{UID: "shop/app", Instance: naming.UserAuthenticatorInstance, Reason: ReasonHostConflict}
	if changed := state.set(next); !slices.Equal(changed, []types.NamespacedName{shopApp}) {
		t.Errorf("another decision changed %v", changed)
	}
	if changed := state.set(entries(console)); !slices.Equal(changed, []types.NamespacedName{console, grafana, shopApp}) {
		t.Errorf("gone objects changed %v", changed)
	}
}
