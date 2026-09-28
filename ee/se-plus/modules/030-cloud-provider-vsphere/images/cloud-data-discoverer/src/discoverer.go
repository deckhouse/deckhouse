/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/cns"
	"github.com/vmware/govmomi/cns/types"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"

	v1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency/vsphere"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	DiscoveryDataKind    = "VsphereCloudDiscoveryData"
	DiscoveryDataVersion = "deckhouse.io/v1"
)

type Discoverer struct {
	logger               *log.Logger
	clusterUUID          string
	csiCompatibilityFlag string
	govmomiClient        *govmomi.Client
	cnsClient            *cns.Client
	vsphereClient        vsphere.Client
	vmFolderPath         string

	// host + insecure + caBundle are kept for the leaf-cert fingerprint computation on
	// every discovery cycle. The govmomi client already round-trips SOAP over the same
	// TLS session, but its public API does not surface the peer certificate — a separate
	// tls.Dial to host:443, using the same trust settings, is the cheapest way to read
	// the leaf cert without a govmomi patch.
	host         string
	insecureFlag bool
	caCertPool   *x509.CertPool
}

func NewDiscoverer(logger *log.Logger) *Discoverer {
	clusterUUID := os.Getenv("CLUSTER_UUID")
	if clusterUUID == "" {
		logger.Fatal("Cannot get CLUSTER_UUID env")
	}
	csiCompatibilityFlag := os.Getenv("CSI_COMPATIBILITY_FLAG")
	if csiCompatibilityFlag == "" {
		logger.Fatal("Cannot get CSI_COMPATIBILITY_FLAG env")
	}

	host := os.Getenv("GOVMOMI_HOST")
	if host == "" {
		logger.Fatal("Cannot get GOVMOMI_HOST env")
	}
	username := os.Getenv("GOVMOMI_USERNAME")
	if username == "" {
		logger.Fatal("Cannot get GOVMOMI_USERNAME env")
	}
	password := os.Getenv("GOVMOMI_PASSWORD")
	if password == "" {
		logger.Fatal("Cannot get GOVMOMI_PASSWORD env")
	}

	insecure := os.Getenv("GOVMOMI_INSECURE")
	if insecure == "" {
		logger.Fatal("Cannot get GOVMOMI_INSECURE env")
	}
	insecureFlag, err := strconv.ParseBool(insecure)
	if err != nil {
		logger.Fatal("Failed to parse GOVMOMI_INSECURE env as bool", "error", err)
	}

	caBundle := os.Getenv("GOVMOMI_CA_BUNDLE")

	parsedURL, err := url.Parse(fmt.Sprintf("https://%s:%s@%s/sdk", url.PathEscape(strings.TrimSpace(username)), url.PathEscape(strings.TrimSpace(password)), url.PathEscape(strings.TrimSpace(host))))
	if err != nil {
		logger.Fatal("Failed to build connection url", "error", err)
	}

	soapClient := soap.NewClient(parsedURL, insecureFlag)
	if err := setCABundleIfNeed(logger, soapClient, insecureFlag, caBundle); err != nil {
		logger.Fatal("Failed to set CA bundle", "error", err)
	}

	vimClient, err := vim25.NewClient(context.TODO(), soapClient)
	if err != nil {
		logger.Fatal("Failed to create vimClient client", "error", err)
	}

	if !vimClient.IsVC() {
		logger.Fatal("Created client not connected to vCenter")
	}

	// vSphere connection is timed out after 30 minutes of inactivity.
	vimClient.RoundTripper = session.KeepAlive(vimClient.RoundTripper, 10*time.Minute)
	govmomiClient := &govmomi.Client{
		Client:         vimClient,
		SessionManager: session.NewManager(vimClient),
	}

	err = govmomiClient.SessionManager.Login(context.TODO(), parsedURL.User)
	if err != nil {
		logger.Fatal("Failed to login with provided credentials", "error", err)
	}

	cnsClient, err := cns.NewClient(context.TODO(), govmomiClient.Client)
	if err != nil {
		logger.Fatal("Failed to create CNS client", "error", err)
	}

	region := os.Getenv("REGION")
	if region == "" {
		logger.Fatal("Cannot get REGION env")
	}

	zonesRaw := os.Getenv("ZONES")
	zones := strings.Split(zonesRaw, ",")

	regionTagCategory := os.Getenv("REGION_TAG_CATEGORY")
	if regionTagCategory == "" {
		logger.Fatal("Cannot get REGION_TAG_CATEGORY env")
	}

	zoneTagCategory := os.Getenv("ZONE_TAG_CATEGORY")
	if zoneTagCategory == "" {
		logger.Fatal("Cannot get ZONE_TAG_CATEGORY env")
	}

	vmFolderPath := os.Getenv("VM_FOLDER_PATH")
	if vmFolderPath == "" {
		logger.Fatal("Cannot get VM_FOLDER_PATH env")
	}

	config := &vsphere.ProviderClusterConfiguration{
		Region:            region,
		Zones:             zones,
		RegionTagCategory: regionTagCategory,
		ZoneTagCategory:   zoneTagCategory,
		Provider: vsphere.Provider{
			Server:   host,
			Username: username,
			Password: password,
			Insecure: insecureFlag,
			CABundle: caBundle,
		},
	}

	vc, err := vsphere.NewClient(config)
	if err != nil {
		logger.Fatal("Failed to create vSphere client", "error", err)
	}

	// Parse caBundle once, share the same pool with the leaf-cert dialer used by
	// discoverThumbprint on every DiscoveryData cycle. Empty caBundle + insecure=false
	// means "use system trust store" (nil pool → tls.Dial default).
	var caCertPool *x509.CertPool
	if caBundle != "" {
		caCertPool = x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM([]byte(caBundle)) {
			logger.Fatal("Failed to parse GOVMOMI_CA_BUNDLE as PEM")
		}
	}

	return &Discoverer{
		logger:               logger,
		clusterUUID:          clusterUUID,
		csiCompatibilityFlag: csiCompatibilityFlag,
		govmomiClient:        govmomiClient,
		cnsClient:            cnsClient,
		vsphereClient:        vc,
		vmFolderPath:         vmFolderPath,
		host:                 host,
		insecureFlag:         insecureFlag,
		caCertPool:           caCertPool,
	}
}

