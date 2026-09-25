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

package validating_test

// The hook reads two shapes of the same resource. v2alpha1 lists every application in
// spec.applications; v1 and v1alpha1 describe the first one in spec itself and the rest in
// spec.additionalApplications, with different field names for the Gateway API reference. Both
// shapes are written out here rather than produced by running the conversion hook, so that a
// broken conversion cannot make these tests fail for a reason that has nothing to do with them.

// gatewayAPIFieldNames maps an application-level Gateway API field to its name in spec.gatewayAPI.
var gatewayAPIFieldNames = map[string]string{
	"httpRouteListenerSetName":        "applicationHTTPRouteListenerSetName",
	"httpRouteListenerSetNamespace":   "applicationHTTPRouteListenerSetNamespace",
	"httpRouteListenerSetSectionName": "applicationHTTPRouteListenerSetSectionName",
}

// listenerSetReference builds the Gateway API reference of one application. An empty namespace or
// section means the field is left out, which is how a user says "the default".
func listenerSetReference(name, namespace, section string) map[string]any {
	reference := map[string]any{"httpRouteListenerSetName": name}
	if namespace != "" {
		reference["httpRouteListenerSetNamespace"] = namespace
	}
	if section != "" {
		reference["httpRouteListenerSetSectionName"] = section
	}

	return reference
}

func authenticator(version, namespace, name string, apps ...any) map[string]any {
	object := map[string]any{
		"apiVersion": "deckhouse.io/" + version,
		"kind":       "DexAuthenticator",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
	}

	if version == "v2alpha1" {
		object["spec"] = map[string]any{"applications": append([]any{}, apps...)}

		return object
	}

	spec := map[string]any{}
	if len(apps) > 0 {
		first := apps[0].(map[string]any)
		spec["applicationDomain"] = first["domain"]
		if class, ok := first["ingressClassName"]; ok {
			spec["applicationIngressClassName"] = class
		}
		if reference, ok := first["gatewayAPI"].(map[string]any); ok {
			gateway := map[string]any{}
			for field, value := range reference {
				gateway[gatewayAPIFieldNames[field]] = value
			}
			spec["gatewayAPI"] = gateway
		}
	}
	if len(apps) > 1 {
		spec["additionalApplications"] = append([]any{}, apps[1:]...)
	}
	object["spec"] = spec

	return object
}

// withApplication puts the application under test either first or after an unrelated one, so that
// both positions of the two shapes are exercised. The filler carries the namespace and the name in
// its domain: two fixtures must not collide on the filler and hide what the test is really about.
func withApplication(version, namespace, name string, app map[string]any, additional bool) map[string]any {
	apps := []any{app}
	if additional {
		filler := map[string]any{
			"domain":           "unrelated-" + namespace + "-" + name + ".example.com",
			"ingressClassName": "nginx",
		}
		apps = []any{filler, app}
	}

	return authenticator(version, namespace, name, apps...)
}
