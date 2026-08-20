package main

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestUpdateBareMetalHostSpecPreservesUnmanagedFields(t *testing.T) {
	bmh := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"architecture": "x86_64",
				"rootDeviceHints": map[string]interface{}{
					"deviceName": "/dev/sda",
				},
				"online":                false,
				"automatedCleaningMode": "disabled",
				"bmc": map[string]interface{}{
					"address":         "redfish+http://old/redfish/v1/Systems/system",
					"credentialsName": "old-bmc",
				},
			},
		},
	}

	spec := instanceSpec{
		Online:                          true,
		BootMACAddress:                  "aa:bb:cc:dd:ee:ff",
		BMCAddress:                      "redfish+http://new/redfish/v1/Systems/system",
		BMCDisableCertificateValidation: true,
	}

	updated, err := updateBareMetalHostSpec(bmh, spec, "new-bmc")
	if err != nil {
		t.Fatalf("update BareMetalHost spec: %v", err)
	}
	if !updated {
		t.Fatal("expected BareMetalHost spec to be updated")
	}

	if got, _, _ := unstructured.NestedString(bmh.Object, "spec", "architecture"); got != "x86_64" {
		t.Fatalf("expected architecture to be preserved, got %q", got)
	}
	if got, _, _ := unstructured.NestedString(bmh.Object, "spec", "rootDeviceHints", "deviceName"); got != "/dev/sda" {
		t.Fatalf("expected rootDeviceHints to be preserved, got %q", got)
	}
	if got, _, _ := unstructured.NestedBool(bmh.Object, "spec", "online"); !got {
		t.Fatal("expected online to be true")
	}
	if got, _, _ := unstructured.NestedString(bmh.Object, "spec", "automatedCleaningMode"); got != "metadata" {
		t.Fatalf("expected automatedCleaningMode to be metadata, got %q", got)
	}
	if got, _, _ := unstructured.NestedString(bmh.Object, "spec", "bootMACAddress"); got != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("expected bootMACAddress to be set, got %q", got)
	}
	if got, _, _ := unstructured.NestedString(bmh.Object, "spec", "bmc", "address"); got != spec.BMCAddress {
		t.Fatalf("expected bmc address to be updated, got %q", got)
	}
	if got, _, _ := unstructured.NestedString(bmh.Object, "spec", "bmc", "credentialsName"); got != "new-bmc" {
		t.Fatalf("expected bmc credentialsName to be updated, got %q", got)
	}
	if got, _, _ := unstructured.NestedBool(bmh.Object, "spec", "bmc", "disableCertificateVerification"); !got {
		t.Fatal("expected disableCertificateVerification to be true")
	}
}

func TestAutomatedCleaningMode(t *testing.T) {
	if got := automatedCleaningMode(false); got != "metadata" {
		t.Fatalf("expected metadata when automated cleaning is enabled, got %q", got)
	}
	if got := automatedCleaningMode(true); got != "disabled" {
		t.Fatalf("expected disabled when automated cleaning is disabled, got %q", got)
	}
}
