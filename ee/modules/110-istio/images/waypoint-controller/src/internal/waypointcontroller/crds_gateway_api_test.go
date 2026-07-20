/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package waypointcontroller

import (
	"strings"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLoadBundledGatewayAPICRDs(t *testing.T) {
	t.Parallel()

	bundledCRDs, err := loadBundledGatewayAPICRDs()
	if err != nil {
		t.Fatalf("loadBundledGatewayAPICRDs returned error: %v", err)
	}
	if len(bundledCRDs) == 0 {
		t.Fatal("expected bundled CRDs to be loaded")
	}

	for _, bundledCRD := range bundledCRDs {
		if bundledCRD.crd == nil {
			t.Fatal("expected bundled CRD object to be populated")
		}
		if bundledCRD.crd.Name == "" {
			t.Fatal("expected bundled CRD name to be populated")
		}
		if bundledCRD.minimumBundleVersion == "" {
			t.Fatalf("expected minimum bundle version for %q to be populated", bundledCRD.crd.Name)
		}
		if bundledCRD.minimumServedVersion == "" {
			t.Fatalf("expected minimum served version for %q to be populated", bundledCRD.crd.Name)
		}
		if bundledCRD.crd.Name == "gateways.gateway.networking.k8s.io" {
			if bundledCRD.requiredExactServedVersion != "v1" {
				t.Fatalf("expected Gateway CRD to require exact served version v1, got %q", bundledCRD.requiredExactServedVersion)
			}
		} else if bundledCRD.requiredExactServedVersion != "" {
			t.Fatalf("did not expect exact served version requirement for %q", bundledCRD.crd.Name)
		}
		if !strings.HasPrefix(bundledCRD.manifestPath, "crds_gateway_api/") {
			t.Fatalf("unexpected manifest path %q", bundledCRD.manifestPath)
		}
	}
}

func TestEvaluateGatewayAPICRDState(t *testing.T) {
	t.Parallel()

	bundledCRDs := []bundledGatewayAPICRD{
		{
			crd:                        testCRD("gateways.gateway.networking.k8s.io", "v1.5.1", "v1"),
			minimumBundleVersion:       "v1.5.1",
			minimumServedVersion:       "v1",
			requiredExactServedVersion: "v1",
		},
		{
			crd:                  testCRD("httproutes.gateway.networking.k8s.io", "v1.5.1", "v1"),
			minimumBundleVersion: "v1.5.1",
			minimumServedVersion: "v1",
		},
	}

	t.Run("creates missing CRDs", func(t *testing.T) {
		t.Parallel()

		toCreate, mismatches := evaluateGatewayAPICRDState(bundledCRDs, map[string]*apiextensionsv1.CustomResourceDefinition{})
		if len(mismatches) != 0 {
			t.Fatalf("expected no mismatches, got %v", mismatches)
		}
		if len(toCreate) != len(bundledCRDs) {
			t.Fatalf("expected %d CRDs to create, got %d", len(bundledCRDs), len(toCreate))
		}
	})

	t.Run("accepts equal and newer bundle versions when minimum API version is served", func(t *testing.T) {
		t.Parallel()

		clusterCRDs := map[string]*apiextensionsv1.CustomResourceDefinition{
			"gateways.gateway.networking.k8s.io":   testCRD("gateways.gateway.networking.k8s.io", "v1.5.1", "v1"),
			"httproutes.gateway.networking.k8s.io": testCRD("httproutes.gateway.networking.k8s.io", "v1.6.0", "v1"),
		}

		toCreate, mismatches := evaluateGatewayAPICRDState(bundledCRDs, clusterCRDs)
		if len(toCreate) != 0 || len(mismatches) != 0 {
			t.Fatalf("expected all CRDs to be compatible, got toCreate=%d mismatches=%v", len(toCreate), mismatches)
		}
	})

	t.Run("accepts missing or invalid bundle version when minimum API version is served", func(t *testing.T) {
		t.Parallel()

		clusterCRDs := map[string]*apiextensionsv1.CustomResourceDefinition{
			"gateways.gateway.networking.k8s.io":   testCRD("gateways.gateway.networking.k8s.io", "", "v1"),
			"httproutes.gateway.networking.k8s.io": testCRD("httproutes.gateway.networking.k8s.io", "not-semver", "v1"),
		}

		toCreate, mismatches := evaluateGatewayAPICRDState(bundledCRDs, clusterCRDs)
		if len(toCreate) != 0 || len(mismatches) != 0 {
			t.Fatalf("expected all CRDs to be compatible, got toCreate=%d mismatches=%v", len(toCreate), mismatches)
		}
	})

	t.Run("requires the exact API endpoint used by the controller", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name     string
			versions []apiextensionsv1.CustomResourceDefinitionVersion
			wantOK   bool
		}{
			{
				name:     "only higher version",
				versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v2", Served: true, Storage: true}},
			},
			{
				name: "exact and higher versions",
				versions: []apiextensionsv1.CustomResourceDefinitionVersion{
					{Name: "v1", Served: true},
					{Name: "v2", Served: true, Storage: true},
				},
				wantOK: true,
			},
			{
				name: "exact version not served",
				versions: []apiextensionsv1.CustomResourceDefinitionVersion{
					{Name: "v1", Served: false},
					{Name: "v2", Served: true, Storage: true},
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				gatewayCRD := testCRD("gateways.gateway.networking.k8s.io", "v1.5.1", "v1")
				gatewayCRD.Spec.Versions = tc.versions
				clusterCRDs := map[string]*apiextensionsv1.CustomResourceDefinition{
					"gateways.gateway.networking.k8s.io":   gatewayCRD,
					"httproutes.gateway.networking.k8s.io": testCRD("httproutes.gateway.networking.k8s.io", "v1.5.1", "v1"),
				}

				_, mismatches := evaluateGatewayAPICRDState(bundledCRDs, clusterCRDs)
				if tc.wantOK && len(mismatches) != 0 {
					t.Fatalf("expected Gateway CRD to be compatible, got %v", mismatches)
				}
				if !tc.wantOK && (len(mismatches) != 1 || !strings.Contains(mismatches[0], "required API version v1 is not served")) {
					t.Fatalf("expected exact version mismatch, got %v", mismatches)
				}
			})
		}
	})

	t.Run("continues to accept a higher served API version for indirectly used resources", func(t *testing.T) {
		t.Parallel()

		minimumV1Beta1 := append([]bundledGatewayAPICRD(nil), bundledCRDs...)
		minimumV1Beta1[1].minimumServedVersion = "v1beta1"
		clusterCRDs := map[string]*apiextensionsv1.CustomResourceDefinition{
			"gateways.gateway.networking.k8s.io":   testCRD("gateways.gateway.networking.k8s.io", "v1.5.1", "v1"),
			"httproutes.gateway.networking.k8s.io": testCRD("httproutes.gateway.networking.k8s.io", "v1.5.1", "v1"),
		}

		toCreate, mismatches := evaluateGatewayAPICRDState(minimumV1Beta1, clusterCRDs)
		if len(toCreate) != 0 || len(mismatches) != 0 {
			t.Fatalf("expected all CRDs to be compatible, got toCreate=%d mismatches=%v", len(toCreate), mismatches)
		}
	})

	for _, tc := range []struct {
		name          string
		bundleVersion string
		servedVersion string
		wantSubstring string
	}{
		{name: "reports older valid bundle versions", bundleVersion: "v1.5.0", servedVersion: "v1", wantSubstring: "cluster bundle=v1.5.0"},
		{name: "reports equal bundle version when required API endpoint is not served", bundleVersion: "v1.5.1", servedVersion: "v1beta1", wantSubstring: "required API version v1 is not served"},
		{name: "reports missing bundle version when required API endpoint is not served", bundleVersion: "", servedVersion: "v1beta1", wantSubstring: "required API version v1 is not served"},
		{name: "reports invalid bundle version when required API endpoint is not served", bundleVersion: "1.6.0", servedVersion: "v1beta1", wantSubstring: "required API version v1 is not served"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			clusterCRDs := map[string]*apiextensionsv1.CustomResourceDefinition{
				"gateways.gateway.networking.k8s.io":   testCRD("gateways.gateway.networking.k8s.io", tc.bundleVersion, tc.servedVersion),
				"httproutes.gateway.networking.k8s.io": testCRD("httproutes.gateway.networking.k8s.io", "v1.5.1", "v1"),
			}

			toCreate, mismatches := evaluateGatewayAPICRDState(bundledCRDs, clusterCRDs)
			if len(toCreate) != 0 {
				t.Fatalf("expected no CRDs to create when all exist, got %d", len(toCreate))
			}
			if len(mismatches) != 1 || !strings.Contains(mismatches[0], tc.wantSubstring) {
				t.Fatalf("expected one mismatch containing %q, got %v", tc.wantSubstring, mismatches)
			}
		})
	}
}

