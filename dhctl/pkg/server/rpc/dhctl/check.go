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
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Service) Check(server pb.DHCTL_CheckServer) error {
	// todo: support task cancellation messages
	r, err := server.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return status.Errorf(codes.Internal, "receiving message: %s", err)
	}

	request := r.GetStart()
	if request == nil {
		return status.Errorf(codes.Unimplemented, "message not supported")
	}

	result, err := s.check(server.Context(), request, &logWriter{server: server})
	if err != nil {
		return err
	}

	err = server.Send(&pb.CheckResponse{
		Message: &pb.CheckResponse_Result{
			Result: result,
		},
	})
	if err != nil {
		return status.Errorf(codes.Internal, "sending message: %s", err)
	}
	return nil
}

func (s *Service) check(
	ctx context.Context,
	request *pb.CheckStart,
	logWriter io.Writer,
) (*pb.CheckResult, error) {
	// set global variables from options
	log.InitLoggerWithOptions("pretty", log.LoggerOptions{
		OutStream: logWriter,
		Width:     int(request.Options.LogWidth),
	})
	app.SanityCheck = request.Options.SanityCheck
	app.UseTfCache = app.UseStateCacheYes
	app.ResourcesTimeout = request.Options.ResourcesTimeout.AsDuration()
	app.DeckhouseTimeout = request.Options.DeckhouseTimeout.AsDuration()

	// parse connection config
	connectionConfig, err := config.ParseConnectionConfig(
		request.ConnectionConfig,
		config.NewSchemaStore(),
		config.ValidateOptionCommanderMode(request.Options.CommanderMode),
		config.ValidateOptionStrictUnmarshal(request.Options.CommanderMode),
		config.ValidateOptionValidateExtensions(request.Options.CommanderMode),
	)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "parsing connection config: %s", err)
	}

	// parse meta config
	metaConfig, err := config.ParseConfigFromData(
		combineYAMLs(request.ClusterConfig, request.ProviderSpecificClusterConfig),
		config.ValidateOptionCommanderMode(request.Options.CommanderMode),
		config.ValidateOptionStrictUnmarshal(request.Options.CommanderMode),
		config.ValidateOptionValidateExtensions(request.Options.CommanderMode),
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "parsing meta config: %s", err)
	}

	// init dhctl cache
	cachePath := metaConfig.CachePath()
	var initialState phases.DhctlState
	if request.State != "" {
		err = json.Unmarshal([]byte(request.State), &initialState)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "unmarshalling dhctl state: %s", err)
		}
	}
	err = cache.InitWithOptions(
		cachePath,
		cache.CacheOptions{InitialState: initialState, ResetInitialState: true},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "initializing cache at %s: %s", cachePath, err)
	}

	// preparse ssh client
	sshClient, err := prepareSSHClient(connectionConfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer sshClient.Stop()

	// check cluster state
	checker := check.NewChecker(&check.Params{
		SSHClient:     sshClient,
		StateCache:    cache.Global(),
		CommanderMode: request.Options.CommanderMode,
		CommanderModeParams: commander.NewCommanderModeParams(
			[]byte(request.ClusterConfig),
			[]byte(request.ProviderSpecificClusterConfig),
		),
		TerraformContext: terraform.NewTerraformContext(),
	})

	result, err := checker.Check(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "checking cluster state: %s", err)
	}

	resultString, err := json.Marshal(result)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshalling check result: %s", err)
	}

	return &pb.CheckResult{Result: string(resultString)}, nil
}

func prepareSSHClient(config *config.ConnectionConfig) (*ssh.Client, error) {
	keysPaths := make([]string, 0, len(config.SSHConfig.SSHAgentPrivateKeys))
	for _, key := range config.SSHConfig.SSHAgentPrivateKeys {
		keyPath, err := writeTempFile([]byte(strings.TrimSpace(key.Key) + "\n"))
		if err != nil {
			return nil, fmt.Errorf("failed to write ssh private key: %w", err)
		}
		keysPaths = append(keysPaths, keyPath)
	}
	normalizedKeysPaths, err := app.ParseSSHPrivateKeyPaths(keysPaths)
	if err != nil {
		return nil, fmt.Errorf("error parsing ssh agent private keys %v: %w", normalizedKeysPaths, err)
	}
	keys := make([]session.AgentPrivateKey, 0, len(normalizedKeysPaths))
	for i, key := range normalizedKeysPaths {
		keys = append(keys, session.AgentPrivateKey{
			Key:        key,
			Passphrase: config.SSHConfig.SSHAgentPrivateKeys[i].Passphrase,
		})
	}

	var sshHosts []string
	if len(config.SSHHosts) > 0 {
		for _, h := range config.SSHHosts {
			sshHosts = append(sshHosts, h.Host)
		}
	} else {
		mastersIPs, err := bootstrap.GetMasterHostsIPs()
		if err != nil {
			return nil, err
		}
		sshHosts = mastersIPs
	}

	sess := session.NewSession(session.Input{
		User:           config.SSHConfig.SSHUser,
		Port:           portToString(config.SSHConfig.SSHPort),
		BastionHost:    config.SSHConfig.SSHBastionHost,
		BastionPort:    portToString(config.SSHConfig.SSHBastionPort),
		BastionUser:    config.SSHConfig.SSHBastionUser,
		ExtraArgs:      config.SSHConfig.SSHExtraArgs,
		AvailableHosts: sshHosts,
	})

	sshClient, err := ssh.NewClient(sess, keys).Start()
	if err != nil {
		return nil, err
	}

	return sshClient, nil
}

func writeTempFile(data []byte) (string, error) {
	f, err := os.CreateTemp("", "*")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}

	_, err = f.Write(data)
	if err != nil {
		return "", fmt.Errorf("writing temp file: %w", err)
	}

	return f.Name(), nil
}

type logWriter struct {
	server pb.DHCTL_CheckServer
}

func (w *logWriter) Write(p []byte) (int, error) {
	err := w.server.Send(&pb.CheckResponse{
		Message: &pb.CheckResponse_Logs{
			Logs: &pb.Logs{
				Logs: p,
			},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("writing check logs: %w", err)
	}
	return len(p), nil
}

func portToString(p *int32) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(int(*p))
}

func combineYAMLs(yamls ...string) string {
	var res string
	for _, yaml := range yamls {
		if yaml == "" {
			continue
		}

		if res != "" {
			res += "---\n"
		}

		res = res + strings.TrimSpace(yaml) + "\n"
	}

	return res
}
