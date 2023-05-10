/*
Copyright 2022 Flant JSC

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

package capacity

import (
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

var (
	ErrInvalidSpec = errors.New("Invalid InstanceClass spec")
	ErrNotFound    = errors.New("Instance Type not found")
)

type InstanceTypesCatalog struct {
	Types map[string]v1alpha1.InstanceType
}

func NewInstanceTypesCatalog(types []v1alpha1.InstanceType) *InstanceTypesCatalog {
	tt := make(map[string]v1alpha1.InstanceType, len(types))
	for _, t := range types {
		tt[t.Name] = t
	}

	return &InstanceTypesCatalog{
		Types: tt,
	}
}

func (c *InstanceTypesCatalog) Get(name string) (*v1alpha1.InstanceType, error) {
	inst, ok := c.Types[name]
	if !ok {
		return nil, ErrNotFound
	}

	return &inst, nil
}

// Capacity node capacity for autoscaler
type Capacity struct {
	CPU    resource.Quantity `json:"cpu,omitempty"`
	Memory resource.Quantity `json:"memory,omitempty"`
}

type vsphereInstanceClass struct {
	CPU    int `json:"numCPUs"`
	Memory int `json:"memory"`
}

func (vic vsphereInstanceClass) ExtractCapacity(_ *InstanceTypesCatalog) (*v1alpha1.InstanceType, error) {
	cpuStr := strconv.FormatInt(int64(vic.CPU), 10)
	memStr := strconv.FormatInt(int64(vic.Memory), 10)

	cpuRes, err := resource.ParseQuantity(cpuStr)
	if err != nil {
		return nil, err
	}
	memRes, err := resource.ParseQuantity(memStr + "Mi")
	if err != nil {
		return nil, err
	}

	return &v1alpha1.InstanceType{
		CPU:      cpuRes,
		Memory:   memRes,
		RootDisk: resource.MustParse("0"),
	}, nil
}

type awsInstanceClass struct {
	Capacity     *v1alpha1.InstanceType `json:"capacity,omitempty"`
	InstanceType string                 `json:"instanceType,omitempty"`
}

func (aic awsInstanceClass) ExtractCapacity(catalog *InstanceTypesCatalog) (*v1alpha1.InstanceType, error) {
	// TODO remove after 1.48
	// we cannot remove capacity now
	// because InstanceTypesCatalog may not have time for discovery
	// and we get empty catalog, machine deployment will be changed
	if aic.Capacity != nil {
		return aic.Capacity, nil
	}

	if aic.InstanceType == "" {
		return nil, ErrInvalidSpec
	}

	return catalog.Get(aic.InstanceType)
}

type azureInstanceClass struct {
	Capacity    *v1alpha1.InstanceType `json:"capacity,omitempty"`
	MachineSize string                 `json:"machineSize,omitempty"`
}

func (azic azureInstanceClass) ExtractCapacity(catalog *InstanceTypesCatalog) (*v1alpha1.InstanceType, error) {
	// TODO remove after 1.48
	// we cannot remove capacity now
	// because InstanceTypesCatalog may not have time for discovery
	// and we get empty catalog, machine deployment will be changed
	if azic.Capacity != nil {
		return azic.Capacity, nil
	}

	if azic.MachineSize == "" {
		return nil, ErrInvalidSpec
	}

	return catalog.Get(azic.MachineSize)
}

type gcpInstanceClass struct {
	Capacity    *v1alpha1.InstanceType `json:"capacity,omitempty"`
	MachineType string                 `json:"machineType,omitempty"`
}

func (gic gcpInstanceClass) ExtractCapacity(catalog *InstanceTypesCatalog) (*v1alpha1.InstanceType, error) {
	// TODO remove after 1.48
	// we cannot remove capacity now
	// because InstanceTypesCatalog may not have time for discovery
	// and we get empty catalog, machine deployment will be changed
	if gic.Capacity != nil {
		return gic.Capacity, nil
	}

	if gic.MachineType == "" {
		return nil, ErrInvalidSpec
	}

	return catalog.Get(gic.MachineType)
}

type yandexInstanceClass struct {
	Cores  int `json:"cores"`
	Memory int `json:"memory"`
}

func (yic yandexInstanceClass) ExtractCapacity(_ *InstanceTypesCatalog) (*v1alpha1.InstanceType, error) {
	cpuStr := strconv.FormatInt(int64(yic.Cores), 10)
	memStr := strconv.FormatInt(int64(yic.Memory), 10)

	cpuRes, err := resource.ParseQuantity(cpuStr)
	if err != nil {
		return nil, err
	}
	memRes, err := resource.ParseQuantity(memStr + "Mi")
	if err != nil {
		return nil, err
	}

	return &v1alpha1.InstanceType{
		CPU:      cpuRes,
		Memory:   memRes,
		RootDisk: resource.MustParse("0"),
	}, nil
}

type openStackInstanceClass struct {
	Capacity   *v1alpha1.InstanceType `json:"capacity,omitempty"`
	FlavorName string                 `json:"flavorName,omitempty"`
}

func (osic openStackInstanceClass) ExtractCapacity(catalog *InstanceTypesCatalog) (*v1alpha1.InstanceType, error) {
	// TODO remove after 1.48
	// we cannot remove capacity now
	// because InstanceTypesCatalog may not have time for discovery
	// and we get empty catalog, machine deployment will be changed
	if osic.Capacity != nil {
		return osic.Capacity, nil
	}

	if osic.FlavorName == "" {
		return nil, ErrInvalidSpec
	}

	return catalog.Get(osic.FlavorName)
}

// extract capacity from defined InstanceClass
type capacityExtractor interface {
	ExtractCapacity(catalog *InstanceTypesCatalog) (*v1alpha1.InstanceType, error)
}

// CalculateNodeTemplateCapacity calculates capacity of the node based on InstanceClass and it's spec
func CalculateNodeTemplateCapacity(instanceClassName string, instanceClassSpec interface{}, catalog *InstanceTypesCatalog) (*v1alpha1.InstanceType, error) {
	var extractor capacityExtractor

	switch instanceClassName {
	case "VsphereInstanceClass":
		var spec vsphereInstanceClass
		extractor = &spec

	case "AWSInstanceClass":
		var spec awsInstanceClass
		extractor = &spec

	case "AzureInstanceClass":
		var spec azureInstanceClass
		extractor = &spec

	case "GCPInstanceClass":
		var spec gcpInstanceClass
		extractor = &spec

	case "YandexInstanceClass":
		var spec yandexInstanceClass
		extractor = &spec

	case "OpenStackInstanceClass":
		var spec openStackInstanceClass
		extractor = &spec

	case "D8TestInstanceClass":
		// for test purpose
		testspec := instanceClassSpec.(map[string]interface{})
		if len(testspec) == 0 {
			return nil, errors.New("Expected error for test")
		}
		return &v1alpha1.InstanceType{
			CPU:    resource.MustParse("4"),
			Memory: resource.MustParse("8Gi"),
		}, nil

	default:
		return nil, errors.New("Unknown cloud provider")
	}

	// unless we don't have structs for InstanceClasses this is the easiest way to get fields from abstract spec
	// trying to use type assertion is much uglier, don't try to use it
	data, _ := json.Marshal(instanceClassSpec)
	err := json.Unmarshal(data, extractor)
	if err != nil {
		return nil, err
	}

	return extractor.ExtractCapacity(catalog)
}
