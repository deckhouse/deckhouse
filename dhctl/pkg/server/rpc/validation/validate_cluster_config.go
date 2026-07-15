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
	"strings"

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
	// validated. An external provider (e.g. DVP) needs its OCI bundle delivered
	// first, and the only registry access here is registry_config: when it is
	// absent there is nothing to validate with, so skip — the operation
	// revalidates after reading the registry from the cluster.
	provider := clusterConfig.Cloud.Provider
	needBundle := provider != "" && !config.ProviderBundledInCandi(provider, s.globalOptions)

	if needBundle && strings.TrimSpace(request.RegistryConfig) == "" {
		return &pb.ValidateProviderSpecificClusterConfigResponse{}, nil
	}

	// A bundle delivery failure is reported the same way as a validation
	// failure: in the response's Err, not as a transport error.
	var validationErr error
	if needBundle {
		validationErr = s.ensureProviderBundle(ctx, provider, request.RegistryConfig)
	}
	if validationErr == nil {
		validationErr = config.ValidateProviderSpecificClusterConfiguration(
			request.Config, clusterConfig, s.schemaStore,
			optionsFromRequest(request.Opts)...,
		)
	}

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
