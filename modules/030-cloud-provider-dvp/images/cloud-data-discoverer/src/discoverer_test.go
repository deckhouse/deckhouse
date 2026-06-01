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

package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
)

func TestMergeStorageDomainsAlwaysUsesWaitForFirstConsumer(t *testing.T) {
	immediate := storagev1.VolumeBindingImmediate
	deletePolicy := corev1.PersistentVolumeReclaimDelete
	allowVolumeExpansion := true

	result := mergeStorageDomains([]cloudDataV1.DVPStorageClass{
		{
			Name:                 "legacy",
			VolumeBindingMode:    "Immediate",
			ReclaimPolicy:        "Retain",
			AllowVolumeExpansion: false,
			IsEnabled:            true,
			IsDefault:            true,
		},
	}, []storagev1.StorageClass{
		{
			ObjectMeta:           newObjectMeta("replicated", map[string]string{stableDefaultAnnotation: "true"}),
			VolumeBindingMode:    &immediate,
			ReclaimPolicy:        &deletePolicy,
			AllowVolumeExpansion: &allowVolumeExpansion,
		},
		{
			ObjectMeta:        newObjectMeta("skipped", map[string]string{skipSCAnnotation: "TRUE"}),
			ReclaimPolicy:     &deletePolicy,
			VolumeBindingMode: &immediate,
		},
	})

	if len(result) != 3 {
		t.Fatalf("expected three storage classes, got %d", len(result))
	}

	if result[0].Name != "legacy" || result[1].Name != "replicated" || result[2].Name != "skipped" {
		t.Fatalf("expected sorted storage classes [legacy replicated skipped], got [%s %s %s]", result[0].Name, result[1].Name, result[2].Name)
	}

	if result[0].VolumeBindingMode != "Immediate" {
		t.Fatalf("expected stored storage class to preserve existing volumeBindingMode, got %q", result[0].VolumeBindingMode)
	}

	if result[0].IsDefault {
		t.Fatalf("expected stored-only storage class default flag to be reset")
	}

	if result[1].VolumeBindingMode != string(storagev1.VolumeBindingWaitForFirstConsumer) {
		t.Fatalf("expected volumeBindingMode %q, got %q", storagev1.VolumeBindingWaitForFirstConsumer, result[1].VolumeBindingMode)
	}

	if result[1].ReclaimPolicy != string(deletePolicy) {
		t.Fatalf("expected reclaimPolicy %q, got %q", deletePolicy, result[1].ReclaimPolicy)
	}

	if result[1].AllowVolumeExpansion != allowVolumeExpansion {
		t.Fatalf("expected allowVolumeExpansion %t, got %t", allowVolumeExpansion, result[1].AllowVolumeExpansion)
	}

	if !result[1].IsEnabled || !result[1].IsDefault {
		t.Fatalf("expected replicated storage class to be enabled and default, got enabled=%t default=%t", result[1].IsEnabled, result[1].IsDefault)
	}

	if result[2].VolumeBindingMode != string(storagev1.VolumeBindingWaitForFirstConsumer) {
		t.Fatalf("expected skipped storage class volumeBindingMode %q, got %q", storagev1.VolumeBindingWaitForFirstConsumer, result[2].VolumeBindingMode)
	}

	if result[2].IsEnabled {
		t.Fatalf("expected skipped storage class to be disabled")
	}

	if result[2].IsDefault {
		t.Fatalf("expected skipped storage class to not be default")
	}
}

func TestMergeStorageDomainsSupportsBetaDefaultAnnotation(t *testing.T) {
	result := mergeStorageDomains(nil, []storagev1.StorageClass{
		{
			ObjectMeta: newObjectMeta("beta-default", map[string]string{
				betaDefaultAnnotation: "TrUe",
			}),
		},
	})

	if len(result) != 1 {
		t.Fatalf("expected one storage class, got %d", len(result))
	}

	if !result[0].IsDefault {
		t.Fatalf("expected storage class with beta default annotation to be default")
	}
}

func TestMergeStorageDomainsUsesDefaultsForNilFieldsAndSkipsStoredDuplicates(t *testing.T) {
	result := mergeStorageDomains([]cloudDataV1.DVPStorageClass{
		{
			Name:                 "duplicated",
			VolumeBindingMode:    "Immediate",
			ReclaimPolicy:        "Retain",
			AllowVolumeExpansion: true,
			IsEnabled:            false,
			IsDefault:            true,
		},
		{
			Name:                 "stored-only",
			VolumeBindingMode:    "Immediate",
			ReclaimPolicy:        "Retain",
			AllowVolumeExpansion: true,
			IsEnabled:            true,
			IsDefault:            true,
		},
	}, []storagev1.StorageClass{
		{
			ObjectMeta: newObjectMeta("duplicated", nil),
		},
	})

	if len(result) != 2 {
		t.Fatalf("expected two storage classes, got %d", len(result))
	}

	if result[0].Name != "duplicated" || result[1].Name != "stored-only" {
		t.Fatalf("unexpected storage class order: [%s %s]", result[0].Name, result[1].Name)
	}

	if result[0].ReclaimPolicy != string(corev1.PersistentVolumeReclaimDelete) {
		t.Fatalf("expected default reclaim policy %q, got %q", corev1.PersistentVolumeReclaimDelete, result[0].ReclaimPolicy)
	}

	if result[0].AllowVolumeExpansion {
		t.Fatalf("expected allowVolumeExpansion to default to false")
	}

	if result[0].IsDefault {
		t.Fatalf("expected discovered storage class without annotations to not be default")
	}

	if result[1].Name != "stored-only" {
		t.Fatalf("expected stored-only storage class to be preserved")
	}
}

func newObjectMeta(name string, annotations map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Annotations: annotations,
	}
}
