/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/vmware/go-vcloud-director/v2/govcd"
	"github.com/vmware/go-vcloud-director/v2/types/v56"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

type Discoverer struct {
	logger *log.Entry
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

func parseEnvToConfig() (*Config, error) {
	c := &Config{}
	user := os.Getenv("VCD_USER")
	if user == "" {
		return nil, fmt.Errorf("VCD_USER env should be set")
	}
	c.User = user

	password := os.Getenv("VCD_PASSWORD")
	token := os.Getenv("VCD_TOKEN")
	if password == "" && token == "" {
		return nil, fmt.Errorf("VCD_PASSWORD or VCD_TOKEN env should be set")
	}
	c.Password = password
	c.Token = token

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
		_ = vcdClient.SetToken(c.Org, govcd.AuthorizationHeader, c.Token)
	} else {
		resp, err := vcdClient.GetAuthResponse(c.User, c.Password, c.Org)
		if err != nil {
			return nil, fmt.Errorf("unable to authenticate: %s", err)
		}
		fmt.Printf("Token: %s\n", resp.Header[govcd.AuthorizationHeader])
	}
	return vcdClient, nil
}

func NewDiscoverer(logger *log.Entry) *Discoverer {
	config, err := parseEnvToConfig()
	if err != nil {
		logger.Fatalf("Cannot get opts from env: %v", err)
	}

	return &Discoverer{
		logger: logger,
		config: config,
	}
}

func (d *Discoverer) DiscoveryData(_ context.Context, cloudProviderDiscoveryData []byte) ([]byte, error) {
	discoveryData := &v1alpha1.VCDCloudProviderDiscoveryData{}
	if len(cloudProviderDiscoveryData) > 0 {
		err := json.Unmarshal(cloudProviderDiscoveryData, &discoveryData)
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

	if sp != nil {
		sizingPolicies = append(sizingPolicies, sp...)
	}

	if discoveryData.SizingPolicies != nil {
		sizingPolicies = append(sizingPolicies, discoveryData.SizingPolicies...)
	}

	sizingPolicies = removeDuplicatesStrings(sizingPolicies)

	discoveryData.SizingPolicies = sizingPolicies

	networks := make([]string, 0)
	nt, err := d.getInternalNetworks(vcdClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get internal networks: %v", err)
	}

	if nt != nil {
		networks = append(networks, nt...)
	}

	if discoveryData.InternalNetworks != nil {
		networks = append(networks, discoveryData.InternalNetworks...)
	}
	networks = removeDuplicatesStrings(networks)
	discoveryData.InternalNetworks = networks

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

	vcdVersion, err := d.getVersion(vcdClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get VCD version: %v", err)
	}
	discoveryData.Version = vcdVersion

	discoveryDataJson, err := json.Marshal(discoveryData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery data: %v", err)
	}

	d.logger.Debugf("discovery data: %v", discoveryDataJson)
	return discoveryDataJson, nil
}

func (d *Discoverer) getSizingPolicies(vcdClient *govcd.VCDClient) ([]string, error) {
	sizingPolicies, err := vcdClient.GetAllVdcComputePoliciesV2(url.Values{})
	if err != nil {
		return nil, err
	}

	if len(sizingPolicies) == 0 {
		return nil, nil
	}

	policies := make([]string, 0, len(sizingPolicies))

	for _, s := range sizingPolicies {
		policies = append(policies, s.VdcComputePolicyV2.Name)
	}

	return policies, nil
}

func (d *Discoverer) getInternalNetworks(vcdClient *govcd.VCDClient) ([]string, error) {
	results, err := vcdClient.QueryWithNotEncodedParams(nil, map[string]string{
		"type": types.QtOrgVdcNetwork,
	})
	if err != nil {
		return nil, err
	}

	if len(results.Results.OrgVdcNetworkRecord) == 0 {
		return nil, nil
	}

	networks := make([]string, 0, len(results.Results.OrgVdcNetworkRecord))

	for _, n := range results.Results.OrgVdcNetworkRecord {
		networks = append(networks, n.Name)
	}

	return networks, nil
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

func (d *Discoverer) getVersion(vcdClient *govcd.VCDClient) (v1alpha1.VCDVersion, error) {
	vcdVersion, err := vcdClient.Client.GetVcdShortVersion()
	if err != nil {
		return v1alpha1.VCDVersion{}, fmt.Errorf("could not get VCD version: %v", err)
	}
	apiVersion, err := vcdClient.Client.MaxSupportedVersion()
	if err != nil {
		return v1alpha1.VCDVersion{}, fmt.Errorf("could not get VCD API version: %v", err)
	}

	return v1alpha1.VCDVersion{VCDVersion: vcdVersion, APIVersion: apiVersion}, nil
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
