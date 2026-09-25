/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"strings"
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"

	zicv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/instanceclass/v1"
	zmeta "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/meta"
)

const masterInstanceClassName = "master-fc613b4dfd67"

// These tests go through the wire shape dhctl actually sends, not through the rule functions:
// the decoding in between is where a renamed field or a wrong namespace silently turns a real
// violation into a clean bill of health.

func testSettings() map[string]any {
	return map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata":   map[string]any{"name": zmeta.ModuleName},
		"spec": map[string]any{
			"enabled": true,
			"version": 2,
			"settings": map[string]any{
				"provider": map[string]any{
					"parameters": map[string]any{
						"server":    "https://zvirt.example.com/ovirt-engine/api",
						"clusterID": "c4bf82a5-b803-40c3-9f6c-b9398378f424",
					},
				},
				"nodes": map[string]any{
					"parameters": map[string]any{
						"sshPublicKey": "ssh-rsa AAAA",
						"layout":       "Standard",
					},
				},
			},
		},
	}
}

// The credential Secret is keyed <namespace>/<name> on this path, not by bare name.
func testSecrets() map[string]map[string]any {
	return map[string]map[string]any{
		zmeta.Namespace + "/" + cpapi.CredentialSecretName: {
			"metadata": map[string]any{
				"name":      cpapi.CredentialSecretName,
				"namespace": zmeta.Namespace,
			},
			"type": validatev1.CredentialsSecretType,
			"stringData": map[string]any{
				"authScheme": string(cpapi.AuthSchemeUserPassword),
				"identity":   "admin@internal",
				"secret":     "password",
			},
		},
	}
}

func testNodeGroups() map[string]map[string]any {
	return map[string]map[string]any{
		"master": {
			"metadata": map[string]any{"name": "master"},
			"spec": map[string]any{
				"nodeType": "CloudPermanent",
				"cloudInstances": map[string]any{
					"classReference": map[string]any{
						"kind": zicv1.ZvirtInstanceClassKind,
						"name": masterInstanceClassName,
					},
				},
			},
		},
	}
}

func testInstanceClasses() map[string]map[string]any {
	return map[string]map[string]any{
		masterInstanceClassName: {
			"metadata": map[string]any{"name": masterInstanceClassName},
			"spec": map[string]any{
				"numCPUs":        4,
				"memory":         8192,
				"template":       "debian-bookworm",
				"vnicProfileID":  "0000-1111",
				"rootDiskSizeGb": 50,
				"etcdDiskSizeGb": 10,
			},
		},
	}
}

func testInput(operation validatev1.Operation) validatev1.Input {
	return validatev1.Input{
		ProviderName:  "zvirt",
		ClusterPrefix: "test",
		Layout:        "Standard",
		Operation:     operation,
		CloudProviderVars: &validatev1.CloudProviderVars{
			Settings:        testSettings(),
			Secrets:         testSecrets(),
			NodeGroups:      testNodeGroups(),
			InstanceClasses: testInstanceClasses(),
		},
	}
}

// The legacy providerClusterConfiguration section travels as a bare top-level map, not wrapped in
// apiVersion/kind/spec: dhctl flattens it before putting it on the wire.
func testProviderClusterConfig(masterReplicas int, masterAddresses []string) map[string]any {
	return map[string]any{
		"layout":       "Standard",
		"sshPublicKey": "ssh-rsa AAAA",
		"clusterID":    "c4bf82a5-b803-40c3-9f6c-b9398378f424",
		"masterNodeGroup": map[string]any{
			"replicas": masterReplicas,
			"instanceClass": map[string]any{
				"numCPUs":  4,
				"memory":   8192,
				"template": "debian-bookworm",
				"customNetworkConfig": map[string]any{
					"networkInterfaceName":    "enp1s0",
					"networkInterfaceAddress": masterAddresses,
					"networkInterfaceNetmask": "255.255.255.0",
					"networkInterfaceGateway": "192.168.1.1",
					"dnsServers":              "8.8.8.8 8.8.4.4",
				},
			},
		},
		"provider": map[string]any{
			"server": "https://zvirt.example.com/ovirt-engine/api",
		},
	}
}

func TestValidateAcceptsACompleteConfiguration(t *testing.T) {
	t.Parallel()

	for _, operation := range []validatev1.Operation{validatev1.OperationBootstrap, validatev1.OperationConverge} {
		t.Run(string(operation), func(t *testing.T) {
			t.Parallel()

			result, err := validate(context.Background(), testInput(operation))
			if err != nil {
				t.Fatalf("validate(%s) = %v, want a result", operation, err)
			}

			if result.HasErrors() {
				t.Fatalf("validate(%s) = %q, want no violations", operation, result.Error())
			}
		})
	}
}

// A cluster that still lives on the legacy providerClusterConfiguration has no ModuleConfig
// customNetworkConfigs to check, so the same address-list rules must run against the PCC itself.
func TestValidateAcceptsALegacyCustomNetworkConfig(t *testing.T) {
	t.Parallel()

	input := testInput(validatev1.OperationConverge)
	input.ProviderClusterConfig = testProviderClusterConfig(1, []string{"192.168.1.10"})

	result, err := validate(context.Background(), input)
	if err != nil {
		t.Fatalf("validate() = %v, want a result", err)
	}

	if result.HasErrors() {
		t.Fatalf("validate() = %q, want no violations", result.Error())
	}
}

