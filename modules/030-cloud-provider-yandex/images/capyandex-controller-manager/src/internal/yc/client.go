package yc

import (
	"context"
	"fmt"

	compute "github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	iamkey "github.com/yandex-cloud/go-sdk/iamkey"
	ycsdk "github.com/yandex-cloud/go-sdk"
)

type MachineSpec struct {
	Name              string
	FolderID          string
	Zone              string
	PlatformID        string
	Cores             int64
	CoreFraction      int64
	MemoryBytes       int64
	GPUs              int64
	BootDiskType      string
	BootDiskSizeBytes int64
	BootDiskImageID   string
	Hostname          string
	NetworkType       string
	Preemptible       bool
	Labels            map[string]string
	Metadata          map[string]string
	NetworkInterfaces []NetworkInterfaceSpec
}

type NetworkInterfaceSpec struct {
	SubnetID               string
	AssignPublicIPAddress  bool
}

type Client struct {
	sdk *ycsdk.SDK
}

func NewClient(ctx context.Context, serviceAccountJSON string) (*Client, error) {
	key, err := iamkey.ReadFromJSONBytes([]byte(serviceAccountJSON))
	if err != nil {
		return nil, fmt.Errorf("parse YC service account json: %w", err)
	}

	creds, err := ycsdk.ServiceAccountKey(key)
	if err != nil {
		return nil, fmt.Errorf("create YC credentials: %w", err)
	}

	sdk, err := ycsdk.Build(ctx, ycsdk.Config{
		Credentials: creds,
	})
	if err != nil {
		return nil, fmt.Errorf("build YC SDK: %w", err)
	}

	return &Client{sdk: sdk}, nil
}

func (c *Client) GetInstance(ctx context.Context, instanceID string) (*compute.Instance, error) {
	instance, err := c.sdk.Compute().Instance().Get(ctx, &compute.GetInstanceRequest{
		InstanceId: instanceID,
		View:       compute.InstanceView_FULL,
	})
	if err != nil {
		return nil, err
	}
	return instance, nil
}

func (c *Client) FindInstanceByName(ctx context.Context, folderID, name string) (*compute.Instance, error) {
	pageToken := ""
	for {
		resp, err := c.sdk.Compute().Instance().List(ctx, &compute.ListInstancesRequest{
			FolderId:  folderID,
			PageSize:  1000,
			PageToken: pageToken,
			Filter:    fmt.Sprintf("name = %q", name),
		})
		if err != nil {
			return nil, err
		}

		for _, instance := range resp.GetInstances() {
			if instance.GetName() == name {
				return instance, nil
			}
		}

		pageToken = resp.GetNextPageToken()
		if pageToken == "" {
			return nil, nil
		}
	}
}

func (c *Client) CreateInstance(ctx context.Context, spec MachineSpec) (*compute.Instance, error) {
	request := &compute.CreateInstanceRequest{
		FolderId:   spec.FolderID,
		Name:       spec.Name,
		Hostname:   spec.Hostname,
		ZoneId:     spec.Zone,
		PlatformId: spec.PlatformID,
		Labels:     spec.Labels,
		Metadata:   spec.Metadata,
		ResourcesSpec: &compute.ResourcesSpec{
			Cores:        spec.Cores,
			CoreFraction: spec.CoreFraction,
			Memory:       spec.MemoryBytes,
			Gpus:         spec.GPUs,
		},
		BootDiskSpec: &compute.AttachedDiskSpec{
			AutoDelete: true,
			Disk: &compute.AttachedDiskSpec_DiskSpec_{
				DiskSpec: &compute.AttachedDiskSpec_DiskSpec{
					TypeId: spec.BootDiskType,
					Size:   spec.BootDiskSizeBytes,
					Source: &compute.AttachedDiskSpec_DiskSpec_ImageId{
						ImageId: spec.BootDiskImageID,
					},
				},
			},
		},
		NetworkInterfaceSpecs: toYCNetworkInterfaces(spec.NetworkInterfaces),
	}

	if spec.Preemptible {
		request.SchedulingPolicy = &compute.SchedulingPolicy{Preemptible: true}
	}

	if spec.NetworkType != "" {
		request.NetworkSettings = &compute.NetworkSettings{Type: mapNetworkType(spec.NetworkType)}
	}

	op, err := c.sdk.WrapOperation(c.sdk.Compute().Instance().Create(ctx, request))
	if err != nil {
		return nil, err
	}
	if err := op.Wait(ctx); err != nil {
		return nil, err
	}
	resp, err := op.Response()
	if err != nil {
		return nil, err
	}
	instance, ok := resp.(*compute.Instance)
	if !ok {
		return nil, fmt.Errorf("unexpected create response type %T", resp)
	}
	return instance, nil
}

func (c *Client) DeleteInstance(ctx context.Context, instanceID string) error {
	op, err := c.sdk.WrapOperation(c.sdk.Compute().Instance().Delete(ctx, &compute.DeleteInstanceRequest{
		InstanceId: instanceID,
	}))
	if err != nil {
		return err
	}
	return op.Wait(ctx)
}

func toYCNetworkInterfaces(items []NetworkInterfaceSpec) []*compute.NetworkInterfaceSpec {
	result := make([]*compute.NetworkInterfaceSpec, 0, len(items))
	for _, item := range items {
		ni := &compute.NetworkInterfaceSpec{
			SubnetId:             item.SubnetID,
			PrimaryV4AddressSpec: &compute.PrimaryAddressSpec{},
		}
		if item.AssignPublicIPAddress {
			ni.PrimaryV4AddressSpec.OneToOneNatSpec = &compute.OneToOneNatSpec{
				IpVersion: compute.IpVersion_IPV4,
			}
		}
		result = append(result, ni)
	}
	return result
}

func mapNetworkType(networkType string) compute.NetworkSettings_Type {
	switch networkType {
	case "SOFTWARE_ACCELERATED", "SoftwareAccelerated":
		return compute.NetworkSettings_SOFTWARE_ACCELERATED
	case "STANDARD", "Standard":
		return compute.NetworkSettings_STANDARD
	default:
		return compute.NetworkSettings_TYPE_UNSPECIFIED
	}
}