func TestMinimumServedVersionForCRD(t *testing.T) {
	t.Parallel()

	newCRD := func(versions ...apiextensionsv1.CustomResourceDefinitionVersion) *apiextensionsv1.CustomResourceDefinition {
		return &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: "example.gateway.networking.k8s.io"},
			Spec:       apiextensionsv1.CustomResourceDefinitionSpec{Versions: versions},
		}
	}

	t.Run("returns the storage version regardless of ordering", func(t *testing.T) {
		t.Parallel()

		// A served, non-storage version precedes the storage version. The result
		// must be the storage version, not the first served version, so the
		// compatibility contract does not depend on spec.versions ordering.
		crd := newCRD(
			apiextensionsv1.CustomResourceDefinitionVersion{Name: "v1beta1", Served: true, Storage: false},
			apiextensionsv1.CustomResourceDefinitionVersion{Name: "v1", Served: true, Storage: true},
		)

		got, err := minimumServedVersionForCRD(crd)
		if err != nil {
			t.Fatalf("minimumServedVersionForCRD returned error: %v", err)
		}
		if got != "v1" {
			t.Fatalf("expected minimum served version v1, got %q", got)
		}
	})

	t.Run("errors when the storage version is not served", func(t *testing.T) {
		t.Parallel()

		crd := newCRD(
			apiextensionsv1.CustomResourceDefinitionVersion{Name: "v1beta1", Served: true, Storage: false},
			apiextensionsv1.CustomResourceDefinitionVersion{Name: "v1", Served: false, Storage: true},
		)

		if _, err := minimumServedVersionForCRD(crd); err == nil {
			t.Fatal("expected an error when the storage version is not served")
		}
	})

	t.Run("errors when no storage version is declared", func(t *testing.T) {
		t.Parallel()

		crd := newCRD(
			apiextensionsv1.CustomResourceDefinitionVersion{Name: "v1", Served: true, Storage: false},
		)

		if _, err := minimumServedVersionForCRD(crd); err == nil {
			t.Fatal("expected an error when no storage version is declared")
		}
	})
}

