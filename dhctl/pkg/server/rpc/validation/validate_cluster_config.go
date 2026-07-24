// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
)

//nolint:musttag
func (s *Service) ValidateClusterConfig(
	_ context.Context,
	request *pb.ValidateClusterConfigRequest,
) (*pb.ValidateClusterConfigResponse, error) {
	var errResponse string

	clusterConfig, err := config.ValidateClusterConfiguration(request.Config, s.schemaStore, optionsFromRequest(request.Opts)...)
	if err != nil {
		if errResponse, err = errorToResponse(err); err != nil {
			return nil, status.Errorf(codes.Internal, "%s", err)
		}
	}

	var clusterConfigBytes []byte
	clusterConfigBytes, err = json.Marshal(clusterConfig)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshalling cluster Config: %s", err)
	}
	clusterConfigResponse := string(clusterConfigBytes)

	return &pb.ValidateClusterConfigResponse{
		ClusterConfig: clusterConfigResponse,
		Err:           errResponse,
	}, nil
}

//nolint:musttag
func (s *Service) ValidateProviderSpecificClusterConfig(
	ctx context.Context,
	request *pb.ValidateProviderSpecificClusterConfigRequest,
) (*pb.ValidateProviderSpecificClusterConfigResponse, error) {
	var clusterConfig config.ClusterConfig
	if err := json.Unmarshal([]byte(request.ClusterConfig), &clusterConfig); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshalling cluster Config: %s", err)
	}

	// In-tree providers ship their schemas in the image's candi and are always
	// validated here. An external provider (e.g. DVP) validates against a schema
	// that lives in its OCI bundle, which this stateless request cannot fetch
	// (no cluster, no registry); skip it — the actual operation revalidates
	// after reading the registry from the target cluster.
	provider := clusterConfig.Cloud.Provider
	if provider != "" && !config.ProviderBundledInCandi(provider, s.globalOptions) {
		return &pb.ValidateProviderSpecificClusterConfigResponse{}, nil
	}

	validationErr := config.ValidateProviderSpecificClusterConfiguration(
		request.Config, clusterConfig, s.schemaStore,
		optionsFromRequest(request.Opts)...,
	)

	errResponse, err := errorToResponse(validationErr)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", err)
	}
	return &pb.ValidateProviderSpecificClusterConfigResponse{Err: errResponse}, nil
}

func (s *Service) ValidateStaticClusterConfig(
	_ context.Context,
	request *pb.ValidateStaticClusterConfigRequest,
) (*pb.ValidateStaticClusterConfigResponse, error) {
	err := config.ValidateStaticClusterConfiguration(request.Config, s.schemaStore, optionsFromRequest(request.Opts)...)
	errResponse, err := errorToResponse(err)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", err)
	}

	return &pb.ValidateStaticClusterConfigResponse{Err: errResponse}, nil
}
