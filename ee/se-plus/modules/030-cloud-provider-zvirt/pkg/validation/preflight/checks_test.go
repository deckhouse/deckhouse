/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package preflight

import (
	"testing"

	"k8s.io/utils/ptr"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"

	zicv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/instanceclass/v1"
	zpccv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/pcc/v1"
	zsettingsv2 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/settings/v2"
	zmeta "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/meta"
	zval "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/validation"
)

const masterInstanceClassName = "master-fc613b4dfd67"

func hasViolationCode(result cpvalapi.Result, code string) bool {
	for _, violation := range result.Errors() {
		if violation.Code == code {
			return true
		}
	}

	return false
}

func TestValidatePreflightAcceptsAValidState(t *testing.T) {
	t.Parallel()

	if result := ValidatePreflight(validState(t)); result.HasErrors() {
		t.Fatalf("ValidatePreflight() = %q, want no errors", result.Error())
	}
}

func TestValidatePreflightNilState(t *testing.T) {
	t.Parallel()

	result := ValidatePreflight(nil)
	if !hasViolationCode(result, cpvalapi.CodeInternalStateNil) {
		t.Fatalf("ValidatePreflight(nil) = %q, want %s", result.Error(), cpvalapi.CodeInternalStateNil)
	}
}

// A half-migrated cluster has neither a complete new model nor a reason to be judged by it.
func TestValidatePreflightSkipsPendingMigration(t *testing.T) {
	t.Parallel()

	state := &zval.State{
		MigrationStatus: cpapi.MigrationStatus{MigrationPending: true, LegacyPCCPresent: true},
	}

	if result := ValidatePreflight(state); result.HasErrors() {
		t.Fatalf("ValidatePreflight() during migration = %q, want no errors", result.Error())
	}
}

// The legacy configuration is what terraform still reads while the migration is pending, so a
// broken endpoint there has to be reported even though the new-model rules are skipped.
func TestValidatePreflightChecksLegacyConfigDuringMigration(t *testing.T) {
	t.Parallel()

	state := &zval.State{
		MigrationStatus: cpapi.MigrationStatus{MigrationPending: true, LegacyPCCPresent: true},
		ProviderClusterConfig: &zpccv1.ZvirtProviderClusterConfiguration{
			Provider: zpccv1.ZvirtProvider{Server: "not-a-url"},
		},
	}

	result := ValidatePreflight(state)
	if !hasViolationCode(result, zval.CodeProviderServerInvalid) {
		t.Fatalf("ValidatePreflight() = %q, want %s", result.Error(), zval.CodeProviderServerInvalid)
	}
}

// The legacy static network configuration is projected into the settings map during the migration,
// and the new-model rules are skipped while it is pending — so the legacy surface has to catch a
// short address list before the apply hands the surplus nodes over to DHCP.
func TestValidatePreflightChecksLegacyCustomNetworkConfigDuringMigration(t *testing.T) {
	t.Parallel()

	state := &zval.State{
		MigrationStatus: cpapi.MigrationStatus{MigrationPending: true, LegacyPCCPresent: true},
		ProviderClusterConfig: &zpccv1.ZvirtProviderClusterConfiguration{
			Provider: zpccv1.ZvirtProvider{Server: "https://zvirt.example.com/ovirt-engine/api"},
			MasterNodeGroup: zpccv1.ZvirtMasterNodeGroup{
				Replicas: 2,
				InstanceClass: zpccv1.ZvirtMasterInstanceClass{
					ZvirtInstanceClass: zpccv1.ZvirtInstanceClass{
						CustomNetworkConfig: &zpccv1.ZvirtNetworkConfig{
							NetworkInterfaceAddress: []string{"192.168.1.10"},
						},
					},
				},
			},
		},
	}

	result := ValidatePreflight(state)
	if !hasViolationCode(result, zval.CodeNetworkInterfaceAddressesInsufficient) {
		t.Fatalf("ValidatePreflight() = %q, want %s", result.Error(), zval.CodeNetworkInterfaceAddressesInsufficient)
	}
}

// A legacy configuration whose static network settings are consistent must keep passing preflight:
// the new rules extend, not tighten, what the migration can carry over.
func TestValidatePreflightAcceptsLegacyCustomNetworkConfigDuringMigration(t *testing.T) {
	t.Parallel()

	state := &zval.State{
		MigrationStatus: cpapi.MigrationStatus{MigrationPending: true, LegacyPCCPresent: true},
		ProviderClusterConfig: &zpccv1.ZvirtProviderClusterConfiguration{
			Provider: zpccv1.ZvirtProvider{Server: "https://zvirt.example.com/ovirt-engine/api"},
			MasterNodeGroup: zpccv1.ZvirtMasterNodeGroup{
				Replicas: 2,
				InstanceClass: zpccv1.ZvirtMasterInstanceClass{
					ZvirtInstanceClass: zpccv1.ZvirtInstanceClass{
						CustomNetworkConfig: &zpccv1.ZvirtNetworkConfig{
							NetworkInterfaceAddress: []string{"192.168.1.10", "192.168.1.11"},
							DNSServers:              "8.8.8.8 8.8.4.4",
						},
					},
				},
			},
		},
	}

	if result := ValidatePreflight(state); result.HasErrors() {
		t.Fatalf("ValidatePreflight() = %q, want no errors", result.Error())
	}
}

