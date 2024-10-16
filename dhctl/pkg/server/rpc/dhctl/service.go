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

package dhctl

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"google.golang.org/grpc"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

var logTypeDHCTL = slog.String("type", "dhctl")

type Service struct {
	pb.UnimplementedDHCTLServer

	podName  string
	cacheDir string
}

func New(podName, cacheDir string) *Service {
	return &Service{
		podName:  podName,
		cacheDir: cacheDir,
	}
}

func (s *Service) shutdown(done <-chan struct{}) {
	go func() {
		<-done
		tomb.Shutdown(0)
	}()
}

func operationCtx(server grpc.ServerStream) context.Context {
	ctx := server.Context()

	var operation string
	switch server.(type) {
	case pb.DHCTL_CheckServer:
		operation = "check"
	case pb.DHCTL_BootstrapServer:
		operation = "bootstrap"
	case pb.DHCTL_ConvergeServer:
		operation = "converge"
	case pb.DHCTL_DestroyServer:
		operation = "destroy"
	case pb.DHCTL_AbortServer:
		operation = "abort"
	case pb.DHCTL_CommanderAttachServer:
		operation = "commander/attach"
	case pb.DHCTL_CommanderDetachServer:
		operation = "commander/detach"
	default:
		operation = "unknown"
	}
	go func() {
		<-ctx.Done()
		tomb.Shutdown(0)
	}()
	return logger.ToContext(
		ctx,
		logger.L(ctx).With(slog.String("operation", operation)),
	)
}

func onCheckResult(checkRes *check.CheckResult) error {
	printableCheckRes := *checkRes
	printableCheckRes.StatusDetails.TerraformPlan = nil

	printableCheckResDump, err := json.MarshalIndent(printableCheckRes, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to encode check result json: %w", err)
	}

	_ = log.Process("default", "Check result", func() error {
		log.InfoF("%s\n", printableCheckResDump)
		return nil
	})

	return nil
}

func portToString(p *int32) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(int(*p))
}

func errToString(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
