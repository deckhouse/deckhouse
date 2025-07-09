/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package vcd

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/vmware/go-vcloud-director/v2/govcd"
	"github.com/vmware/go-vcloud-director/v2/types/v56"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/cloud-data/discovery/meta"
)

type Discoverer struct {
	logger *log.Logger
	config *Config
}

type Config struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Org      string `json:"org"`
	Href     string `json:"href"`
	VDC      string `json:"vdc"`
	Insecure bool   `json:"insecure"`
	Token    string `json:"token"`
}

func ParseEnvToConfig() (*Config, error) {
	c := &Config{}
	password := os.Getenv("VCD_PASSWORD")
	token := os.Getenv("VCD_TOKEN")
	if password == "" && token == "" {
		return nil, fmt.Errorf("VCD_PASSWORD or VCD_TOKEN env should be set")
	}
	c.Password = password
	c.Token = token

	user := os.Getenv("VCD_USER")
	if user == "" && password != "" {
		return nil, fmt.Errorf("VCD_USER env should be set")
	}
	c.User = user

	org := os.Getenv("VCD_ORG")
	if org == "" {
		return nil, fmt.Errorf("VCD_ORG env should be set")
	}
	c.Org = org

	vdc := os.Getenv("VCD_VDC")
	if vdc == "" {
		return nil, fmt.Errorf("VCD_VDC env should be set")
	}
	c.VDC = vdc

	insecure := os.Getenv("VCD_INSECURE")
	if insecure == "true" {
		c.Insecure = true
	}

	href := os.Getenv("VCD_HREF")
	if href == "" {
		return nil, fmt.Errorf("VCD_HREF env should be set")
	}

	if !strings.HasSuffix(href, "api") {
		href = href + "/api"
	}

	c.Href = href

	return c, nil
}

// Client Creates a vCD client
func (c *Config) client() (*govcd.VCDClient, error) {
	u, err := url.ParseRequestURI(c.Href)
	if err != nil {
		return nil, fmt.Errorf("unable to pass url: %s", err)
	}

	vcdClient := govcd.NewVCDClient(*u, c.Insecure)
	if c.Token != "" {
		err := vcdClient.SetToken(c.Org, govcd.ApiTokenHeader, c.Token)
		if err != nil {
			return nil, fmt.Errorf("failed to set authorization header: %s", err)
		}
	} else {
		resp, err := vcdClient.GetAuthResponse(c.User, c.Password, c.Org)
		if err != nil {
			return nil, fmt.Errorf("unable to authenticate: %s", err)
		}
		fmt.Printf("Token: %s\n", resp.Header[govcd.AuthorizationHeader])
	}
	return vcdClient, nil
}

func NewDiscoverer(logger *log.Logger, config *Config) *Discoverer {
	return &Discoverer{
		logger: logger,
		config: config,
	}
}

func (d *Discoverer) CheckCloudConditions(ctx context.Context) ([]v1alpha1.CloudCondition, error) {
	return nil, nil
}

