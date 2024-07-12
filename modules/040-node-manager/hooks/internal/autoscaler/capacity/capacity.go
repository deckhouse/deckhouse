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

func (c *InstanceTypesCatalog) Get(ic instanceClass) (*v1alpha1.InstanceType, error) {
	iType := ic.GetType()
	if iType == "" {
		return nil, ErrInvalidSpec
	}

	inst, ok := c.Types[iType]
	if ok {
		return &inst, nil
	}

	// TODO remove after 1.48
	// we cannot remove capacity now
	// because InstanceTypesCatalog may not have time for discovery
	// and we get empty catalog, machine deployment will be changed
	capacity := ic.GetCapacity()
	if capacity != nil {
		return capacity.ToInstanceType(), nil
	}

	return nil, ErrNotFound
}

// Capacity node capacity for autoscaler
type Capacity struct {
	CPU    resource.Quantity `json:"cpu,omitempty"`
	Memory resource.Quantity `json:"memory,omitempty"`
}

func (c *Capacity) ToInstanceType() *v1alpha1.InstanceType {
	return &v1alpha1.InstanceType{
		CPU:      c.CPU.DeepCopy(),
		Memory:   c.Memory.DeepCopy(),
		RootDisk: resource.MustParse("0"),
	}
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
	Capacity     *Capacity `json:"capacity,omitempty"`
	InstanceType string    `json:"instanceType,omitempty"`
}

func (aic *awsInstanceClass) GetCapacity() *Capacity {
	return aic.Capacity
}

func (aic *awsInstanceClass) GetType() string {
	return aic.InstanceType
}

func (aic *awsInstanceClass) ExtractCapacity(catalog *InstanceTypesCatalog) (*v1alpha1.InstanceType, error) {
	return catalog.Get(aic)
}

type azureInstanceClass struct {
	Capacity    *Capacity `json:"capacity,omitempty"`
	MachineSize string    `json:"machineSize,omitempty"`
}

func (aic *azureInstanceClass) GetCapacity() *Capacity {
	return aic.Capacity
}

func (aic *azureInstanceClass) GetType() string {
	return aic.MachineSize
}

func (aic *azureInstanceClass) ExtractCapacity(catalog *InstanceTypesCatalog) (*v1alpha1.InstanceType, error) {
	return catalog.Get(aic)
}

type gcpInstanceClass struct {
	Capacity    *Capacity `json:"capacity,omitempty"`
	MachineType string    `json:"machineType,omitempty"`
}

func (aic *gcpInstanceClass) GetCapacity() *Capacity {
	return aic.Capacity
}

func (aic *gcpInstanceClass) GetType() string {
	return aic.MachineType
}

func (aic *gcpInstanceClass) ExtractCapacity(catalog *InstanceTypesCatalog) (*v1alpha1.InstanceType, error) {
	return catalog.Get(aic)
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
	Capacity   *Capacity `json:"capacity,omitempty"`
	FlavorName string    `json:"flavorName,omitempty"`
}

func (aic *openStackInstanceClass) GetCapacity() *Capacity {
	return aic.Capacity
}

func (aic *openStackInstanceClass) GetType() string {
	return aic.FlavorName
}

func (aic *openStackInstanceClass) ExtractCapacity(catalog *InstanceTypesCatalog) (*v1alpha1.InstanceType, error) {
	return catalog.Get(aic)
}

type vcdInstanceClass struct {
	SizingPolicy string `json:"sizingPolicy,omitempty"`
}

func (aic *vcdInstanceClass) GetCapacity() *Capacity {
	return nil
}

func (aic *vcdInstanceClass) GetType() string {
	return aic.SizingPolicy
}

func (aic *vcdInstanceClass) ExtractCapacity(catalog *InstanceTypesCatalog) (*v1alpha1.InstanceType, error) {
	return catalog.Get(aic)
}

type zvirtInstanceClass struct {
	Cores  int `json:"cores"`
	Memory int `json:"memory"`
}

func (zic zvirtInstanceClass) ExtractCapacity(_ *InstanceTypesCatalog) (*v1alpha1.InstanceType, error) {
	cpuStr := strconv.FormatInt(int64(zic.Cores), 10)
	memStr := strconv.FormatInt(int64(zic.Memory), 10)

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

type testInstanceClass struct {
	Capacity *Capacity `json:"capacity,omitempty"`
	Type     string    `json:"type,omitempty"`
}

func (aic *testInstanceClass) GetCapacity() *Capacity {
	return aic.Capacity
}

func (aic *testInstanceClass) GetType() string {
	return aic.Type
}

func (aic *testInstanceClass) ExtractCapacity(catalog *InstanceTypesCatalog) (*v1alpha1.InstanceType, error) {
	return catalog.Get(aic)
}

type instanceClass interface {
	GetCapacity() *Capacity
	GetType() string
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

	case "VCDInstanceClass":
		var spec vcdInstanceClass
		extractor = &spec

	case "ZvirtInstanceClass":
		var spec zvirtInstanceClass
		extractor = &spec

	case "D8TestInstanceClass":
		var spec testInstanceClass
		extractor = &spec
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
