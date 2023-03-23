/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	log "github.com/sirupsen/logrus"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

type Discoverer struct {
	logger   *log.Entry
	authOpts gophercloud.AuthOptions
	region   string
}

func NewDiscoverer(logger *log.Entry) *Discoverer {
	authOpts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		logger.Fatalf("Cannnot get opts from env: %v", err)
	}

	region := os.Getenv("OS_REGION")
	if region == "" {
		logger.Fatalf("Cannnot get OS_REGION env")
	}

	return &Discoverer{
		logger:   logger,
		region:   region,
		authOpts: authOpts,
	}
}

func (d *Discoverer) InstanceTypes(_ context.Context) ([]v1alpha1.InstanceType, error) {
	provider, err := openstack.AuthenticatedClient(d.authOpts)
	if err != nil {
		return nil, fmt.Errorf("cannot create AuthenticatedClient: %v", err)
	}

	client, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{
		Region: d.region,
	})

	if err != nil {
		return nil, fmt.Errorf("cannot create ComputeV2 client: %v", err)
	}

	pages, err := flavors.ListDetail(client, nil).AllPages()
	if err != nil {
		return nil, err
	}

	flvs, err := flavors.ExtractFlavors(pages)
	if err != nil {
		return nil, err
	}

	res := make([]v1alpha1.InstanceType, 0, len(flvs))
	for _, f := range flvs {
		res = append(res, v1alpha1.InstanceType{
			Name:   f.Name,
			CPU:    int64(f.VCPUs),
			Memory: int64(f.RAM),
		})
	}

	return res, nil
}
