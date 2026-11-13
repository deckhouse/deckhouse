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

package v1alpha1_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"fencing-controller/internal/testenv"
)

const fencingSLAProfileCRD = "crds/fencingslaprofiles.yaml"

// TestFencingSLAProfileCRDRules runs the rules and the schema of the CRD of
// FencingSLAProfile in a real API server. The built-in profiles of the module
// templates are the fixtures: a test changes one value and expects the CRD to
// refuse it.
func TestFencingSLAProfileCRDRules(t *testing.T) {
	env := testenv.Start(t)
	c := env.Client

	created := 0

	// builtinProfile reads the built-in profile from the module templates. It
	// gives each object its own name, because the API server keeps the
	// objects of the earlier cases.
	builtinProfile := func(t *testing.T, name string) *unstructured.Unstructured {
		t.Helper()

		profile := testenv.Manifests(t, "templates/fencing-agent/sla-profiles/"+name+".yaml")[0]
		if profile.GetKind() != "FencingSLAProfile" || profile.GetName() != name {
			t.Fatalf("the built-in profile %s parsed as %s %q", name, profile.GetKind(), profile.GetName())
		}

		created++
		profile.SetName(fmt.Sprintf("%s-%d", name, created))
		profile.SetAnnotations(nil)

		return profile
	}

	for _, name := range []string{"critical", "normal", "slow"} {
		t.Run("built-in "+name+" is accepted", func(t *testing.T) {
			profile := builtinProfile(t, name)
			checkRefusal(t, c.Create(t.Context(), profile), "")
			t.Cleanup(func() { _ = c.Delete(t.Context(), profile) })
		})
	}

	tests := []struct {
		name    string
		path    []string
		value   any
		wantErr string
	}{
		{"fractional watchdog timeout", []string{"watchdog", "timeout"}, "2.5s", "whole number of seconds"},
		{"feed interval above a quarter of the watchdog timeout", []string{"watchdog", "feedInterval"}, "1.5s", "at most a quarter"},
		{"watchdog margin below 300ms", []string{"watchdog", "margin"}, "200ms", "watchdog.margin must be at least 300ms"},
		{"probe timeout above half of the probe interval", []string{"memberlist", "probeTimeout"}, "101ms", "memberlist.probeTimeout must be at most half"},
		{"suspicion multiplier below 3", []string{"memberlist", "suspicionMult"}, int64(2), "memberlist.suspicionMult must be at least 3"},
		{"heartbeat and API timeout above the window", []string{"heartbeat", "apiTimeout"}, "3.2s", "heartbeat.interval + heartbeat.apiTimeout"},
		{"heartbeat above a third of the window", []string{"heartbeat", "interval"}, "1.4s", "heartbeat.interval must be at most a third"},
		{"duration without a unit", []string{"controlPlaneLostAfter"}, "3", "should match"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			profile := builtinProfile(t, "critical")
			if err := unstructured.SetNestedField(profile.Object, tc.value, append([]string{"spec"}, tc.path...)...); err != nil {
				t.Fatalf("set %v: %v", tc.path, err)
			}

			err := c.Create(t.Context(), profile)
			if err == nil {
				t.Cleanup(func() { _ = c.Delete(t.Context(), profile) })
			}

			checkRefusal(t, err, tc.wantErr)
		})
	}

	t.Run("the schema has none of the removed fields and none of the correlation, brake and limit fields", func(t *testing.T) {
		spec := shippedCRDSchema(t, fencingSLAProfileCRD)["properties"].(map[string]any)["spec"].(map[string]any)
		for _, removed := range []string{"reactionGoal", "detectionWindowTarget", "fallback", "evacuation"} {
			if _, found := spec["properties"].(map[string]any)[removed]; found {
				t.Errorf("spec.%s is still in the schema", removed)
			}
		}

		raw := strings.ToLower(strings.Join(propertyNames(spec), " "))
		for _, forbidden := range []string{"minfailures", "window", "corrwindow", "massfailurethreshold", "maxconcurrentevictions", "ttl"} {
			if strings.Contains(raw, forbidden) {
				t.Errorf("the schema has a field with %q in its name", forbidden)
			}
		}
	})
}

// checkRefusal checks that the API server accepted the write when wantErr is
// empty, and refused it with wantErr in the message otherwise.
func checkRefusal(t *testing.T, err error, wantErr string) {
	t.Helper()

	if wantErr == "" {
		if err != nil {
			t.Fatalf("want accepted, got: %v", err)
		}

		return
	}

	if err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("want refused with %q, got: %v", wantErr, err)
	}
}

// shippedCRDSchema reads the openAPIV3Schema of the one version that a CRD of
// the module serves. The path is relative to the directory of the module.
func shippedCRDSchema(t *testing.T, path string) map[string]any {
	t.Helper()

	raw, err := os.ReadFile(testenv.ModuleFile(path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var crd struct {
		Spec struct {
			Versions []struct {
				Schema struct {
					OpenAPIV3Schema map[string]any `json:"openAPIV3Schema"`
				} `json:"schema"`
			} `json:"versions"`
		} `json:"spec"`
	}

	if err := yaml.Unmarshal(raw, &crd); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	if len(crd.Spec.Versions) != 1 {
		t.Fatalf("%s serves %d versions, want 1", path, len(crd.Spec.Versions))
	}

	return crd.Spec.Versions[0].Schema.OpenAPIV3Schema
}

// propertyNames lists the names of all properties of a schema, at any depth.
func propertyNames(schema map[string]any) []string {
	var names []string

	properties, _ := schema["properties"].(map[string]any)
	for name, property := range properties {
		names = append(names, name)
		if child, ok := property.(map[string]any); ok {
			names = append(names, propertyNames(child)...)
		}
	}

	return names
}
