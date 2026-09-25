/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package validation

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"slices"
	"strings"
	"testing"
	"time"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"

	zicv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/instanceclass/v1"
	zpccv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/pcc/v1"
	zsettingsv2 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/settings/v2"
)

// testCABundle builds the base64-encoded PEM bundle the settings carry. The rule parses the
// certificate and insists it is a CA, so the fixture has to be a real one — generating it keeps it
// valid by construction rather than by a pasted blob that expires.
func testCABundle(t *testing.T) string {
	t.Helper()

	return testCertificateBundle(t, true)
}

// testLeafCertificateBundle is a well-formed certificate that is not a CA.
func testLeafCertificateBundle(t *testing.T) string {
	t.Helper()

	return testCertificateBundle(t, false)
}

func testCertificateBundle(t *testing.T, isCA bool) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "zvirt-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  isCA,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	return base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// testPEMBlock encodes an arbitrary PEM block, used to check that a non-certificate block is
// rejected even though it is perfectly well-formed PEM.
func testPEMBlock(blockType string) string {
	return base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: []byte("payload")}))
}

func hasCode(violations []cpvalapi.Violation, code string) bool {
	for _, violation := range violations {
		if violation.Code == code {
			return true
		}
	}

	return false
}

func TestValidateProviderConnection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		server      string
		caBundle    func(*testing.T) string
		insecure    bool
		wantError   string
		wantWarning string
	}{
		{
			name:     "a plain https endpoint is accepted",
			server:   "https://zvirt.example.com/ovirt-engine/api",
			caBundle: testCABundle,
		},
		{
			name:      "an empty endpoint is rejected",
			server:    "   ",
			wantError: CodeProviderServerRequired,
		},
		{
			name:      "an endpoint without a scheme is rejected",
			server:    "zvirt.example.com/ovirt-engine/api",
			wantError: CodeProviderServerInvalid,
		},
		{
			name:      "an endpoint with a foreign scheme is rejected",
			server:    "ftp://zvirt.example.com",
			wantError: CodeProviderServerInvalid,
		},
		{
			name:      "an endpoint without a host is rejected",
			server:    "https:///ovirt-engine/api",
			wantError: CodeProviderServerInvalid,
		},
		{
			name:      "a CA bundle that is not base64 is rejected",
			server:    "https://zvirt.example.com",
			caBundle:  func(*testing.T) string { return "%%% not base64 %%%" },
			wantError: CodeProviderCABundleInvalid,
		},
		{
			name:      "a base64 CA bundle that is not PEM is rejected",
			server:    "https://zvirt.example.com",
			caBundle:  func(*testing.T) string { return base64.StdEncoding.EncodeToString([]byte("just some text")) },
			wantError: CodeProviderCABundleInvalid,
		},
		{
			name:      "a PEM block of the wrong type is rejected",
			server:    "https://zvirt.example.com",
			caBundle:  func(*testing.T) string { return testPEMBlock("RSA PRIVATE KEY") },
			wantError: CodeProviderCABundleInvalid,
		},
		{
			name:      "a well-formed certificate that is not a CA is rejected",
			server:    "https://zvirt.example.com",
			caBundle:  testLeafCertificateBundle,
			wantError: CodeProviderCABundleInvalid,
		},
		{
			name:        "a CA bundle together with insecure only warns",
			server:      "https://zvirt.example.com",
			caBundle:    testCABundle,
			insecure:    true,
			wantWarning: CodeProviderCABundleIgnored,
		},
		{
			name:     "insecure without a CA bundle is silent",
			server:   "https://zvirt.example.com",
			insecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			caBundle := ""
			if tt.caBundle != nil {
				caBundle = tt.caBundle(t)
			}

			state := &State{
				ModuleConfig: &cpapi.ModuleConfig[*zsettingsv2.ModuleConfigSettings]{
					Spec: cpapi.ModuleConfigSpec[*zsettingsv2.ModuleConfigSettings]{
						Settings: &zsettingsv2.ModuleConfigSettings{
							Provider: zsettingsv2.Provider{
								Parameters: zsettingsv2.ProviderParameters{
									Server:   tt.server,
									CABundle: caBundle,
									Insecure: tt.insecure,
								},
							},
						},
					},
				},
			}

			result := ValidateProviderConnection(state)
			assertResult(t, result, tt.wantError, tt.wantWarning)

			// The legacy configuration must be held to exactly the same standard: the whole point
			// of sharing the rule is that a cluster cannot pass one surface and fail the other.
			legacy := ValidateLegacyProviderConnection(&zpccv1.ZvirtProviderClusterConfiguration{
				Provider: zpccv1.ZvirtProvider{
					Server:   tt.server,
					CABundle: caBundle,
					Insecure: tt.insecure,
				},
			})
			assertResult(t, legacy, tt.wantError, tt.wantWarning)
		})
	}
}

