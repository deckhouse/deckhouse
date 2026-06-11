// Copyright 2026 Flant JSC
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

// ConfigExtender returns the recommended config for the requested extension
// kind. Runs in commander mode unconditionally: the request proto has no
// opts, and parsing here is non-mutating preview-only.
func (s *Service) ConfigExtender(
	ctx context.Context,
	request *pb.ConfigExtenderRequest,
) (*pb.ConfigExtenderResponse, error) {
	if request.Kind != pb.ConfigExtensionKind_CONFIG_EXTENSION_KIND_CNI {
		return &pb.ConfigExtenderResponse{}, nil
	}

	metaConfig, err := config.ParseConfigFromData(
		ctx,
		request.Config,
		config.DummyPreparatorProvider(),
		nil,
		config.ValidateOptionCommanderMode(true),
	)
	if err != nil {
		return &pb.ConfigExtenderResponse{Err: err.Error()}, nil
	}

	analysis, err := config.AnalyzeCNIBootstrap(ctx, metaConfig, nil)
	if err != nil {
		return &pb.ConfigExtenderResponse{Err: err.Error()}, nil
	}

	if analysis.ModuleConfig == nil || analysis.ModuleConfig.Recommended == nil {
		return &pb.ConfigExtenderResponse{}, nil
	}

	recommended, err := json.Marshal(analysis.ModuleConfig.Recommended)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal recommended ModuleConfig: %s", err)
	}

	return &pb.ConfigExtenderResponse{Config: string(recommended)}, nil
}
