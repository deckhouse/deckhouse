/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package vcd

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/vmware/go-vcloud-director/v2/govcd"
)

type Config struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Org      string `json:"org"`
	Href     string `json:"href"`
	VDC      string `json:"vdc"`
	VApp     string `json:"vapp"`
	Insecure bool   `json:"insecure"`
	Token    string `json:"token"`
}

func NewConfigFromEnv() (*Config, error) {
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

	vapp := os.Getenv("VCD_VAPP")
	if vapp == "" {
		return nil, fmt.Errorf("VCD_VAPP env should be set")
	}
	c.VApp = vapp

	insecure := os.Getenv("VCD_INSECURE")
	if insecure == "true" {
		c.Insecure = true
	}

	href := os.Getenv("VCD_HREF")
	if href == "" {
		return nil, fmt.Errorf("VCD_HREF env should be set")
	}

	if !strings.HasSuffix(href, "api") {
		href = fmt.Sprintf("%s/api", href)
	}

	c.Href = href

	return c, nil
}

func (c *Config) NewClient() (*govcd.VCDClient, error) {
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

func (c *Config) NewOrgClient() (*govcd.Org, error) {
	vcdClient, err := c.NewClient()
	if err != nil {
		return nil, err
	}

	orgClient, err := govcd.GetOrgByName(vcdClient, c.Org)
	if err != nil {
		return nil, err
	}

	return &orgClient, nil
}

func (c *Config) NewVDCClient() (*govcd.Vdc, error) {
	orgClient, err := c.NewOrgClient()
	if err != nil {
		return nil, err
	}

	vdcClient, err := orgClient.GetVDCByName(c.VDC, false)
	if err != nil {
		return nil, err
	}

	return vdcClient, nil
}

func (c *Config) NewVAppClient() (*govcd.VApp, error) {
	vdcClient, err := c.NewVDCClient()
	if err != nil {
		return nil, err
	}

	return c.NewVAppClientFromVDCClient(vdcClient)
}

func (c *Config) NewVAppClientFromVDCClient(vdcClient *govcd.Vdc) (*govcd.VApp, error) {
	if vdcClient == nil {
		return nil, fmt.Errorf("vdcClient is nil")
	}

	vappClient, err := vdcClient.GetVAppByName(c.VApp, false)
	if err != nil {
		return nil, err
	}

	return vappClient, nil
}