func assertResult(t *testing.T, result cpvalapi.Result, wantError, wantWarning string) {
	t.Helper()

	switch {
	case wantError == "" && result.HasErrors():
		t.Errorf("unexpected errors: %v", result.Error())
	case wantError != "" && !hasCode(result.Errors(), wantError):
		t.Errorf("errors = %v, want code %s", result.Error(), wantError)
	}

	if wantWarning == "" {
		if len(result.Warnings()) != 0 {
			t.Errorf("unexpected warnings: %v", result.Warnings())
		}

		return
	}

	if !hasCode(result.Warnings(), wantWarning) {
		t.Errorf("warnings = %v, want code %s", result.Warnings(), wantWarning)
	}
}

// The schema cannot carry uniqueItems, so this rule is the only thing standing between an operator
// and two nodes sharing an IP address.
func TestValidateCustomNetworkConfigs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		addresses []string
		dnsServer []string
		wantError string
	}{
		{
			name:      "distinct values are accepted",
			addresses: []string{"192.168.1.10", "192.168.1.11"},
			dnsServer: []string{"8.8.8.8", "8.8.4.4"},
		},
		{
			name:      "empty lists are accepted",
			addresses: nil,
			dnsServer: []string{},
		},
		{
			name:      "a repeated address is rejected",
			addresses: []string{"192.168.1.10", "192.168.1.10"},
			wantError: CodeNetworkInterfaceAddressDuplicate,
		},
		{
			name:      "a repeated DNS server is rejected",
			dnsServer: []string{"8.8.8.8", "8.8.4.4", "8.8.8.8"},
			wantError: CodeDNSServerDuplicate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state := newCustomNetworkState(map[string]zsettingsv2.CustomNetworkConfig{
				"worker": newCustomNetworkConfig(tt.addresses, tt.dnsServer),
			})

			assertResult(t, ValidateCustomNetworkConfigParameters(state), tt.wantError, "")
		})
	}
}

// Every duplicate has to be named: an operator fixing a long list should not have to re-apply once
// per duplicate to discover the next one.
func TestValidateCustomNetworkConfigsReportsEveryDuplicate(t *testing.T) {
	t.Parallel()

	state := newCustomNetworkState(map[string]zsettingsv2.CustomNetworkConfig{
		"worker": newCustomNetworkConfig(
			[]string{"192.168.1.10", "192.168.1.11", "192.168.1.10", "192.168.1.11", "192.168.1.10"},
			nil,
		),
	})

	violations := ValidateCustomNetworkConfigParameters(state).Errors()
	if len(violations) != 1 {
		t.Fatalf("violations = %d, want 1 naming both duplicates: %v", len(violations), violations)
	}

	for _, want := range []string{"192.168.1.10", "192.168.1.11"} {
		if !strings.Contains(violations[0].Message, want) {
			t.Errorf("message = %q, want it to name %s", violations[0].Message, want)
		}
	}
}

// Violations are keyed by code and path, so entries must be told apart by the NodeGroup name they
// report — otherwise a duplicate under the second key silently overwrites the one under the first.
func TestValidateCustomNetworkConfigsReportsEveryNodeGroup(t *testing.T) {
	t.Parallel()

	state := newCustomNetworkState(map[string]zsettingsv2.CustomNetworkConfig{
		"worker": newCustomNetworkConfig([]string{"192.168.1.10", "192.168.1.10"}, nil),
		"system": newCustomNetworkConfig([]string{"192.168.2.10", "192.168.2.10"}, nil),
	})

	violations := ValidateCustomNetworkConfigParameters(state).Errors()
	if len(violations) != 2 {
		t.Fatalf("violations = %d, want one per NodeGroup key: %v", len(violations), violations)
	}

	paths := make([]string, 0, len(violations))
	for _, violation := range violations {
		paths = append(paths, violation.Path)
	}

	for _, want := range []string{
		"ModuleConfig.spec.settings.nodes.parameters.customNetworkConfigs[worker].networkInterfaceAddresses",
		"ModuleConfig.spec.settings.nodes.parameters.customNetworkConfigs[system].networkInterfaceAddresses",
	} {
		if !slices.Contains(paths, want) {
			t.Errorf("paths = %v, want %s", paths, want)
		}
	}
}

