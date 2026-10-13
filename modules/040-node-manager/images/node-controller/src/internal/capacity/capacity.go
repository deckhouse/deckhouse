/*
Copyright 2026 Flant JSC

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
)

var (
	ErrInvalidSpec = errors.New("Invalid InstanceClass spec")
	ErrNotFound    = errors.New("Instance Type not found")
)

type InstanceType struct {
	Name   string            `json:"name"`
	CPU    resource.Quantity `json:"cpu"`
	Memory resource.Quantity `json:"memory"`
	// RootDisk is the catalog's disk of the instance type, and 0 for a type resolved from the
	// InstanceClass fields. It is published in the NodeGroup element as nodeCapacity.rootDisk,
	// which bashible-apiserver hashes into every node's configurationChecksum, so it keeps exactly
	// the value it has always had. The disk the machine really gets is BootDisk.
	RootDisk resource.Quantity `json:"rootDisk"`
	// BootDisk is the size of the disk the machine boots from, as the provider creates it, and
	// zero when it is unknown. It feeds the CAPI ephemeral-disk annotation and is never serialized,
	// see RootDisk.
	BootDisk resource.Quantity `json:"-"`
}

// diskGiB returns a disk of sizeGiB, or of defaultGiB when the InstanceClass leaves the size
// unset. defaultGiB is the size the provider creates the disk with in that case, and 0 when
// node-controller cannot know it.
func diskGiB(sizeGiB, defaultGiB int) resource.Quantity {
	if sizeGiB <= 0 {
		sizeGiB = defaultGiB
	}
	return *resource.NewQuantity(int64(sizeGiB)<<30, resource.BinarySI)
}

// withBootDisk sets the boot disk of an instance type resolved from the catalog or from
// spec.capacity: the size the InstanceClass names, otherwise the catalog's disk of the type.
func withBootDisk(it *InstanceType, err error, size resource.Quantity) (*InstanceType, error) {
	if err != nil {
		return it, err
	}
	if size.IsZero() {
		size = it.RootDisk
	}
	it.BootDisk = size
	return it, nil
}

type InstanceTypesCatalog struct {
	Types map[string]InstanceType
}

func NewInstanceTypesCatalog(types []InstanceType) *InstanceTypesCatalog {
	tt := make(map[string]InstanceType, len(types))
	for _, t := range types {
		tt[t.Name] = t
	}

	return &InstanceTypesCatalog{
		Types: tt,
	}
}

func (c *InstanceTypesCatalog) Get(ic instanceClass) (*InstanceType, error) {
	iType := ic.GetType()
	if iType == "" {
		return nil, ErrInvalidSpec
	}

	inst, ok := c.Types[iType]
	if ok {
		return &inst, nil
	}

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

// capacityFromExplicitField reads a provider-agnostic spec.capacity {cpu, memory} from any
// InstanceClass spec. It returns nil unless both cpu and memory are set, so a partial or
// absent field falls back to the provider-specific extraction.
func capacityFromExplicitField(instanceClassSpec interface{}) *InstanceType {
	var generic struct {
		Capacity *Capacity `json:"capacity,omitempty"`
	}
	data, err := json.Marshal(instanceClassSpec)
	if err != nil {
		return nil
	}
	if err := json.Unmarshal(data, &generic); err != nil {
		return nil
	}
	if generic.Capacity == nil || generic.Capacity.CPU.IsZero() || generic.Capacity.Memory.IsZero() {
		return nil
	}
	return generic.Capacity.ToInstanceType()
}

func (c *Capacity) ToInstanceType() *InstanceType {
	return &InstanceType{
		CPU:      c.CPU.DeepCopy(),
		Memory:   c.Memory.DeepCopy(),
		RootDisk: resource.MustParse("0"),
	}
}

type vsphereInstanceClass struct {
	CPU          int `json:"numCPUs"`
	Memory       int `json:"memory"`
	RootDiskSize int `json:"rootDiskSize"`
}

func (vic vsphereInstanceClass) ExtractCapacity(_ *InstanceTypesCatalog) (*InstanceType, error) {
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

	return &InstanceType{
		CPU:      cpuRes,
		Memory:   memRes,
		RootDisk: resource.MustParse("0"),
		// 20 is cloud-provider-vsphere capi/template.yaml's default.
		BootDisk: diskGiB(vic.RootDiskSize, 20),
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

func (aic *awsInstanceClass) ExtractCapacity(catalog *InstanceTypesCatalog) (*InstanceType, error) {
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

func (aic *azureInstanceClass) ExtractCapacity(catalog *InstanceTypesCatalog) (*InstanceType, error) {
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

func (aic *gcpInstanceClass) ExtractCapacity(catalog *InstanceTypesCatalog) (*InstanceType, error) {
	return catalog.Get(aic)
}

type yandexInstanceClass struct {
	Cores      int `json:"cores"`
	Memory     int `json:"memory"`
	DiskSizeGB int `json:"diskSizeGB"`
}

func (yic yandexInstanceClass) ExtractCapacity(_ *InstanceTypesCatalog) (*InstanceType, error) {
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

	return &InstanceType{
		CPU:      cpuRes,
		Memory:   memRes,
		RootDisk: resource.MustParse("0"),
		// 50 is cloud-provider-yandex capi/template.yaml's default.
		BootDisk: diskGiB(yic.DiskSizeGB, 50),
	}, nil
}

type openStackInstanceClass struct {
	Capacity   *Capacity `json:"capacity,omitempty"`
	FlavorName string    `json:"flavorName,omitempty"`
	// RootDiskSize is unset when the machine boots from the flavor's disk, which the catalog
	// carries as the type's RootDisk.
	RootDiskSize int `json:"rootDiskSize,omitempty"`
}

func (aic *openStackInstanceClass) GetCapacity() *Capacity {
	return aic.Capacity
}

func (aic *openStackInstanceClass) GetType() string {
	return aic.FlavorName
}

func (aic *openStackInstanceClass) ExtractCapacity(catalog *InstanceTypesCatalog) (*InstanceType, error) {
	it, err := catalog.Get(aic)
	return withBootDisk(it, err, diskGiB(aic.RootDiskSize, 0))
}

type vcdInstanceClass struct {
	SizingPolicy string `json:"sizingPolicy,omitempty"`
	// RootDiskSizeGb is defaulted to 20 by the VCDInstanceClass CRD.
	RootDiskSizeGb int `json:"rootDiskSizeGb,omitempty"`
}

func (aic *vcdInstanceClass) GetCapacity() *Capacity {
	return nil
}

func (aic *vcdInstanceClass) GetType() string {
	return aic.SizingPolicy
}

func (aic *vcdInstanceClass) ExtractCapacity(catalog *InstanceTypesCatalog) (*InstanceType, error) {
	it, err := catalog.Get(aic)
	return withBootDisk(it, err, diskGiB(aic.RootDiskSizeGb, 0))
}

type zvirtInstanceClass struct {
	Cores          int `json:"cores"`
	Memory         int `json:"memory"`
	RootDiskSizeGb int `json:"rootDiskSizeGb"`
}

func (zic zvirtInstanceClass) ExtractCapacity(_ *InstanceTypesCatalog) (*InstanceType, error) {
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

	return &InstanceType{
		CPU:      cpuRes,
		Memory:   memRes,
		RootDisk: resource.MustParse("0"),
		// 20 is the ZvirtMachineTemplate CRD default (cloud-provider-zvirt crds/external).
		BootDisk: diskGiB(zic.RootDiskSizeGb, 20),
	}, nil
}

type dynamixInstanceClass struct {
	Cores          int `json:"cores"`
	Memory         int `json:"memory"`
	RootDiskSizeGb int `json:"rootDiskSizeGb"`
}

func (d dynamixInstanceClass) ExtractCapacity(_ *InstanceTypesCatalog) (*InstanceType, error) {
	cpuStr := strconv.FormatInt(int64(d.Cores), 10)
	memStr := strconv.FormatInt(int64(d.Memory), 10)

	cpuRes, err := resource.ParseQuantity(cpuStr)
	if err != nil {
		return nil, err
	}
	memRes, err := resource.ParseQuantity(memStr + "Mi")
	if err != nil {
		return nil, err
	}

	return &InstanceType{
		CPU:      cpuRes,
		Memory:   memRes,
		RootDisk: resource.MustParse("0"),
		// 30 is the DynamixMachineTemplate CRD default (cloud-provider-dynamix crds/external).
		BootDisk: diskGiB(d.RootDiskSizeGb, 30),
	}, nil
}

type huaweiCloudInstanceClass struct {
	FlavorName   string `json:"flavorName,omitempty"`
	RootDiskSize int    `json:"rootDiskSize,omitempty"`
}

func (c *huaweiCloudInstanceClass) GetCapacity() *Capacity {
	return nil
}

func (c *huaweiCloudInstanceClass) GetType() string {
	return c.FlavorName
}

func (c *huaweiCloudInstanceClass) ExtractCapacity(catalog *InstanceTypesCatalog) (*InstanceType, error) {
	it, err := catalog.Get(c)
	// 30 is the HuaweiCloudMachineTemplate CRD default (cloud-provider-huaweicloud crds/external).
	return withBootDisk(it, err, diskGiB(c.RootDiskSize, 30))
}

type dvpInstanceClass struct {
	// RootDisk.Size is required by the DVPInstanceClass CRD.
	RootDisk struct {
		Size string `json:"size"`
	} `json:"rootDisk"`

	VirtualMachine struct {
		CPU struct {
			Cores int `json:"cores"`
		} `json:"cpu"`

		Memory struct {
			Size string `json:"size"`
		} `json:"memory"`
	} `json:"virtualMachine"`
}

func (d dvpInstanceClass) ExtractCapacity(_ *InstanceTypesCatalog) (*InstanceType, error) {
	cpuStr := strconv.FormatInt(int64(d.VirtualMachine.CPU.Cores), 10)

	cpuRes, err := resource.ParseQuantity(cpuStr)
	if err != nil {
		return nil, err
	}

	memRes, err := resource.ParseQuantity(d.VirtualMachine.Memory.Size)
	if err != nil {
		return nil, err
	}

	// The disk only feeds the ephemeral-disk annotation, so an unparsable size leaves it unknown
	// instead of failing the CPU and memory the template cannot do without.
	diskRes, err := resource.ParseQuantity(d.RootDisk.Size)
	if err != nil {
		diskRes = resource.MustParse("0")
	}

	return &InstanceType{
		CPU:      cpuRes,
		Memory:   memRes,
		RootDisk: resource.MustParse("0"),
		BootDisk: diskRes,
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

func (aic *testInstanceClass) ExtractCapacity(catalog *InstanceTypesCatalog) (*InstanceType, error) {
	return catalog.Get(aic)
}

type instanceClass interface {
	GetCapacity() *Capacity
	GetType() string
}

// extract capacity from defined InstanceClass
type capacityExtractor interface {
	ExtractCapacity(catalog *InstanceTypesCatalog) (*InstanceType, error)
}

// CalculateNodeTemplateCapacity calculates capacity of the node based on InstanceClass and it's spec
func CalculateNodeTemplateCapacity(instanceClassName string, instanceClassSpec interface{}, catalog *InstanceTypesCatalog) (*InstanceType, error) {
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

	case "DynamixInstanceClass":
		var spec dynamixInstanceClass
		extractor = &spec

	case "HuaweiCloudInstanceClass":
		var spec huaweiCloudInstanceClass
		extractor = &spec

	case "DVPInstanceClass":
		var spec dvpInstanceClass
		extractor = &spec

	case "D8TestInstanceClass":
		var spec testInstanceClass
		extractor = &spec
	default:
		// Provider-agnostic path: node-controller has no case for this kind (an out-of-tree
		// provider), but any InstanceClass may publish an explicit spec.capacity. Use it
		// instead of failing, so capacity for scale-from-zero works without a per-provider
		// case here. An in-tree provider's case can be dropped once it publishes spec.capacity.
		if it := capacityFromExplicitField(instanceClassSpec); it != nil {
			return it, nil
		}
		return nil, errors.New("Unknown cloud provider")
	}

	data, _ := json.Marshal(instanceClassSpec)
	err := json.Unmarshal(data, extractor)
	if err != nil {
		return nil, err
	}

	return extractor.ExtractCapacity(catalog)
}
