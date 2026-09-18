/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import (
	"testing"

	deckhousev1alpha1 "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	zicv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal/api/instanceclass/v1"
	zpccv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal/api/pcc/v1"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	corev1 "k8s.io/api/core/v1"
)

func testPCC() zpccv1.ZvirtProviderClusterConfiguration {
	return zpccv1.ZvirtProviderClusterConfiguration{
		APIVersion:   "deckhouse.io/v1",
		Kind:         "ZvirtClusterConfiguration",
		Layout:       "Standard",
		SSHPublicKey: "ssh-rsa AAAA",
		ClusterID:    "b46372e7-0d52-40c7-9bbf-fda31e187088",
		MasterNodeGroup: zpccv1.ZvirtMasterNodeGroup{
			Replicas: 3,
			InstanceClass: zpccv1.ZvirtMasterInstanceClass{
				ZvirtInstanceClass: zpccv1.ZvirtInstanceClass{
					NumCPUs:         4,
					Memory:          8192,
					Template:        "debian-bookworm",
					VNICProfileID:   "49bb4594-0cd4-4eb7-8288-8594eafd5a86",
					StorageDomainID: "c4bf82a5-b803-40c3-9f6c-b9398378f424",
				},
			},
		},
		Provider: zpccv1.ZvirtProvider{
			Server:   "https://zvirt.example.com/ovirt-engine/api",
			Username: "admin@internal",
			Password: "s3cret",
			Insecure: true,
		},
	}
}

func TestBuildModuleConfigSettingsV2(t *testing.T) {
	settings := BuildModuleConfigSettingsV2(testPCC())

	if got, want := settings.Provider.Parameters.Server, "https://zvirt.example.com/ovirt-engine/api"; got != want {
		t.Errorf("server = %q, want %q", got, want)
	}
	if got, want := settings.Provider.Parameters.ClusterID, "b46372e7-0d52-40c7-9bbf-fda31e187088"; got != want {
		t.Errorf("clusterID = %q, want %q", got, want)
	}
	if !settings.Provider.Parameters.Insecure {
		t.Error("insecure must be carried over from the legacy configuration")
	}
	if got, want := settings.Nodes.Parameters.SSHPublicKey, "ssh-rsa AAAA"; got != want {
		t.Errorf("sshPublicKey = %q, want %q", got, want)
	}
	if got, want := settings.Nodes.Parameters.Layout, "Standard"; got != want {
		t.Errorf("layout = %q, want %q", got, want)
	}
}

// An empty legacy configuration must still yield settings that satisfy the schema, so that a
// cluster without one (a hybrid installation) keeps rendering.
func TestBuildModuleConfigSettingsV2UsesPlaceholders(t *testing.T) {
	settings := BuildModuleConfigSettingsV2(zpccv1.ZvirtProviderClusterConfiguration{})

	if got := settings.Provider.Parameters.Server; got != PlaceholderServer {
		t.Errorf("server = %q, want the placeholder", got)
	}
	if got := settings.Provider.Parameters.ClusterID; got != PlaceholderClusterID {
		t.Errorf("clusterID = %q, want the placeholder", got)
	}
	if got := settings.Nodes.Parameters.SSHPublicKey; got != PlaceholderSSHPublicKey {
		t.Errorf("sshPublicKey = %q, want the placeholder", got)
	}
	if got := settings.Nodes.Parameters.Layout; got != DefaultLayout {
		t.Errorf("layout = %q, want %q", got, DefaultLayout)
	}
}

// The credentials must leave the ModuleConfig and land in the managed Secret instead.
func TestBuildCredentialSecret(t *testing.T) {
	secret := BuildCredentialSecret(testPCC())

	if got, want := string(secret.Type), cpapi.CredentialsSecretType; got != want {
		t.Errorf("type = %v, want %v", got, want)
	}
	if got, want := secret.StringData[cpapi.CredentialSecretAuthSchemeKey], string(cpapi.AuthSchemeUserPassword); got != want {
		t.Errorf("authScheme = %v, want %v", got, want)
	}
	if got, want := secret.StringData[cpapi.CredentialSecretIdentityKey], "admin@internal"; got != want {
		t.Errorf("identity = %v, want %v", got, want)
	}
	if got, want := secret.StringData[cpapi.CredentialSecretSecretKey], "s3cret"; got != want {
		t.Errorf("secret = %v, want %v", got, want)
	}
}

// Disk sizes must always be explicit, and they must use the terraform defaults rather than the
// ones the CRD documents — otherwise migrating a cluster that never set them replaces its disks.
func TestMapPCCInstanceClassToSpecFillsTerraformDefaults(t *testing.T) {
	spec := MapPCCInstanceClassToSpec(testPCC().MasterNodeGroup.InstanceClass.ZvirtInstanceClass, nil, true)

	if spec.RootDiskSizeGb != defaultRootDiskSizeGb {
		t.Errorf("rootDiskSizeGb = %v, want %d", spec.RootDiskSizeGb, defaultRootDiskSizeGb)
	}
	if spec.EtcdDiskSizeGb == nil || *spec.EtcdDiskSizeGb != defaultEtcdDiskSizeGb {
		t.Errorf("etcdDiskSizeGb = %v, want %d", spec.EtcdDiskSizeGb, defaultEtcdDiskSizeGb)
	}
}