// Addresses are picked by node index, so a list shorter than the replica count leaves the surplus
// nodes without a static configuration instead of failing the apply.
func TestValidateCustomNetworkConfigsCoverReplicas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		addresses   []string
		replicas    int
		nodeType    cpapi.NodeType
		wantError   string
		wantWarning string
	}{
		{
			name:      "an address per replica is accepted",
			addresses: []string{"192.168.1.10", "192.168.1.11", "192.168.1.12"},
			replicas:  3,
			nodeType:  cpapi.NodeTypeCloudPermanent,
		},
		{
			name:      "spare addresses are accepted",
			addresses: []string{"192.168.1.10", "192.168.1.11", "192.168.1.12"},
			replicas:  2,
			nodeType:  cpapi.NodeTypeCloudPermanent,
		},
		{
			name:      "fewer addresses than replicas are rejected",
			addresses: []string{"192.168.1.10", "192.168.1.11"},
			replicas:  3,
			nodeType:  cpapi.NodeTypeCloudPermanent,
			wantError: CodeNetworkInterfaceAddressesInsufficient,
		},
		{
			// The static configuration only reaches CloudPermanent nodes, and an autoscaled group
			// has no fixed node count to cover in the first place. The key is still reported so
			// the operator learns those nodes come up over DHCP.
			name:        "a CloudEphemeral NodeGroup is warned about, not counted",
			addresses:   []string{"192.168.1.10"},
			replicas:    5,
			nodeType:    cpapi.NodeTypeCloudEphemeral,
			wantWarning: CodeCustomNetworkConfigUnknownNodeGroup,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state := newCustomNetworkState(map[string]zsettingsv2.CustomNetworkConfig{
				"worker": newCustomNetworkConfig(tt.addresses, nil),
			})
			state.NodeGroups = []cpapi.NodeGroup{newNodeGroup("worker", tt.nodeType, tt.replicas)}

			assertResult(
				t,
				ValidateCustomNetworkConfigsCoverNodeGroupReplicas(state, true),
				tt.wantError,
				tt.wantWarning,
			)
		})
	}
}

// A NodeGroup with no entry of its own is not held to anything: its nodes get their addresses
// from DHCP, however many of them there are.
func TestValidateCustomNetworkConfigsCoverReplicasIgnoresUnlistedNodeGroups(t *testing.T) {
	t.Parallel()

	state := newCustomNetworkState(map[string]zsettingsv2.CustomNetworkConfig{
		"worker": newCustomNetworkConfig([]string{"192.168.1.10"}, nil),
	})
	state.NodeGroups = []cpapi.NodeGroup{
		newNodeGroup("worker", cpapi.NodeTypeCloudPermanent, 1),
		newNodeGroup("system", cpapi.NodeTypeCloudPermanent, 5),
	}

	assertResult(t, ValidateCustomNetworkConfigsCoverNodeGroupReplicas(state, true), "", "")
}

// A key matching no CloudPermanent NodeGroup is not a harmless typo: the operator believes those
// nodes carry a static configuration, and they come up on DHCP instead. The write itself is
// allowed — the NodeGroup the key describes may not have been created yet.
func TestValidateCustomNetworkConfigsWarnAboutUnknownKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		configKey   string
		nodeGroups  []cpapi.NodeGroup
		wantWarning string
	}{
		{
			name:       "a key naming an existing CloudPermanent NodeGroup is silent",
			configKey:  "worker",
			nodeGroups: []cpapi.NodeGroup{newNodeGroup("worker", cpapi.NodeTypeCloudPermanent, 1)},
		},
		{
			name:        "a key naming no NodeGroup warns",
			configKey:   "typo",
			nodeGroups:  []cpapi.NodeGroup{newNodeGroup("worker", cpapi.NodeTypeCloudPermanent, 1)},
			wantWarning: CodeCustomNetworkConfigUnknownNodeGroup,
		},
		{
			name:        "a key naming a CloudEphemeral NodeGroup warns",
			configKey:   "worker",
			nodeGroups:  []cpapi.NodeGroup{newNodeGroup("worker", cpapi.NodeTypeCloudEphemeral, 1)},
			wantWarning: CodeCustomNetworkConfigUnknownNodeGroup,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state := newCustomNetworkState(map[string]zsettingsv2.CustomNetworkConfig{
				tt.configKey: newCustomNetworkConfig([]string{"192.168.1.10"}, nil),
			})
			state.NodeGroups = tt.nodeGroups

			assertResult(t, ValidateCustomNetworkConfigsCoverNodeGroupReplicas(state, true), "", tt.wantWarning)
		})
	}
}

