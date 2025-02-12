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
	"strconv"

	log "github.com/sirupsen/logrus"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

type Discoverer struct {
	logger     *log.Entry
	credsFiles string
	project    string
	zones      []string
}

func NewDiscoverer(logger *log.Entry, credsFile, project string, zones []string) *Discoverer {
	return &Discoverer{
		credsFiles: credsFile,
		project:    project,
		logger:     logger,
		zones:      zones,
	}
}

func (d *Discoverer) InstanceTypes(ctx context.Context) ([]v1alpha1.InstanceType, error) {
	computeService, err := compute.NewService(ctx, option.WithCredentialsFile(d.credsFiles))
	if err != nil {
		return nil, err
	}

	instances := make(map[string]struct{})
	res := make([]v1alpha1.InstanceType, 0)

	for _, zone := range d.zones {
		req := computeService.MachineTypes.List(d.project, zone)
		if err := req.Pages(ctx, func(page *compute.MachineTypeList) error {
			for _, machineType := range page.Items {
				name, cpu, memory := machineType.Name, machineType.GuestCpus, machineType.MemoryMb
				iKey := key(name, cpu, memory)
				if _, ok := instances[iKey]; ok {
					continue
				}

				instances[iKey] = struct{}{}
				res = append(res, v1alpha1.InstanceType{
					Name:     name,
					CPU:      resource.MustParse(strconv.FormatInt(cpu, 10)),
					Memory:   resource.MustParse(strconv.FormatInt(memory, 10) + "Mi"),
					RootDisk: resource.MustParse("0"),
				})
			}

			return nil
		}); err != nil {
			return nil, err
		}
	}

	return res, nil
}

func key(name string, cpu, memory int64) string {
	return fmt.Sprintf("%s-%d-%d", name, cpu, memory)
}

func (d *Discoverer) DiscoveryData(ctx context.Context, cloudProviderDiscoveryData []byte) ([]byte, error) {
	return nil, nil
}

// NotImplemented
func (d *Discoverer) DisksMeta(ctx context.Context) ([]v1alpha1.DiskMeta, error) {
	return []v1alpha1.DiskMeta{}, nil
}