func (d *Discoverer) DiscoveryData(_ context.Context, options meta.DiscoveryDataOptions) ([]byte, error) {
	discoveryData := &v1alpha1.VCDCloudProviderDiscoveryData{
		Kind:       "VCDCloudProviderDiscoveryData",
		APIVersion: "deckhouse.io/v1alpha1",
	}
	if len(options.CloudProviderDiscoveryData) > 0 {
		err := json.Unmarshal(options.CloudProviderDiscoveryData, &discoveryData)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal cloud provider discovery data: %v", err)
		}
	}

	vcdClient, err := d.config.client()
	if err != nil {
		return nil, fmt.Errorf("failed to create vcd client: %v", err)
	}

	sizingPolicies := make([]string, 0)

	sp, err := d.getSizingPolicies(vcdClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get sizing policies: %v", err)
	}

	policiesDetails := make([]v1alpha1.VCDSizingPolicyDetails, 0, len(sp))

	for _, s := range sp {
		sizingPolicies = append(sizingPolicies, s.VdcComputePolicyV2.Name)
		policiesDetails = append(policiesDetails, sizingPolicyDetails(s.VdcComputePolicyV2))
	}

	if discoveryData.SizingPolicies != nil {
		sizingPolicies = append(sizingPolicies, discoveryData.SizingPolicies...)
	}

	sizingPolicies = removeDuplicatesStrings(sizingPolicies)

	discoveryData.SizingPolicies = sizingPolicies
	discoveryData.SizingPoliciesDetails = policiesDetails

	networks := make([]string, 0)

	nt, err := d.getInternalNetworks(vcdClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get internal networks: %v", err)
	}

	networksDetails := make([]v1alpha1.VCDInternalNetworkDetails, 0, len(nt))
	for _, net := range nt {
		networks = append(networks, net.Name)
		networksDetails = append(networksDetails, v1alpha1.VCDInternalNetworkDetails{
			Name: net.Name,
			CIDR: cidrNotation(net.Netmask, net.DefaultGateway),
		})
	}

	if discoveryData.InternalNetworks != nil {
		networks = append(networks, discoveryData.InternalNetworks...)
	}
	networks = removeDuplicatesStrings(networks)
	discoveryData.InternalNetworks = networks
	discoveryData.InternalNetworksDetails = networksDetails

	storageProfiles := make([]v1alpha1.VCDStorageProfile, 0)
	st, err := d.getStorageProfiles(vcdClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage profiles: %v", err)
	}

	if st != nil {
		storageProfiles = append(storageProfiles, st...)
	}

	if discoveryData.StorageProfiles != nil {
		storageProfiles = append(storageProfiles, discoveryData.StorageProfiles...)
	}

	storageProfiles = removeDuplicatesStorageProfiles(storageProfiles)
	discoveryData.StorageProfiles = storageProfiles

	vcdVersion, err := vcdClient.Client.GetVcdShortVersion()
	if err != nil {
		return nil, fmt.Errorf("could not get VCD version: %v", err)
	}
	discoveryData.VCDInstallationVersion = vcdVersion

	vcdAPIVersion, err := vcdClient.Client.MaxSupportedVersion()
	if err != nil {
		return nil, fmt.Errorf("could not get VCD API version: %v", err)
	}
	discoveryData.VCDAPIVersion = vcdAPIVersion

	vApps, err := d.getVApps(vcdClient)
	if err != nil {
		return nil, fmt.Errorf("could not get vApps: %w", err)
	}
	discoveryData.VApps = vApps

	vAppTemplates, err := d.getVAppTemplates(vcdClient)
	if err != nil {
		return nil, fmt.Errorf("cloud not get vAppTemplates: %w", err)
	}
	discoveryData.VAppTemplates = vAppTemplates

	vDCs, err := d.getVDCs(vcdClient)
	if err != nil {
		return nil, fmt.Errorf("could not get vDCs: %w", err)
	}
	discoveryData.VDCs = vDCs

	discoveryDataJson, err := json.Marshal(discoveryData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery data: %v", err)
	}

	d.logger.Debugf("discovery data: %v", discoveryDataJson)
	return discoveryDataJson, nil
}

func (d *Discoverer) getSizingPolicies(vcdClient *govcd.VCDClient) ([]*govcd.VdcComputePolicyV2, error) {
	sizingPolicies, err := vcdClient.GetAllVdcComputePoliciesV2(url.Values{})
	if err != nil {
		return nil, err
	}

	return sizingPolicies, nil
}

func (d *Discoverer) getInternalNetworks(vcdClient *govcd.VCDClient) ([]*types.QueryResultOrgVdcNetworkRecordType, error) {
	results, err := vcdClient.QueryWithNotEncodedParams(nil, map[string]string{
		"type": types.QtOrgVdcNetwork,
	})
	if err != nil {
		return nil, err
	}

	if len(results.Results.OrgVdcNetworkRecord) == 0 {
		return nil, nil
	}

	return results.Results.OrgVdcNetworkRecord, nil
}