// The NodeGroup surface sees a single NodeGroup, so a key naming a sibling must not be read as one
// naming nothing: every update would otherwise warn about the configurations of the other groups.
func TestValidateCustomNetworkConfigsDoNotWarnWithoutAllNodeGroups(t *testing.T) {
	t.Parallel()

	state := newCustomNetworkState(map[string]zsettingsv2.CustomNetworkConfig{
		"master": newCustomNetworkConfig([]string{"192.168.1.10"}, nil),
	})
	state.NodeGroups = []cpapi.NodeGroup{newNodeGroup("worker", cpapi.NodeTypeCloudPermanent, 1)}

	assertResult(t, ValidateCustomNetworkConfigsCoverNodeGroupReplicas(state, false), "", "")

	// The same state is warned about where the sibling names are supposed to be known.
	warnings := ValidateCustomNetworkConfigsCoverNodeGroupReplicas(state, true).Warnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0].Message, "master") {
		t.Errorf("warnings = %v, want one about the unloaded sibling", warnings)
	}
}

// The rules run on surfaces where the ModuleConfig has not been created yet, or carries no
// settings. That is the ModuleConfig rule's business to report, not theirs.
func TestValidateCustomNetworkConfigsTolerantOfMissingState(t *testing.T) {
	t.Parallel()

	states := map[string]*State{
		"nil state":       nil,
		"no ModuleConfig": {},
		"no settings":     {ModuleConfig: &cpapi.ModuleConfig[*zsettingsv2.ModuleConfigSettings]{}},
		"no configs":      newCustomNetworkState(nil),
	}

	rules := map[string]func(*State) cpvalapi.Result{
		"ValidateCustomNetworkConfigParameters": ValidateCustomNetworkConfigParameters,
		"ValidateCustomNetworkConfigsCoverNodeGroupReplicas": func(state *State) cpvalapi.Result {
			return ValidateCustomNetworkConfigsCoverNodeGroupReplicas(state, true)
		},
	}

	for stateName, state := range states {
		for ruleName, rule := range rules {
			t.Run(stateName+"/"+ruleName, func(t *testing.T) {
				t.Parallel()

				if result := rule(state); result.HasErrors() {
					t.Errorf("%s() = %v, want silence", ruleName, result.Error())
				}
			})
		}
	}
}

// The legacy configuration carries the same static network configuration the settings map does, and
// the migration copies it verbatim. A duplicate the settings rule rejects must not slip through the
// only surface a legacy cluster has.
func TestValidateLegacyCustomNetworkConfigParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pcc       *zpccv1.ZvirtProviderClusterConfiguration
		wantError string
	}{
		{
			name: "distinct values are accepted",
			pcc: legacyPCC(2,
				newLegacyCustomNetworkConfig([]string{"192.168.1.10", "192.168.1.11"}, "8.8.8.8 8.8.4.4"),
				legacyNodeGroup("worker", 1, newLegacyCustomNetworkConfig([]string{"192.168.2.10"}, "1.1.1.1")),
			),
		},
		{
			name: "absent configurations are accepted",
			pcc:  legacyPCC(2, nil, legacyNodeGroup("worker", 1, nil)),
		},
		{
			name:      "a repeated master address is rejected",
			pcc:       legacyPCC(2, newLegacyCustomNetworkConfig([]string{"192.168.1.10", "192.168.1.10"}, "")),
			wantError: CodeNetworkInterfaceAddressDuplicate,
		},
		{
			name: "a repeated node group DNS server is rejected",
			pcc: legacyPCC(1, nil, legacyNodeGroup("worker", 1,
				newLegacyCustomNetworkConfig([]string{"192.168.2.10"}, "8.8.8.8 8.8.4.4 8.8.8.8"),
			)),
			wantError: CodeDNSServerDuplicate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertResult(t, ValidateLegacyCustomNetworkConfigParameters(tt.pcc), tt.wantError, "")
		})
	}

	if result := ValidateLegacyCustomNetworkConfigParameters(nil); result.HasErrors() || len(result.Warnings()) != 0 {
		t.Errorf("ValidateLegacyCustomNetworkConfigParameters(nil) = %v, want silence", result.Error())
	}
}

