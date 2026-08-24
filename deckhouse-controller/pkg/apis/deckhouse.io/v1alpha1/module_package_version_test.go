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

package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/validation"
)

func TestPackageRepositoryNameForModuleSource(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{source: "deckhouse", want: "deckhouse-modules"},
		{source: "example", want: "example"},
		{source: "deckhouse-prod", want: "deckhouse-prod"},
		{source: "", want: ""},
	}

	for _, c := range cases {
		if got := PackageRepositoryNameForModuleSource(c.source); got != c.want {
			t.Errorf("PackageRepositoryNameForModuleSource(%q) = %q, want %q", c.source, got, c.want)
		}
	}
}

func TestSanitizeVersionForName(t *testing.T) {
	cases := []struct {
		version string
		want    string
	}{
		{version: "v1.78.0", want: "v1.78.0"},
		{version: "v1.78.0-pr22189+8776a42", want: "v1.78.0-pr22189-8776a42"},
		{version: "v1.0.0-RC1", want: "v1.0.0-rc1"},
		{version: "dev", want: "dev"},
		{version: "+v1.0.0+", want: "v1.0.0"},
		{version: "", want: ""},
	}

	for _, c := range cases {
		got := SanitizeVersionForName(c.version)
		if got != c.want {
			t.Errorf("SanitizeVersionForName(%q) = %q, want %q", c.version, got, c.want)
		}

		if got == "" {
			continue
		}

		if errs := validation.IsDNS1123Subdomain(got); len(errs) > 0 {
			t.Errorf("SanitizeVersionForName(%q) = %q is not a valid DNS-1123 subdomain: %v", c.version, got, errs)
		}
	}
}

func TestMakeEmbeddedModulePackageVersionName(t *testing.T) {
	cases := []struct {
		module  string
		version string
		want    string
	}{
		{module: "deckhouse", version: "v1.78.0", want: "embedded-deckhouse-v1.78.0"},
		{module: "pod-reloader", version: "v1.78.0-pr22189+8776a42", want: "embedded-pod-reloader-v1.78.0-pr22189-8776a42"},
	}

	for _, c := range cases {
		got := MakeEmbeddedModulePackageVersionName(c.module, c.version)
		if got != c.want {
			t.Errorf("MakeEmbeddedModulePackageVersionName(%q, %q) = %q, want %q", c.module, c.version, got, c.want)
		}

		if errs := validation.IsDNS1123Subdomain(got); len(errs) > 0 {
			t.Errorf("name %q is not a valid DNS-1123 subdomain: %v", got, errs)
		}
	}
}
