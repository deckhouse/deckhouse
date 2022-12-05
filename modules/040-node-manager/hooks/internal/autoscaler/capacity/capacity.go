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

	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/autoscaler/instance_types"
)

var (
	ErrInvalidSpec = errors.New("Invalid InstanceClass spec")
)

// Capacity node capacity for autoscaler
type Capacity struct {
	CPU    resource.Quantity `json:"cpu,omitempty"`
	Memory resource.Quantity `json:"memory,omitempty"`
}

type vsphereInstanceClass struct {
	CPU    int `json:"numCPUs"`
	Memory int `json:"memory"`
}

func (vic vsphereInstanceClass) ExtractCapacity() (*Capacity, error) {
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

	return &Capacity{
		CPU:    cpuRes,
		Memory: memRes,
	}, nil
}

type awsInstanceClass struct {
	Capacity     *Capacity `json:"capacity,omitempty"`
	InstanceType string    `json:"instanceType,omitempty"`
}

func (aic awsInstanceClass) ExtractCapacity() (*Capacity, error) {
	if aic.Capacity != nil {
		return aic.Capacity, nil
	}

	if aic.InstanceType == "" {
		return nil, ErrInvalidSpec
	}

	inst, err := instance_types.GetInstanceType("AWSInstanceClass", aic.InstanceType)
	if err != nil {
		return nil, err
	}

	return &Capacity{
		CPU:    resource.MustParse(strconv.FormatInt(int64(inst.VCPU), 10)),
		Memory: resource.MustParse(strconv.FormatInt(int64(inst.MemoryMb), 10) + "Mi"),
	}, nil
}

type azureInstanceClass struct {
	Capacity    *Capacity `json:"capacity,omitempty"`
	MachineSize string    `json:"machineSize,omitempty"`
}

func (azic azureInstanceClass) ExtractCapacity() (*Capacity, error) {
	if azic.Capacity != nil {
		return azic.Capacity, nil
	}

	if azic.MachineSize == "" {
		return nil, ErrInvalidSpec
	}

	inst, err := instance_types.GetInstanceType("AzureInstanceClass", azic.MachineSize)
	if err != nil {
		return nil, err
	}

	return &Capacity{
		CPU:    resource.MustParse(strconv.FormatInt(int64(inst.VCPU), 10)),
		Memory: resource.MustParse(strconv.FormatInt(int64(inst.MemoryMb), 10) + "Mi"),
	}, nil
}

type gcpInstanceClass struct {
	Capacity    *Capacity `json:"capacity,omitempty"`
	MachineType string    `json:"machineType,omitempty"`
}

func (gic gcpInstanceClass) ExtractCapacity() (*Capacity, error) {
	if gic.Capacity != nil {
		return gic.Capacity, nil
	}

	if gic.MachineType == "" {
		return nil, ErrInvalidSpec
	}

	inst, err := instance_types.GetInstanceType("GCPInstanceClass", gic.MachineType)
	if err != nil {
		return nil, err
	}

	return &Capacity{
		CPU:    resource.MustParse(strconv.FormatInt(int64(inst.VCPU), 10)),
		Memory: resource.MustParse(strconv.FormatInt(int64(inst.MemoryMb), 10) + "Mi"),
	}, nil
}

type yandexInstanceClass struct {
	Cores  int `json:"cores"`
	Memory int `json:"memory"`
}

func (yic yandexInstanceClass) ExtractCapacity() (*Capacity, error) {
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

	return &Capacity{
		CPU:    cpuRes,
		Memory: memRes,
	}, nil
}

type openStackInstanceClass struct {
	Capacity   *Capacity `json:"capacity,omitempty"`
	FlavorName string    `json:"flavorName,omitempty"`
}

func (osic openStackInstanceClass) ExtractCapacity() (*Capacity, error) {
	if osic.Capacity != nil {
		return osic.Capacity, nil
	}

	if osic.FlavorName == "" {
		return nil, ErrInvalidSpec
	}

	inst, err := instance_types.GetInstanceType("OpenStackInstanceClass", osic.FlavorName)
	if err != nil {
		return nil, err
	}

	return &Capacity{
		CPU:    resource.MustParse(strconv.FormatInt(int64(inst.VCPU), 10)),
		Memory: resource.MustParse(strconv.FormatInt(int64(inst.MemoryMb), 10) + "Mi"),
	}, nil
}

// extract capacity from defined InstanceClass
type capacityExtractor interface {
	ExtractCapacity() (*Capacity, error)
}

// CalculateNodeTemplateCapacity calculates capacity of the node based on InstanceClass and it's spec
func CalculateNodeTemplateCapacity(instanceClassName string, instanceClassSpec interface{}) (*Capacity, error) {
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
		return &Capacity{
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

	return extractor.ExtractCapacity()
}