func (d *Discoverer) CheckCloudConditions(ctx context.Context) ([]v1alpha1.CloudCondition, error) {
	return nil, nil
}

// NotImplemented
func (d *Discoverer) InstanceTypes(ctx context.Context) ([]v1alpha1.InstanceType, error) {
	return nil, nil
}

func (d *Discoverer) DiscoveryData(ctx context.Context, cloudProviderDiscoveryData []byte) ([]byte, error) {
	discoveryData := new(v1.VsphereCloudDiscoveryData)
	if len(cloudProviderDiscoveryData) > 0 {
		err := json.Unmarshal(cloudProviderDiscoveryData, &discoveryData)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal cloud provider discovery data: %v", err)
		}
	}

	err := d.vsphereClient.RefreshClient()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh vSphere client: %v", err)
	}

	zonesDatastores, err := d.vsphereClient.GetZonesDatastores()
	if err != nil {
		return nil, fmt.Errorf("error on GetZonesDatastores: %v", err)
	}

	storagePolicies, err := d.vsphereClient.ListPolicies()
	if err != nil {
		return nil, fmt.Errorf("failed to list Storage Policies: %v", err)
	}

	discoveryData.Kind = DiscoveryDataKind
	discoveryData.APIVersion = DiscoveryDataVersion
	discoveryData.Datacenter = zonesDatastores.Datacenter
	discoveryData.Zones = mergeZones(discoveryData.Zones, zonesDatastores.Zones)
	discoveryData.Datastores = mergeDatastores(discoveryData.Datastores, zonesDatastores.ZonedDataStores)
	discoveryData.ZoneComputeClusterPaths = zonesDatastores.ZoneComputeClusterPaths
	discoveryData.VMFolderPath = d.vmFolderPath

	// Ensure the "deckhouse-cluster-name" tag exists for this cluster and publish its URN,
	// so capi/template.yaml can render VSphereMachineTemplate.spec.tagIDs and CAPV attaches
	// the tag on clone. This is a partial parity with MCM: MCM also attached a
	// "deckhouse-node-role/<ng>-<zone>" tag, which the CAPI path does not yet reproduce —
	// see the module USAGE doc for the rationale.
	//
	// Any failure here is logged and swallowed: the rest of discovery data is independent
	// of tagging, and losing the tag on new VMs is a UI regression, not a functional one.
	// The previous TagURNs value in cloudProviderDiscoveryData is preserved on error via
	// json.Unmarshal above — an operator that once had the tag will keep it until the next
	// successful ensure.
	if urn, err := d.vsphereClient.EnsureClusterTagURN(ctx, d.clusterUUID); err != nil {
		d.logger.Warn("Failed to ensure cluster tag URN, VMs cloned in the meantime will not carry the tag", "error", err)
	} else {
		discoveryData.TagURNs = []string{urn}
	}

	// Publish the vCenter leaf-cert SHA-1 fingerprint. capi/cluster.yaml renders it into
	// VSphereCluster.spec.thumbprint, which CAPV's session code uses to pin the leaf on
	// every reconcile. Best-effort: on transport error the previously published thumbprint
	// is retained (via the unmarshal at the top of DiscoveryData) — dropping the last-good
	// value would flip CAPV to insecure for the whole cluster on any temporary DNS blip.
	if tp, err := d.discoverThumbprint(ctx); err != nil {
		d.logger.Warn("Failed to fetch vCenter leaf certificate for thumbprint", "error", err)
	} else {
		discoveryData.Thumbprint = tp
	}

	for i := range storagePolicies {
		discoveryData.StoragePolicies = append(discoveryData.StoragePolicies, v1.VsphereStoragePolicy{
			Name: storagePolicies[i].Name,
			ID:   storagePolicies[i].ID,
		})
	}

	discoveryDataJSON, err := json.Marshal(discoveryData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery data: %w", err)
	}

	d.logger.Debug("discovery data:", "discovery_data_json", discoveryDataJSON)
	return discoveryDataJSON, nil
}

