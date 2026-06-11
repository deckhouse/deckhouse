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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
)

func (s *Service) ValidateResources(
	ctx context.Context,
	request *pb.ValidateResourcesRequest,
) (*pb.ValidateResourcesResponse, error) {
	errs := &config.ValidationError{}
	for _, v := range config.ResourcesPipeline {
		errs.Merge(v(ctx, request.Config, s.schemaStore, optionsFromRequest(request.Opts)...))
	}

	errResponse, convErr := errorToResponse(errs.ErrorOrNil())
	if convErr != nil {
		return nil, status.Errorf(codes.Internal, "%s", convErr)
	}
	return &pb.ValidateResourcesResponse{Err: errResponse}, nil
}