// A dedicated etcd disk belongs to the master node group alone.
func TestMapPCCInstanceClassToSpecOmitsEtcdDiskForWorkers(t *testing.T) {
	spec := MapPCCInstanceClassToSpec(testPCC().MasterNodeGroup.InstanceClass.ZvirtInstanceClass, nil, false)

	if spec.EtcdDiskSizeGb != nil {
		t.Errorf("etcdDiskSizeGb = %v, want absent for a non-master node group", *spec.EtcdDiskSizeGb)
	}
}

func TestMapPCCInstanceClassToSpecKeepsExplicitSizes(t *testing.T) {
	root, etcd := 120, 25
	instanceClass := testPCC().MasterNodeGroup.InstanceClass.ZvirtInstanceClass
	instanceClass.RootDiskSizeGb = &root

	spec := MapPCCInstanceClassToSpec(instanceClass, &etcd, true)

	if spec.RootDiskSizeGb != root {
		t.Errorf("rootDiskSizeGb = %v, want %d", spec.RootDiskSizeGb, root)
	}
	if spec.EtcdDiskSizeGb == nil || *spec.EtcdDiskSizeGb != etcd {
		t.Errorf("etcdDiskSizeGb = %v, want %d", spec.EtcdDiskSizeGb, etcd)
	}
}

// The legacy configuration attaches the static network configuration to a node group's
// InstanceClass and stores DNS servers as one space-separated string; the settings key the
// configuration by NodeGroup name and take a DNS list.
func TestBuildModuleConfigSettingsV2CustomNetworkConfigs(t *testing.T) {
	pcc := testPCC()
	pcc.MasterNodeGroup.InstanceClass.CustomNetworkConfig = &zpccv1.ZvirtNetworkConfig{
		NetworkInterfaceName:    "enp1s0",
		NetworkInterfaceAddress: []string{"192.168.1.10", "192.168.1.11"},
		NetworkInterfaceNetmask: "255.255.255.0",
		NetworkInterfaceGateway: "192.168.1.1",
		DNSServers:              "8.8.8.8 8.8.4.4",
	}

	configs := BuildModuleConfigSettingsV2(pcc).Nodes.Parameters.CustomNetworkConfigs

	config, ok := configs[masterNodeGroupName]
	if !ok {
		t.Fatalf("customNetworkConfigs must be keyed by NodeGroup name, got %+v", configs)
	}

	if config.NetworkInterfaceName != "enp1s0" {
		t.Fatalf("customNetworkConfig must be carried over, got %+v", config)
	}

	// The legacy field is singular and the settings one is plural, but the values are indexed by
	// node index on both sides — the terraform projection asserts the same thing, and the two must
	// not drift apart.
	assertStrings(t, "networkInterfaceAddresses", config.NetworkInterfaceAddresses, []string{"192.168.1.10", "192.168.1.11"})
	assertStrings(t, "dnsServers", config.DNSServers, []string{"8.8.8.8", "8.8.4.4"})
}

// A node group that declares no static configuration must not gain an empty entry: the hook omits
// the key entirely, and the terraform projection has to match that or terraform sees drift.
func TestBuildModuleConfigSettingsV2OmitsAbsentCustomNetworkConfigs(t *testing.T) {
	if configs := BuildModuleConfigSettingsV2(testPCC()).Nodes.Parameters.CustomNetworkConfigs; configs != nil {
		t.Errorf("customNetworkConfigs = %+v, want nil", configs)
	}
}

func assertStrings(t *testing.T, name string, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s[%d] = %q, want %q", name, i, got[i], want[i])
		}
	}
}

