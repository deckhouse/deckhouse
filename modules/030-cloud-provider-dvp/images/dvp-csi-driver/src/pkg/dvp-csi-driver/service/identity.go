/*
Copyright 2025 Flant JSC

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

package service

import (
	"context"

	dvpapi "dvp-common/api"

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
	dvpCloudAPI   *dvpapi.DVPCloudAPI
}

func NewIdentity(
	driverName string,
	vendorVersion string,
	dvpCloudAPI *dvpapi.DVPCloudAPI,
) *IdentityService {
	return &IdentityService{
		driverName:    driverName,
		vendorVersion: vendorVersion,
		dvpCloudAPI:   dvpCloudAPI,
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

// Probe checks the state of the connection
func (i *IdentityService) Probe(
	ctx context.Context,
	_ *csi.ProbeRequest,
) (*csi.ProbeResponse, error) {
	err := i.dvpCloudAPI.PortalService.Test(ctx)
	if err != nil {
		klog.Errorf("Could not get connection %v", err)
		return nil, status.Error(
			codes.FailedPrecondition,
			"Could not get connection to DVP",
		)
	}
	return &csi.ProbeResponse{Ready: &wrappers.BoolValue{Value: true}}, nil
}
