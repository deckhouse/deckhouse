package main

import (
	"context"
	"os"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	log "github.com/sirupsen/logrus"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

type Discoverer struct {
	client *gophercloud.ServiceClient
	logger *log.Entry
}

func NewDiscoverer(logger *log.Entry) *Discoverer {
	authOpts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		logger.Fatalf("Cannnot get opts from env: %v", err)
	}

	provider, err := openstack.AuthenticatedClient(authOpts)
	if err != nil {
		logger.Fatalf("Cannnot create client: %v", err)
	}

	region := os.Getenv("OS_REGION")
	if region == "" {
		logger.Fatalf("Cannnot get OS_REGION env: %v", err)
	}

	client, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{
		Region: region,
	})

	if err != nil {
		logger.Fatalf("Cannnot create compute v2 client: %v", err)
	}

	return &Discoverer{
		client: client,
		logger: logger,
	}
}

func (d *Discoverer) InstanceTypes(_ context.Context) ([]v1alpha1.InstanceType, error) {
	pages, err := flavors.ListDetail(d.client, nil).AllPages()
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