func (d *Discoverer) getStorageProfiles(vcdClient *govcd.VCDClient) ([]v1alpha1.VCDStorageProfile, error) {
	results, err := vcdClient.QueryWithNotEncodedParams(nil, map[string]string{
		"type": types.QtOrgVdcStorageProfile,
	})
	if err != nil {
		return nil, err
	}

	if len(results.Results.OrgVdcStorageProfileRecord) == 0 {
		return nil, nil
	}

	profiles := make([]v1alpha1.VCDStorageProfile, 0, len(results.Results.OrgVdcStorageProfileRecord))

	for _, p := range results.Results.OrgVdcStorageProfileRecord {
		if p.Name == "" {
			continue
		}
		profiles = append(profiles, v1alpha1.VCDStorageProfile{
			Name:                    p.Name,
			IsEnabled:               p.IsEnabled,
			IsDefaultStorageProfile: p.IsDefaultStorageProfile,
		})
	}
	return profiles, nil
}

func (d *Discoverer) InstanceTypes(_ context.Context) ([]v1alpha1.InstanceType, error) {
	vcdClient, err := d.config.client()
	if err != nil {
		return nil, fmt.Errorf("failed to create vcd client: %v", err)
	}

	sizingPolicies, err := vcdClient.GetAllVdcComputePoliciesV2(url.Values{})
	if err != nil {
		return nil, err
	}

	if len(sizingPolicies) == 0 {
		return nil, nil
	}

	instanceTypes := make([]v1alpha1.InstanceType, 0, len(sizingPolicies))
	for _, s := range sizingPolicies {
		if s.VdcComputePolicyV2.Name == "" {
			continue
		}
		var cpuCount, memory int64
		if s.VdcComputePolicyV2.CPUCount != nil {
			cpuCount = int64(*s.VdcComputePolicyV2.CPUCount)
		}
		if s.VdcComputePolicyV2.Memory != nil {
			memory = int64(*s.VdcComputePolicyV2.Memory)
		}

		instanceTypes = append(instanceTypes, v1alpha1.InstanceType{
			Name:     s.VdcComputePolicyV2.Name,
			CPU:      resource.MustParse(strconv.FormatInt(cpuCount, 10)),
			Memory:   resource.MustParse(strconv.FormatInt(memory, 10) + "Mi"),
			RootDisk: resource.MustParse("0Gi"),
		})
	}

	instanceTypes = removeDuplicatesInstanceTypes(instanceTypes)
	return instanceTypes, nil
}

func (d *Discoverer) getVDCs(client *govcd.VCDClient) ([]string, error) {
	vdcRecord, err := client.QueryWithNotEncodedParams(nil, map[string]string{
		"type": "orgVdc",
	})
	if err != nil {
		return nil, fmt.Errorf("error getting VDCs: %w", err)
	}

	vdcs := make([]string, 0, len(vdcRecord.Results.OrgVdcRecord))
	for _, v := range vdcRecord.Results.OrgVdcRecord {
		vdcs = append(vdcs, v.Name)
	}
	return vdcs, nil
}

func (d *Discoverer) getVApps(client *govcd.VCDClient) ([]v1alpha1.VCDvApp, error) {
	vAppAll, err := client.Client.QueryVappList()
	if err != nil {
		return nil, fmt.Errorf("error getting vApps: %w", err)
	}

	vApps := make([]v1alpha1.VCDvApp, 0, len(vAppAll))
	for _, v := range vAppAll {
		if v.Status == "MIXED" {
			continue
		}
		vApps = append(vApps, v1alpha1.VCDvApp{
			Name:    v.Name,
			VDCName: v.VdcName,
		})
	}
	return vApps, nil
}