func TestBuildMigrationResources(t *testing.T) {
	pcc := testPCC()
	pcc.NodeGroups = []zpccv1.ZvirtStaticNodeGroup{
		{
			Name:     "worker",
			Replicas: 2,
			InstanceClass: zpccv1.ZvirtStaticInstanceClass{
				ZvirtInstanceClass: zpccv1.ZvirtInstanceClass{
					NumCPUs:       4,
					Memory:        8192,
					Template:      "debian-bookworm",
					VNICProfileID: "49bb4594-0cd4-4eb7-8288-8594eafd5a86",
				},
			},
		},
	}

	resources, err := BuildMigrationResources(pcc, false)
	if err != nil {
		t.Fatalf("BuildMigrationResources: %v", err)
	}

	// Secret, ModuleConfig, and an InstanceClass plus NodeGroup pair per node group.
	if got, want := len(resources), 6; got != want {
		t.Fatalf("resource count = %d, want %d", got, want)
	}

	// The bundle is built from typed manifests, so the check is on the types themselves rather
	// than on a "kind" string.
	var secrets, moduleConfigs, instanceClasses, nodeGroups int
	for _, resource := range resources {
		switch resource.(type) {
		case corev1.Secret:
			secrets++
		case deckhousev1alpha1.ModuleConfig:
			moduleConfigs++
		case zicv1.ZvirtInstanceClass:
			instanceClasses++
		case cpapi.NodeGroup:
			nodeGroups++
		default:
			t.Fatalf("unexpected resource type %T in the bundle", resource)
		}
	}

	for _, tt := range []struct {
		name string
		got  int
		want int
	}{
		{"Secret", secrets, 1},
		{"ModuleConfig", moduleConfigs, 1},
		{"ZvirtInstanceClass", instanceClasses, 2},
		{"NodeGroup", nodeGroups, 2},
	} {
		if tt.got != tt.want {
			t.Errorf("%s count = %d, want %d", tt.name, tt.got, tt.want)
		}
	}
}

// A hybrid cluster manages its nodes itself, so the bundle must not generate NodeGroups for it.
func TestBuildMigrationResourcesForHybridClusterSkipsNodeGroups(t *testing.T) {
	resources, err := BuildMigrationResources(testPCC(), true)
	if err != nil {
		t.Fatalf("BuildMigrationResources: %v", err)
	}

	if got, want := len(resources), 2; got != want {
		t.Fatalf("resource count = %d, want %d (Secret and ModuleConfig only)", got, want)
	}
}

func TestBuildMigrationResourcesRejectsIncompleteInstanceClass(t *testing.T) {
	pcc := testPCC()
	pcc.MasterNodeGroup.InstanceClass.Template = ""

	if _, err := BuildMigrationResources(pcc, false); err == nil {
		t.Fatal("an instanceClass without a template must be rejected")
	}
}

func TestIsHybridCluster(t *testing.T) {
	tests := []struct {
		name       string
		nodeGroups []NodeGroupFilterResult
		want       bool
	}{
		{name: "no node groups", nodeGroups: nil, want: false},
		{
			name:       "cloud master",
			nodeGroups: []NodeGroupFilterResult{{Name: "master", NodeType: string(cpapi.NodeTypeCloudPermanent)}},
			want:       false,
		},
		{
			name:       "static master",
			nodeGroups: []NodeGroupFilterResult{{Name: "master", NodeType: string(cpapi.NodeTypeStatic)}},
			want:       true,
		},
		{
			name:       "static worker only",
			nodeGroups: []NodeGroupFilterResult{{Name: "worker", NodeType: string(cpapi.NodeTypeStatic)}},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHybridCluster(tt.nodeGroups); got != tt.want {
				t.Errorf("IsHybridCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}

// The InstanceClass name the bundle generates has to match the one the completeness check looks
// for, or the migration would never be considered done.
func TestBundleInstanceClassNamesMatchTheCompletenessCheck(t *testing.T) {
	nodeGroup, instanceClass, err := BuildNodeGroupAndInstanceClassResources(
		"worker", 2, testPCC().MasterNodeGroup.InstanceClass.ZvirtInstanceClass, nil, nil, false,
	)
	if err != nil {
		t.Fatalf("BuildNodeGroupAndInstanceClassResources: %v", err)
	}

	want := cpapi.BuildInstanceClassName("worker")
	if got := instanceClass.Name; got != want {
		t.Errorf("instance class name = %v, want %v", got, want)
	}

	// The NodeGroup has to point at the very class the bundle emits.
	if got := nodeGroup.Spec.CloudInstances.ClassReference.Name; got != want {
		t.Errorf("classReference.name = %v, want %v", got, want)
	}
	if got := nodeGroup.Spec.CloudInstances.ClassReference.Kind; got != zicv1.ZvirtInstanceClassKind {
		t.Errorf("classReference.kind = %v, want %v", got, zicv1.ZvirtInstanceClassKind)
	}
}

// candi/terraform-modules/migration projects the very same legacy configuration onto the very same
// resources, and its test suite pins the resulting names as literals. Terraform reads the cluster
// through those names, so the two projections must agree on them: were only the hook to change, the
// infrastructure would keep pointing at an InstanceClass nobody writes any more.
//
// Keep these literals in sync with candi/terraform-modules/migration/migration.tftest.hcl.
func TestInstanceClassNamesMatchTheTerraformProjection(t *testing.T) {
	for nodeGroupName, want := range map[string]string{
		"master": "master-fc613b4dfd67",
		"worker": "worker-87eba76e7f31",
	} {
		if got := cpapi.BuildInstanceClassName(nodeGroupName); got != want {
			t.Errorf("BuildInstanceClassName(%q) = %v, want %v (terraform pins this literal)", nodeGroupName, got, want)
		}
	}
}