func TestValidatePreflightRejectsInvalidProviderEndpoint(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.ModuleConfig.Spec.Settings.Provider.Parameters.Server = "zvirt.example.com"

	result := ValidatePreflight(state)
	if !hasViolationCode(result, zval.CodeProviderServerInvalid) {
		t.Fatalf("ValidatePreflight() = %q, want %s", result.Error(), zval.CodeProviderServerInvalid)
	}
}

func TestValidatePreflightRequiresCredentialSecret(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.CredentialSecrets = nil

	result := ValidatePreflight(state)
	if !hasViolationCode(result, cpval.CodeCredentialSecretRequired) {
		t.Fatalf("ValidatePreflight() = %q, want %s", result.Error(), cpval.CodeCredentialSecretRequired)
	}
}

// zVirt authenticates with a login and a password: a Secret missing the identity is unusable.
func TestValidatePreflightRequiresCredentialIdentity(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.CredentialSecrets[0].StringData.Identity = ""

	result := ValidatePreflight(state)
	if !hasViolationCode(result, cpval.CodeCredentialIdentityRequired) {
		t.Fatalf("ValidatePreflight() = %q, want %s", result.Error(), cpval.CodeCredentialIdentityRequired)
	}
}

func TestValidatePreflightRejectsForeignAuthScheme(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.CredentialSecrets[0].StringData.AuthScheme = cpapi.AuthSchemeAPIToken

	result := ValidatePreflight(state)
	if !hasViolationCode(result, cpval.CodeUnsupportedAuthScheme) {
		t.Fatalf("ValidatePreflight() = %q, want %s", result.Error(), cpval.CodeUnsupportedAuthScheme)
	}
}

func TestValidatePreflightRequiresMasterNodeGroup(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups = nil

	result := ValidatePreflight(state)
	if !hasViolationCode(result, cpval.CodeMasterNodeGroupRequired) {
		t.Fatalf("ValidatePreflight() = %q, want %s", result.Error(), cpval.CodeMasterNodeGroupRequired)
	}
}

func TestValidatePreflightRequiresTheReferencedInstanceClass(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.InstanceClasses = nil

	result := ValidatePreflight(state)
	if !hasViolationCode(result, cpval.CodeInstanceClassNotFound) {
		t.Fatalf("ValidatePreflight() = %q, want %s", result.Error(), cpval.CodeInstanceClassNotFound)
	}
}

// The master class carries the etcd disk; without it etcd would share the root disk.
func TestValidatePreflightRequiresMasterEtcdDisk(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.InstanceClasses[0].Spec.EtcdDiskSizeGb = nil

	result := ValidatePreflight(state)
	if !hasViolationCode(result, cpval.CodeMasterEtcdDiskRequired) {
		t.Fatalf("ValidatePreflight() = %q, want %s", result.Error(), cpval.CodeMasterEtcdDiskRequired)
	}
}

func validState(t *testing.T) *zval.State {
	t.Helper()

	return &zval.State{
		ModuleName:    zmeta.ModuleName,
		NamespaceName: zmeta.Namespace,
		ModuleConfig: &cpapi.ModuleConfig[*zsettingsv2.ModuleConfigSettings]{
			ObjectMeta: cpapi.ObjectMeta{Name: zmeta.ModuleName},
			Spec: cpapi.ModuleConfigSpec[*zsettingsv2.ModuleConfigSettings]{
				Enabled: ptr.To(true),
				Version: 2,
				Settings: &zsettingsv2.ModuleConfigSettings{
					Provider: zsettingsv2.Provider{
						Parameters: zsettingsv2.ProviderParameters{
							Server:    "https://zvirt.example.com/ovirt-engine/api",
							ClusterID: "c4bf82a5-b803-40c3-9f6c-b9398378f424",
						},
					},
					Nodes: zsettingsv2.Nodes{
						Parameters: zsettingsv2.NodesParameters{
							SSHPublicKey: "ssh-rsa AAAA",
							Layout:       "Standard",
						},
					},
				},
			},
		},
		CredentialSecrets: []cpapi.CredentialSecret{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: cpapi.CredentialSecretName, Namespace: zmeta.Namespace},
				Type:       cpapi.CredentialsSecretType,
				StringData: cpapi.CredentialSecretStringData{
					AuthScheme: cpapi.AuthSchemeUserPassword,
					Identity:   "admin@internal",
					Secret:     "password",
				},
			},
		},
		NodeGroups: []cpapi.NodeGroup{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "master"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{
							Kind: zicv1.ZvirtInstanceClassKind,
							Name: masterInstanceClassName,
						},
					},
				},
			},
		},
		InstanceClasses: []*zicv1.ZvirtInstanceClass{
			masterInstanceClass(masterInstanceClassName),
		},
	}
}

func masterInstanceClass(name string) *zicv1.ZvirtInstanceClass {
	class := &zicv1.ZvirtInstanceClass{}
	class.Kind = zicv1.ZvirtInstanceClassKind
	class.Name = name
	class.Spec = zicv1.InstanceClassSpec{
		NumCPUs:         4,
		Memory:          8192,
		Template:        "debian-bookworm",
		VNICProfileID:   "0000-1111",
		RootDiskSizeGb:  50,
		EtcdDiskSizeGb:  ptr.To(10),
		StorageDomainID: "c4bf82a5-b803-40c3-9f6c-b9398378f424",
	}

	return class
}