func (d *Discoverer) DisksMeta(ctx context.Context) ([]v1alpha1.DiskMeta, error) {
	if d.csiCompatibilityFlag != "none" {
		d.logger.Warn("Skipping orphaned disks discovery: \"legacy\" CSI driver in-use")
		return []v1alpha1.DiskMeta{}, nil
	}

	disks, err := d.getDisksCreatedByCSIDriver(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get disks: %v", err)
	}

	disksMeta := make([]v1alpha1.DiskMeta, 0, len(disks))

	for _, disk := range disks {
		disksMeta = append(disksMeta, v1alpha1.DiskMeta{ID: disk.VolumeId.Id, Name: disk.Name})
	}

	return disksMeta, nil
}

func (d *Discoverer) getDisksCreatedByCSIDriver(ctx context.Context) ([]types.CnsVolume, error) {
	diskList, err := d.cnsClient.QueryVolume(ctx, types.CnsQueryFilter{ContainerClusterIds: []string{d.clusterUUID}})
	if err != nil {
		return nil, fmt.Errorf("failed to list disks: %v", err)
	}

	return diskList.Volumes, nil
}

func mergeZones(discoveredZones, newZones []string) []string {
	zones := make([]string, 0, len(discoveredZones)+len(discoveredZones))
	zones = append(zones, discoveredZones...)
	zones = append(zones, newZones...)

	resMap := make(map[string]struct{}, len(zones))
	res := make([]string, 0, len(zones))

	for i := range zones {
		if _, found := resMap[zones[i]]; !found {
			resMap[zones[i]] = struct{}{}
			res = append(res, zones[i])
		}
	}

	slices.Sort(res)
	return res
}

