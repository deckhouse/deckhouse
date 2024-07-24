/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package service

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

type IdentityService struct {
	csi.UnimplementedIdentityServer
	driverName    string
	vendorVersion string
}

func NewIdentity(
	driverName string,
	vendorVersion string,
) *IdentityService {
	return &IdentityService{
		driverName:    driverName,
		vendorVersion: vendorVersion,
	}
}

// GetPluginInfo returns the vendor name and version
func (i *IdentityService) GetPluginInfo(
	ctx context.Context,
	req *csi.GetPluginInfoRequest,
) (*csi.GetPluginInfoResponse, error) {
	return &csi.GetPluginInfoResponse{
		Name:          i.driverName,
		VendorVersion: i.vendorVersion,
	}, nil
}

// GetPluginCapabilities declares the plugins capabilities
func (i *IdentityService) GetPluginCapabilities(
	context.Context,
	*csi.GetPluginCapabilitiesRequest,
) (*csi.GetPluginCapabilitiesResponse, error) {
	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_VolumeExpansion_{
					VolumeExpansion: &csi.PluginCapability_VolumeExpansion{
						Type: csi.PluginCapability_VolumeExpansion_ONLINE,
					},
				},
			},
		},
	}, nil
}

// Probe checks the state of the connection to ovirt-engine
func (i *IdentityService) Probe(
	ctx context.Context,
	_ *csi.ProbeRequest,
) (*csi.ProbeResponse, error) {
	// TODO: Implement probe to dynamix
	var err error
	err = nil
	if err != nil {
		klog.Errorf("Could not get connection %v", err)
		return nil, status.Error(
			codes.FailedPrecondition,
			"Could not get connection to dynamix",
		)
	}
	return &csi.ProbeResponse{Ready: &wrappers.BoolValue{Value: true}}, nil
}
