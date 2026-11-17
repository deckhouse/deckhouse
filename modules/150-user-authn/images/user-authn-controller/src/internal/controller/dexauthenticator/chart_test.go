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
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	"user-authn-controller/internal/adoption"
)

// chartVariants are the module settings the files of testdata/chart were rendered under, and the
// kinds each file holds. A kind whose objects do not depend on what tells a variant from another
// one is kept in one file: the Service depends on no setting, the Dex client on the sessions of
// Dex only, the HTTPRoute on the Gateway API only.
var chartVariants = map[string]struct {
	settings   Settings
	gatewayAPI bool
	kinds      []string
}{
	"certmanager": {
		settings:   Settings{ClusterDomain: "cluster.local", HTTPSMode: "CertManager", IngressEnabled: true, IngressNginxHSTSAnnotation: true},
		gatewayAPI: true,
		kinds:      []string{"OAuth2Client", "Service", "Ingress", "HTTPRoute"},
	},
	"custom-certificate-sessions": {
		settings: Settings{ClusterDomain: "cluster.local", HTTPSMode: "CustomCertificate", IngressEnabled: true, DexSessions: true, PlatformSSO: true},
		kinds:    []string{"OAuth2Client", "Ingress", "HTTPRoute"},
	},
	"only-in-uri-sessions-no-sso": {
		settings: Settings{ClusterDomain: "cluster.local", HTTPSMode: "OnlyInURI", IngressEnabled: true, IngressNginxHSTSAnnotation: true, DexSessions: true},
		kinds:    []string{"OAuth2Client", "Ingress", "HTTPRoute"},
	},
	"disabled-old-annotations": {
		settings:   Settings{ClusterDomain: "cluster.local", HTTPSMode: "Disabled", IngressEnabled: true},
		gatewayAPI: true,
		kinds:      []string{"Ingress"},
	},
}

// readDocuments reads the YAML documents of a file of testdata/chart.
func readDocuments(t *testing.T, name string) []map[string]any {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("testdata", "chart", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var documents []map[string]any
	for _, part := range strings.Split("\n"+string(raw), "\n---") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		var document map[string]any
		if err := yaml.Unmarshal([]byte(part), &document); err != nil {
			t.Fatalf("parse a document of %s: %v", name, err)
		}
		if len(document) > 0 {
			documents = append(documents, document)
		}
	}
	return documents
}

// chartAuthenticators decodes the DexAuthenticator objects of testdata/chart.
func chartAuthenticators(t *testing.T) []Authenticator {
	t.Helper()

	var authenticators []Authenticator
	for _, document := range readDocuments(t, "authenticators.yaml") {
		a, err := Decode(&unstructured.Unstructured{Object: document})
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		authenticators = append(authenticators, a)
	}
	if len(authenticators) != 7 {
		t.Fatalf("%d objects in authenticators.yaml, want 7", len(authenticators))
	}
	return authenticators
}

// asDocument turns an object of the controller into the form the documents of the chart are in.
func asDocument(t *testing.T, obj any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return document
}

func documentKey(document map[string]any) string {
	metadata, _ := document["metadata"].(map[string]any)
	namespace, _ := metadata["namespace"].(string)
	name, _ := metadata["name"].(string)
	kind, _ := document["kind"].(string)
	return kind + " " + namespace + "/" + name
}

// comparable brings a document to what the chart and the controller must agree on. The label
// and the annotation the controller marks its objects with are its own; a secret is not compared;
// a list of peers the chart rendered empty is no list.
func comparable(document map[string]any) map[string]any {
	out := make(map[string]any, len(document))
	for key, value := range document {
		out[key] = value
	}
	delete(out, "secret")
	delete(out, "status")
	if peers, _ := out["ssoSharedWith"].([]any); len(peers) == 0 {
		delete(out, "ssoSharedWith")
	}

	metadata := map[string]any{}
	if source, ok := document["metadata"].(map[string]any); ok {
		for _, key := range []string{"name", "namespace", "labels", "annotations"} {
			values, isMap := source[key].(map[string]any)
			if !isMap {
				if value, has := source[key]; has {
					metadata[key] = value
				}
				continue
			}
			kept := make(map[string]any, len(values))
			for k, v := range values {
				if k != adoption.ManagedByLabel && k != adoption.ManagedMetadataAnnotation {
					kept[k] = v
				}
			}
			if len(kept) > 0 {
				metadata[key] = kept
			}
		}
	}
	out["metadata"] = metadata
	return out
}