// Violations are keyed by code and path, so the master and each node group have to be reported under
// paths of their own — a shared prefix would collapse two broken configurations into one.
func TestValidateLegacyCustomNetworkConfigParametersReportEveryNodeGroup(t *testing.T) {
	t.Parallel()

	pcc := legacyPCC(2,
		newLegacyCustomNetworkConfig([]string{"192.168.1.10", "192.168.1.10"}, ""),
		legacyNodeGroup("worker", 1, newLegacyCustomNetworkConfig([]string{"192.168.2.10", "192.168.2.10"}, "")),
	)

	violations := ValidateLegacyCustomNetworkConfigParameters(pcc).Errors()
	if len(violations) != 2 {
		t.Fatalf("violations = %d, want one per node group: %v", len(violations), violations)
	}

	paths := make([]string, 0, len(violations))
	for _, violation := range violations {
		paths = append(paths, violation.Path)
	}

	for _, want := range []string{
		"ProviderClusterConfiguration.masterNodeGroup.instanceClass.customNetworkConfig.networkInterfaceAddress",
		"ProviderClusterConfiguration.nodeGroups[worker].instanceClass.customNetworkConfig.networkInterfaceAddress",
	} {
		if !slices.Contains(paths, want) {
			t.Errorf("paths = %v, want %s", paths, want)
		}
	}
}

// The migration projects the legacy replica count onto both bounds of the NodeGroup and hands the
// addresses out by node index, so a short list leaves the surplus nodes on DHCP — the same failure
// the settings rule guards against after the migration.
func TestValidateLegacyCustomNetworkConfigsCoverNodeGroupReplicas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pcc       *zpccv1.ZvirtProviderClusterConfiguration
		wantError string
	}{
		{
			name: "an address per master replica is accepted",
			pcc: legacyPCC(2,
				newLegacyCustomNetworkConfig([]string{"192.168.1.10", "192.168.1.11"}, ""),
			),
		},
		{
			name: "spare master addresses are accepted",
			pcc: legacyPCC(2,
				newLegacyCustomNetworkConfig([]string{"192.168.1.10", "192.168.1.11", "192.168.1.12"}, ""),
			),
		},
		{
			name: "fewer master addresses than replicas are rejected",
			pcc: legacyPCC(3,
				newLegacyCustomNetworkConfig([]string{"192.168.1.10", "192.168.1.11"}, ""),
			),
			wantError: CodeNetworkInterfaceAddressesInsufficient,
		},
		{
			name: "fewer node group addresses than its replicas are rejected",
			pcc: legacyPCC(1, nil,
				legacyNodeGroup("worker", 2, newLegacyCustomNetworkConfig([]string{"192.168.2.10"}, "")),
			),
			wantError: CodeNetworkInterfaceAddressesInsufficient,
		},
		{
			name: "a node group without a configuration is not held to its replicas",
			pcc:  legacyPCC(1, nil, legacyNodeGroup("worker", 5, nil)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertResult(t, ValidateLegacyCustomNetworkConfigsCoverNodeGroupReplicas(tt.pcc), tt.wantError, "")
		})
	}

	if result := ValidateLegacyCustomNetworkConfigsCoverNodeGroupReplicas(nil); result.HasErrors() || len(result.Warnings()) != 0 {
		t.Errorf("ValidateLegacyCustomNetworkConfigsCoverNodeGroupReplicas(nil) = %v, want silence", result.Error())
	}
}

// The master is reported by the name the migration gives it, and under the masterNodeGroup path:
// without either, the operator has to guess which list the error is about.
func TestValidateLegacyCustomNetworkConfigsCoverReplicasNamesTheMaster(t *testing.T) {
	t.Parallel()

	pcc := legacyPCC(3, newLegacyCustomNetworkConfig([]string{"192.168.1.10"}, ""))

	violations := ValidateLegacyCustomNetworkConfigsCoverNodeGroupReplicas(pcc).Errors()
	if len(violations) != 1 {
		t.Fatalf("violations = %d, want 1: %v", len(violations), violations)
	}

	if want := "ProviderClusterConfiguration.masterNodeGroup.instanceClass.customNetworkConfig.networkInterfaceAddress"; violations[0].Path != want {
		t.Errorf("path = %q, want %q", violations[0].Path, want)
	}

	if !strings.Contains(violations[0].Message, `NodeGroup "master"`) {
		t.Errorf("message = %q, want it to name the master NodeGroup", violations[0].Message)
	}
}