func mergeDatastores(discoveredZonedDataStores []v1.VsphereDatastore, newZonedDataStores []vsphere.ZonedDataStore) []v1.VsphereDatastore {
	zonedDataStores := make([]v1.VsphereDatastore, 0, len(discoveredZonedDataStores)+len(newZonedDataStores))
	zonedDataStores = append(zonedDataStores, discoveredZonedDataStores...)
	zonedDataStores = append(zonedDataStores, vsphereZonedDataStoresToV1(newZonedDataStores)...)

	res := make([]v1.VsphereDatastore, 0, len(zonedDataStores))
	resMap := make(map[string]struct{}, len(zonedDataStores))

	for i := range zonedDataStores {
		if _, found := resMap[zonedDataStores[i].Name]; !found {
			resMap[zonedDataStores[i].Name] = struct{}{}
			res = append(res, zonedDataStores[i])
		}
	}

	sort.SliceStable(res, func(i, j int) bool {
		return res[i].Name < res[j].Name
	})
	return res
}

func vsphereZonedDataStoresToV1(in []vsphere.ZonedDataStore) []v1.VsphereDatastore {
	result := make([]v1.VsphereDatastore, 0, len(in))
	for i := range in {
		result = append(result, v1.VsphereDatastore{
			Zones:         in[i].Zones,
			InventoryPath: in[i].InventoryPath,
			Name:          in[i].Name,
			DatastoreType: in[i].DatastoreType,
			DatastoreURL:  in[i].DatastoreURL,
		})
	}
	return result
}

// discoverThumbprint opens an independent TLS connection to the vCenter host and returns
// the SHA-1 fingerprint of the peer's leaf certificate, formatted as colon-separated
// upper-case hex (matching CAPV's expected shape, `AA:BB:CC:...:99`).
//
// The dial reuses the discoverer's trust settings (insecureFlag + caBundle-derived pool),
// so the certificate is validated by the same rules the SOAP session uses — a MitM the
// SOAP layer would reject cannot leak its fingerprint into VSphereCluster.spec.thumbprint.
// When insecureFlag is true validation is skipped entirely; that is the "no chain to
// verify against" case, and CAPV would in any event fall back to insecure without a
// thumbprint. Read-only, side-effect free.
func (d *Discoverer) discoverThumbprint(ctx context.Context) (string, error) {
	addr := d.host
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "443")
	}
	tlsCfg := &tls.Config{
		ServerName:         d.host,
		InsecureSkipVerify: d.insecureFlag, //nolint:gosec // matches session verification policy
		RootCAs:            d.caCertPool,
	}
	// DialContext honors caller cancellation and the reconcile-loop timeout, so a hung
	// vCenter cannot pin the discoverer goroutine forever. Timeout is set as a hard
	// upper bound on top of ctx to protect against callers passing context.Background().
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 10 * time.Second},
		Config:    tlsCfg,
	}
	rawConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", fmt.Errorf("dial %s: %w", addr, err)
	}
	conn := rawConn.(*tls.Conn)
	defer conn.Close()
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", errors.New("no peer certificates presented by vCenter")
	}
	sum := sha1.Sum(certs[0].Raw) //nolint:gosec // CAPV thumbprint format is SHA-1 by design
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, ":"), nil
}

func setCABundleIfNeed(logger *log.Logger, soapClient *soap.Client, insecure bool, caBundle string) error {
	if caBundle == "" {
		return nil
	}

	if insecure {
		logger.Warn("Set insecure flag to true, CA bundle will be ignored")
		return nil
	}

	logger.Debug("Setting up CA bundle for SOAP client")

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(caBundle)) {
		return errors.New("failed to parse CA bundle")
	}

	soapClient.DefaultTransport().TLSClientConfig.RootCAs = pool

	return nil
}
