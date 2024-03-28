/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

type Discoverer struct {
	logger         *log.Entry
	location       string
	subscriptionID string
}

func NewDiscoverer(logger *log.Entry) *Discoverer {
	location := os.Getenv("AZURE_LOCATION")
	if location == "" {
		logger.Fatalf("Cannot get AZURE_LOCATION env")
	}

	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if location == "" {
		logger.Fatalf("Cannot get AZURE_SUBSCRIPTION_ID env")
	}

	return &Discoverer{
		logger:         logger,
		location:       location,
		subscriptionID: subscriptionID,
	}
}

func (d *Discoverer) InstanceTypes(ctx context.Context) ([]v1alpha1.InstanceType, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("cannot get credentials: %v", err)
	}

	cl, err := armcompute.NewResourceSKUsClient(d.subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create vm sizes client: %v", err)
	}

	pager := cl.NewListPager(&armcompute.ResourceSKUsClientListOptions{
		Filter: pointer.String(fmt.Sprintf("location eq '%s'", d.location)),
	})

	pagerCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	res := make([]v1alpha1.InstanceType, 0)
	for pager.More() {
		p, err := pager.NextPage(pagerCtx)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch next page: %v", err)
		}

		for _, r := range p.Value {
			process, err := d.continueProcessing(r)
			if err != nil {
				return nil, fmt.Errorf("%v: %v", err, r)
			}

			if !process {
				continue
			}

			cpu := ""
			memory := ""

			for _, cpb := range r.Capabilities {
				if cpu != "" && memory != "" {
					break
				}

				if cpb.Name == nil || cpb.Value == nil {
					return nil, fmt.Errorf("capability name or value is nil: %v", r)
				}

				switch *cpb.Name {
				case "MemoryGB":
					memory = *cpb.Value
					continue
				case "vCPUs":
					cpu = *cpb.Value
					continue
				}
			}

			if cpu == "" || memory == "" {
				return nil, fmt.Errorf("cpu or memory is zero: %v", r)
			}

			res = append(res, v1alpha1.InstanceType{
				Name:     *r.Name,
				CPU:      resource.MustParse(cpu),
				Memory:   resource.MustParse(memory + "Gi"),
				RootDisk: resource.MustParse("0"),
			})
		}
	}

	return res, nil
}

func (d *Discoverer) continueProcessing(r *armcompute.ResourceSKU) (bool, error) {
	if r == nil {
		return false, fmt.Errorf("sku is nil")
	}

	if r.ResourceType == nil {
		return false, fmt.Errorf("resource type is nil")
	}

	if r.Name == nil {
		return false, fmt.Errorf("name is nil")
	}

	if *r.ResourceType != "virtualMachines" {
		d.logger.Debugf("resource type is not virtual machine %s. skip", *r.ResourceType)
		return false, nil
	}

	for _, restr := range r.Restrictions {
		if restr.ReasonCode == nil {
			return false, fmt.Errorf("ReasonCode for restriction: %v", restr)
		}

		if *restr.ReasonCode == "NotAvailableForSubscription" {
			d.logger.Debugln("sku not available for subscription")
			return false, nil
		}
	}

	return true, nil
}

func (d *Discoverer) DiscoveryData(ctx context.Context, cloudProviderDiscoveryData []byte) ([]byte, error) {
	return nil, nil
}

// NotImplemented
func (d *Discoverer) DisksMeta(ctx context.Context) ([]v1alpha1.DiskMeta, error) {
	return []v1alpha1.DiskMeta{}, nil
}