func TestValidateReportsViolations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*validatev1.Input)
		wantSub string
	}{
		{
			name:    "no credential Secret",
			mutate:  func(in *validatev1.Input) { in.CloudProviderVars.Secrets = nil },
			wantSub: cpapi.CredentialSecretName,
		},
		{
			name: "credential Secret without a login",
			mutate: func(in *validatev1.Input) {
				secret := in.CloudProviderVars.Secrets[zmeta.Namespace+"/"+cpapi.CredentialSecretName]
				delete(secret["stringData"].(map[string]any), "identity")
			},
			wantSub: "identity is required",
		},
		{
			name:    "no master NodeGroup",
			mutate:  func(in *validatev1.Input) { in.CloudProviderVars.NodeGroups = nil },
			wantSub: `NodeGroup "master" is required`,
		},
		{
			name:    "the referenced InstanceClass is missing",
			mutate:  func(in *validatev1.Input) { in.CloudProviderVars.InstanceClasses = nil },
			wantSub: "was not found",
		},
		{
			name: "the master class has no etcd disk",
			mutate: func(in *validatev1.Input) {
				spec := in.CloudProviderVars.InstanceClasses[masterInstanceClassName]["spec"].(map[string]any)
				delete(spec, "etcdDiskSizeGb")
			},
			// The shared rule must name the field zVirt actually has, not the etcdDisk of other
			// providers: an operator told to set a non-existent field cannot act on the error.
			wantSub: "must define spec.etcdDiskSizeGb",
		},
		{
			name: "the API endpoint is not a URL",
			mutate: func(in *validatev1.Input) {
				settings := in.CloudProviderVars.Settings["spec"].(map[string]any)["settings"].(map[string]any)
				settings["provider"].(map[string]any)["parameters"].(map[string]any)["server"] = "zvirt.example.com"
			},
			wantSub: "invalid zVirt API endpoint",
		},
		{
			name: "the CA bundle is not base64",
			mutate: func(in *validatev1.Input) {
				settings := in.CloudProviderVars.Settings["spec"].(map[string]any)["settings"].(map[string]any)
				settings["provider"].(map[string]any)["parameters"].(map[string]any)["caBundle"] = "%%% not base64 %%%"
			},
			wantSub: "invalid CA bundle",
		},
		{
			name: "the legacy address list is shorter than the master replicas",
			mutate: func(in *validatev1.Input) {
				in.ProviderClusterConfig = testProviderClusterConfig(2, []string{"192.168.1.10"})
			},
			wantSub: "networkInterfaceAddress",
		},
		{
			name: "the legacy address list repeats an address",
			mutate: func(in *validatev1.Input) {
				in.ProviderClusterConfig = testProviderClusterConfig(2, []string{"192.168.1.10", "192.168.1.10"})
			},
			wantSub: "must not contain duplicates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := testInput(validatev1.OperationConverge)
			tt.mutate(&input)

			result, err := validate(context.Background(), input)
			if err != nil {
				t.Fatalf("validate() = %v, want a result", err)
			}

			if !result.HasErrors() {
				t.Fatalf("validate() = no violations, want one mentioning %q", tt.wantSub)
			}

			if !strings.Contains(result.Error(), tt.wantSub) {
				t.Fatalf("validate() = %q, want it to mention %q", result.Error(), tt.wantSub)
			}
		})
	}
}

// A cluster is often destroyed precisely because its configuration is broken; refusing to tear it
// down would leave the operator stuck with infrastructure they are trying to release.
func TestValidateExemptsDestroy(t *testing.T) {
	t.Parallel()

	input := testInput(validatev1.OperationDestroy)
	input.CloudProviderVars.Secrets = nil
	input.CloudProviderVars.NodeGroups = nil
	input.CloudProviderVars.InstanceClasses = nil

	result, err := validate(context.Background(), input)
	if err != nil {
		t.Fatalf("validate(destroy) = %v, want a result", err)
	}

	if result.HasErrors() {
		t.Fatalf("validate(destroy) = %q, want no violations", result.Error())
	}
}

// The response keeps errors and warnings apart: a warning must reach the operator without dhctl
// refusing to proceed, so collapsing the two would either hide it or block a working cluster.
func TestToResponseSeparatesWarningsFromErrors(t *testing.T) {
	t.Parallel()

	result := cpvalapi.Result{}
	result.AddError("Secret/d8-credentials", "credential_secret_required", nil, "credential Secret is required")
	result.AddWarning("ModuleConfig.spec.settings.provider.parameters.caBundle", "provider_ca_bundle_ignored", "masked", "caBundle is ignored")

	response := toResponse(result)

	if len(response.Errors) != 1 || len(response.Warnings) != 1 {
		t.Fatalf("toResponse() = %d errors and %d warnings, want one of each", len(response.Errors), len(response.Warnings))
	}

	gotError := response.Errors[0]
	if gotError.Path != "Secret/d8-credentials" || gotError.Code != "credential_secret_required" {
		t.Errorf("error violation = %+v, want the path and code carried through", gotError)
	}

	// A nil value must not become the string "<nil>" in the operator's face.
	if gotError.Value != "" {
		t.Errorf("error violation value = %q, want it left empty", gotError.Value)
	}

	if got := response.Warnings[0].Value; got != "masked" {
		t.Errorf("warning violation value = %q, want %q", got, "masked")
	}
}

// An empty result is an empty response, not a response carrying one blank violation.
func TestToResponseOfACleanResultIsEmpty(t *testing.T) {
	t.Parallel()

	response := toResponse(cpvalapi.Result{})
	if len(response.Errors) != 0 || len(response.Warnings) != 0 {
		t.Fatalf("toResponse() = %+v, want an empty response", response)
	}
}
