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
		logger.Fatalf("Cannnot get opts from env: %v", err)
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

	sizingPolicies, err := d.getSizingPolicies(vcdClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get sizing policies: %v", err)
	}

	discoveryData.SizingPolicies = sizingPolicies

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

	policies := make([]string, 0, len(sizingPolicies))

	for _, s := range sizingPolicies {
		if s.VdcComputePolicyV2.Name == "" {
			continue
		}
		policies = append(policies, s.VdcComputePolicyV2.Name)
	}
	return removeDuplicates(policies), nil
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

	instanceTypes := make([]v1alpha1.InstanceType, 0, len(sizingPolicies))
	for _, s := range sizingPolicies {
		if s.VdcComputePolicyV2.Name == "" {
			continue
		}
		instanceTypes = append(instanceTypes, v1alpha1.InstanceType{
			Name:     s.VdcComputePolicyV2.Name,
			CPU:      resource.MustParse(strconv.FormatInt(int64(*s.VdcComputePolicyV2.CPUCount), 10)),
			Memory:   resource.MustParse(strconv.FormatInt(int64(*s.VdcComputePolicyV2.Memory), 10) + "Mi"),
			RootDisk: resource.MustParse("0"),
		})
	}
	return instanceTypes, nil
}

type VCDCloudProviderDiscoveryData struct {
	SizingPolicies []string `json:"sizingPolicies,omitempty" yaml:"sizingPolicies,omitempty"`
}

// removeDuplicates removes duplicates from slice and sort it
func removeDuplicates(list []string) []string {
	var (
		keys       = make(map[string]struct{})
		uniqueList []string
	)

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