func (d *Discoverer) getVAppTemplates(client *govcd.VCDClient) ([]string, error) {
	templatesRecord, err := client.QueryWithNotEncodedParams(nil, map[string]string{
		"type": "vAppTemplate",
	})
	if err != nil {
		return nil, fmt.Errorf("error getting vAppTemplates: %w", err)
	}

	templates := make([]string, 0, len(templatesRecord.Results.VappTemplateRecord))

	for _, t := range templatesRecord.Results.VappTemplateRecord {
		if t.Name == "" || t.CatalogName == "" {
			d.logger.Debug("got unusable template with empty name or catalog name, skipping", "template_id", t.ID)
			continue
		}
		templates = append(templates, fmt.Sprintf("%s/%s", t.CatalogName, t.Name))
	}

	return templates, nil
}

// NotImplemented
func (d *Discoverer) DisksMeta(ctx context.Context) ([]v1alpha1.DiskMeta, error) {
	return []v1alpha1.DiskMeta{}, nil
}

// removeDuplicates removes duplicates from slice and sort it
func removeDuplicatesStrings(list []string) []string {
	if len(list) == 0 {
		return nil
	}

	keys := make(map[string]struct{})
	uniqueList := make([]string, 0, len(list))

	for _, elem := range list {
		if elem == "" {
			continue
		}

		if _, ok := keys[elem]; !ok {
			keys[elem] = struct{}{}
			uniqueList = append(uniqueList, elem)
		}
	}

	sort.Strings(uniqueList)
	return uniqueList
}

// removeDupluicatesStorageProfiles removes duplicates from slice and sort it
func removeDuplicatesStorageProfiles(list []v1alpha1.VCDStorageProfile) []v1alpha1.VCDStorageProfile {
	if len(list) == 0 {
		return nil
	}

	uniqueMap := make(map[string]v1alpha1.VCDStorageProfile, len(list))
	for _, elem := range list {
		if elem.Name == "" {
			continue
		}
		uniqueMap[elem.Name] = elem
	}

	uniqueList := make([]v1alpha1.VCDStorageProfile, 0, len(list))
	for _, elem := range uniqueMap {
		uniqueList = append(uniqueList, elem)
	}

	sort.SliceStable(uniqueList, func(i, j int) bool {
		return uniqueList[i].Name < uniqueList[j].Name
	})
	return uniqueList
}

// removeDuplicatesInstanceTypes removes duplicates from slice and sort it
func removeDuplicatesInstanceTypes(list []v1alpha1.InstanceType) []v1alpha1.InstanceType {
	if len(list) == 0 {
		return nil
	}

	uniqueMap := make(map[string]v1alpha1.InstanceType, len(list))
	for _, elem := range list {
		if elem.Name == "" {
			continue
		}
		uniqueMap[elem.Name] = elem
	}

	uniqueList := make([]v1alpha1.InstanceType, 0, len(list))
	for _, elem := range uniqueMap {
		uniqueList = append(uniqueList, elem)
	}

	sort.SliceStable(uniqueList, func(i, j int) bool {
		return uniqueList[i].Name < uniqueList[j].Name
	})
	return uniqueList
}

func cidrNotation(netMask, gateway string) string {
	mask := net.IPMask(net.ParseIP(netMask).To4())
	cidr, _ := mask.Size()

	ip := net.ParseIP(gateway)

	return fmt.Sprintf("%s/%d", ip.Mask(mask), cidr)
}

func sizingPolicyDetails(policy *types.VdcComputePolicyV2) v1alpha1.VCDSizingPolicyDetails {
	details := v1alpha1.VCDSizingPolicyDetails{Name: policy.Name}

	if policy.CPUCount != nil {
		details.VCPUs = *policy.CPUCount
	}
	if policy.Memory != nil {
		details.RAM = *policy.Memory
	}

	return details
}
