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
		if bundledCRD.storageVersion == "" {
			t.Fatalf("expected storage version for %q to be populated", bundledCRD.crd.Name)
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
			crd:            testCRD("gateways.gateway.networking.k8s.io", "v1"),
			storageVersion: "v1",
		},
		{
			crd:            testCRD("httproutes.gateway.networking.k8s.io", "v1"),
			storageVersion: "v1",
		},
	}

	t.Run("creates missing CRDs", func(t *testing.T) {
		t.Parallel()

		toCreate, mismatches, err := evaluateGatewayAPICRDState(bundledCRDs, map[string]*apiextensionsv1.CustomResourceDefinition{})
		if err != nil {
			t.Fatalf("evaluateGatewayAPICRDState returned error: %v", err)
		}
		if len(mismatches) != 0 {
			t.Fatalf("expected no mismatches, got %v", mismatches)
		}
		if len(toCreate) != len(bundledCRDs) {
			t.Fatalf("expected %d CRDs to create, got %d", len(bundledCRDs), len(toCreate))
		}
	})

	t.Run("reports storage version mismatches", func(t *testing.T) {
		t.Parallel()

		clusterCRDs := map[string]*apiextensionsv1.CustomResourceDefinition{
			"gateways.gateway.networking.k8s.io":   testCRD("gateways.gateway.networking.k8s.io", "v1beta1"),
			"httproutes.gateway.networking.k8s.io": testCRD("httproutes.gateway.networking.k8s.io", "v1"),
		}

		toCreate, mismatches, err := evaluateGatewayAPICRDState(bundledCRDs, clusterCRDs)
		if err != nil {
			t.Fatalf("evaluateGatewayAPICRDState returned error: %v", err)
		}
		if len(toCreate) != 0 {
			t.Fatalf("expected no CRDs to create when all exist, got %d", len(toCreate))
		}
		if len(mismatches) != 1 {
			t.Fatalf("expected 1 mismatch, got %d (%v)", len(mismatches), mismatches)
		}
		if !strings.Contains(mismatches[0], "gateways.gateway.networking.k8s.io") {
			t.Fatalf("expected mismatch to mention CRD name, got %q", mismatches[0])
		}
	})
}

func testCRD(name, storageVersion string) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
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
