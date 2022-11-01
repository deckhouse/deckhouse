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

type VsphereInstanceClass struct {
	CPU    int `json:"numCPUs"`
	Memory int `json:"memory"`
}

func (vic VsphereInstanceClass) ExtractCapacity() (*Capacity, error) {
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

type AWSInstanceClass struct {
	Capacity     *Capacity `json:"capacity,omitempty"`
	InstanceType string    `json:"instanceType,omitempty"`
}

func (aic AWSInstanceClass) ExtractCapacity() (*Capacity, error) {
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

type AzureInstanceClass struct {
	Capacity    *Capacity `json:"capacity,omitempty"`
	MachineSize string    `json:"machineSize,omitempty"`
}

func (azic AzureInstanceClass) ExtractCapacity() (*Capacity, error) {
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

type GCPInstanceClass struct {
	Capacity    *Capacity `json:"capacity,omitempty"`
	MachineType string    `json:"machineType,omitempty"`
}

func (gic GCPInstanceClass) ExtractCapacity() (*Capacity, error) {
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

type YandexInstanceClass struct {
	Cores  int `json:"cores"`
	Memory int `json:"memory"`
}

func (yic YandexInstanceClass) ExtractCapacity() (*Capacity, error) {
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

type OpenStackInstanceClass struct {
	Capacity   *Capacity `json:"capacity,omitempty"`
	FlavorName string    `json:"flavorName,omitempty"`
}

func (osic OpenStackInstanceClass) ExtractCapacity() (*Capacity, error) {
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
		var spec VsphereInstanceClass
		extractor = &spec

	case "AWSInstanceClass":
		var spec AWSInstanceClass
		extractor = &spec

	case "AzureInstanceClass":
		var spec AzureInstanceClass
		extractor = &spec

	case "GCPInstanceClass":
		var spec GCPInstanceClass
		extractor = &spec

	case "YandexInstanceClass":
		var spec YandexInstanceClass
		extractor = &spec

	case "OpenStackInstanceClass":
		var spec OpenStackInstanceClass
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

	return extractor.ExtractCapacity()
}