func newCustomNetworkState(configs map[string]zsettingsv2.CustomNetworkConfig) *State {
	return &State{
		ModuleConfig: &cpapi.ModuleConfig[*zsettingsv2.ModuleConfigSettings]{
			Spec: cpapi.ModuleConfigSpec[*zsettingsv2.ModuleConfigSettings]{
				Settings: &zsettingsv2.ModuleConfigSettings{
					Nodes: zsettingsv2.Nodes{
						Parameters: zsettingsv2.NodesParameters{
							CustomNetworkConfigs: configs,
						},
					},
				},
			},
		},
	}
}

func newCustomNetworkConfig(addresses, dnsServers []string) zsettingsv2.CustomNetworkConfig {
	return zsettingsv2.CustomNetworkConfig{
		NetworkInterfaceName:      "enp1s0",
		NetworkInterfaceAddresses: addresses,
		NetworkInterfaceNetmask:   "255.255.255.0",
		NetworkInterfaceGateway:   "192.168.1.1",
		DNSServers:                dnsServers,
	}
}

func newNodeGroup(name string, nodeType cpapi.NodeType, replicas int) cpapi.NodeGroup {
	return cpapi.NodeGroup{
		ObjectMeta: cpapi.ObjectMeta{Name: name},
		Spec: cpapi.NodeGroupSpec{
			NodeType: nodeType,
			CloudInstances: &cpapi.CloudInstances{
				ClassReference: &cpapi.ClassReference{
					Kind: zicv1.ZvirtInstanceClassKind,
					Name: name,
				},
				// The migration projects replicas onto both bounds.
				MinPerZone: replicas,
				MaxPerZone: replicas,
			},
		},
	}
}

// legacyPCC builds a legacy configuration whose master and additional node groups declare the given
// static network configurations. Replica counts stay with the caller so each test sees only the
// mismatch it is about.
func legacyPCC(masterReplicas int, masterConfig *zpccv1.ZvirtNetworkConfig, nodeGroups ...zpccv1.ZvirtStaticNodeGroup) *zpccv1.ZvirtProviderClusterConfiguration {
	return &zpccv1.ZvirtProviderClusterConfiguration{
		MasterNodeGroup: zpccv1.ZvirtMasterNodeGroup{
			Replicas: masterReplicas,
			InstanceClass: zpccv1.ZvirtMasterInstanceClass{
				ZvirtInstanceClass: zpccv1.ZvirtInstanceClass{
					CustomNetworkConfig: masterConfig,
				},
			},
		},
		NodeGroups: nodeGroups,
	}
}

func legacyNodeGroup(name string, replicas int, config *zpccv1.ZvirtNetworkConfig) zpccv1.ZvirtStaticNodeGroup {
	return zpccv1.ZvirtStaticNodeGroup{
		Name:     name,
		Replicas: replicas,
		InstanceClass: zpccv1.ZvirtStaticInstanceClass{
			ZvirtInstanceClass: zpccv1.ZvirtInstanceClass{
				CustomNetworkConfig: config,
			},
		},
	}
}

func newLegacyCustomNetworkConfig(addresses []string, dnsServers string) *zpccv1.ZvirtNetworkConfig {
	return &zpccv1.ZvirtNetworkConfig{
		NetworkInterfaceName:    "enp1s0",
		NetworkInterfaceAddress: addresses,
		NetworkInterfaceNetmask: "255.255.255.0",
		NetworkInterfaceGateway: "192.168.1.1",
		DNSServers:              dnsServers,
	}
}

// A nil state reaches the rule whenever the ModuleConfig has not been created yet. That is not a
// violation on its own — the ModuleConfig rule reports it — so the rule must stay silent.
func TestValidateProviderConnectionTolerantOfMissingConfig(t *testing.T) {
	t.Parallel()

	for name, state := range map[string]*State{
		"nil state":         nil,
		"no ModuleConfig":   {},
		"no settings":       {ModuleConfig: &cpapi.ModuleConfig[*zsettingsv2.ModuleConfigSettings]{}},
		"nil legacy config": {},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if result := ValidateProviderConnection(state); result.HasErrors() || len(result.Warnings()) != 0 {
				t.Errorf("ValidateProviderConnection() = %v, want silence", result.Error())
			}
		})
	}

	if result := ValidateLegacyProviderConnection(nil); result.HasErrors() {
		t.Errorf("ValidateLegacyProviderConnection(nil) = %v, want silence", result.Error())
	}
}