// TestObjectsAreTheOnesOfTheChart compares the Dex client, the Ingress objects and the HTTPRoute
// objects the controller makes with what the module chart rendered, and the Service by all but
// what it points at: the chart selected the pods of the object, the controller names the Service
// of the instance.
func TestObjectsAreTheOnesOfTheChart(t *testing.T) {
	t.Parallel()

	authenticators := chartAuthenticators(t)
	keys := make([]types.NamespacedName, 0, len(authenticators))
	for _, a := range authenticators {
		keys = append(keys, a.Key())
	}

	for name, variant := range chartVariants {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			settings := variant.settings
			settings.GatewayAPI = variant.gatewayAPI
			in := testInput(authenticators...)
			in.settings = settings
			tables := build(t, in)

			got := map[string]map[string]any{}
			for _, a := range authenticators {
				entry := tables.entries[a.Key()]
				if entry.Record == nil || entry.Reason != "" {
					t.Fatalf("%s: %+v", a.Key(), entry)
				}

				client := desiredClient(a, *entry.Record, settings, ssoPeers(a, keys, settings))
				got[documentKey(client.Object)] = client.Object

				service, _ := desiredService(a, settings)
				document := asDocument(t, service)
				document["kind"] = "Service"
				got[documentKey(document)] = document

				for _, route := range desiredIngresses(a, entry.Record, nil, settings) {
					document := asDocument(t, route.ingress)
					document["apiVersion"], document["kind"] = "networking.k8s.io/v1", "Ingress"
					got[documentKey(document)] = document
				}
				for _, route := range desiredHTTPRoutes(a, entry.Record, settings) {
					got[documentKey(route.object.Object)] = asDocument(t, route.object.Object)
				}
			}

			want := readDocuments(t, name+".yaml")
			var wantKeys []string
			for _, document := range want {
				key := documentKey(document)
				wantKeys = append(wantKeys, key)
				if kind, _ := document["kind"].(string); !slices.Contains(variant.kinds, kind) {
					t.Fatalf("%s.yaml holds a %s, which is not among the kinds of the variant", name, kind)
				}

				mine, ok := got[key]
				if !ok {
					t.Errorf("the controller makes no %s", key)
					continue
				}
				if document["kind"] == "Service" {
					compareService(t, key, document, mine)
					continue
				}
				if a, b := comparable(document), comparable(mine); !reflect.DeepEqual(a, b) {
					rawA, _ := json.MarshalIndent(a, "", "  ")
					rawB, _ := json.MarshalIndent(b, "", "  ")
					t.Errorf("%s differs.\nThe chart:\n%s\nThe controller:\n%s", key, rawA, rawB)
				}
			}
			for key, document := range got {
				kind, _ := document["kind"].(string)
				if slices.Contains(variant.kinds, kind) && !slices.Contains(wantKeys, key) {
					t.Errorf("the controller makes %s, the chart did not", key)
				}
			}
		})
	}
}

// compareService compares the metadata and the port of the Service of an object.
func compareService(t *testing.T, key string, chart, mine map[string]any) {
	t.Helper()

	if a, b := comparable(chart)["metadata"], comparable(mine)["metadata"]; !reflect.DeepEqual(a, b) {
		t.Errorf("%s: metadata %v, the chart had %v", key, b, a)
	}

	port := func(document map[string]any) (string, float64) {
		spec, _ := document["spec"].(map[string]any)
		ports, _ := spec["ports"].([]any)
		if len(ports) != 1 {
			t.Fatalf("%s: %d ports", key, len(ports))
		}
		first, _ := ports[0].(map[string]any)
		name, _ := first["name"].(string)
		number, _ := first["port"].(float64)
		return name, number
	}
	chartName, chartPort := port(chart)
	name, number := port(mine)
	if name != chartName || number != chartPort {
		t.Errorf("%s: port %s %v, the chart had %s %v", key, name, number, chartName, chartPort)
	}

	spec, _ := mine["spec"].(map[string]any)
	namespace, _ := mine["metadata"].(map[string]any)["namespace"].(string)
	wantTarget := "dex-authenticator.d8-user-authn.svc.cluster.local"
	if strings.HasPrefix(namespace, "d8-") {
		wantTarget = "dex-authenticator-system.d8-user-authn.svc.cluster.local"
	}
	if spec["type"] != "ExternalName" || spec["externalName"] != wantTarget {
		t.Errorf("%s: type %v, externalName %v; want ExternalName %s", key, spec["type"], spec["externalName"], wantTarget)
	}
	if _, has := spec["selector"]; has {
		t.Errorf("%s has a selector", key)
	}
}