func TestServesVersionOrHigher(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name           string
		minimumVersion string
		servedVersion  string
		served         bool
		want           bool
	}{
		{name: "exact version", minimumVersion: "v1beta1", servedVersion: "v1beta1", served: true, want: true},
		{name: "stable satisfies beta minimum", minimumVersion: "v1beta1", servedVersion: "v1", served: true, want: true},
		{name: "newer alpha satisfies minimum", minimumVersion: "v1alpha2", servedVersion: "v1alpha3", served: true, want: true},
		{name: "older alpha does not satisfy minimum", minimumVersion: "v1alpha3", servedVersion: "v1alpha2", served: true, want: false},
		{name: "non-served higher version does not satisfy minimum", minimumVersion: "v1beta1", servedVersion: "v1", served: false, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			crd := testCRD("example.gateway.networking.k8s.io", "", tc.servedVersion)
			crd.Spec.Versions[0].Served = tc.served
			if got := servesVersionOrHigher(crd, tc.minimumVersion); got != tc.want {
				t.Fatalf("servesVersionOrHigher() = %v, want %v", got, tc.want)
			}
		})
	}
}

func testCRD(name, bundleVersion, storageVersion string) *apiextensionsv1.CustomResourceDefinition {
	annotations := map[string]string{}
	if bundleVersion != "" {
		annotations[gatewayAPIBundleVersionAnnotation] = bundleVersion
	}

	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    storageVersion,
					Served:  true,
					Storage: true,
				},
			},
		},
	}
}
