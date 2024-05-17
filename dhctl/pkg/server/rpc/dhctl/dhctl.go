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
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

type Service struct {
	pb.UnimplementedDHCTLServer

	podName  string
	cacheDir string
	logc     *slog.Logger
	logb     *slog.Logger
	logd     *slog.Logger
}

func New(podName, cacheDir string, log *slog.Logger) *Service {
	return &Service{
		podName:  podName,
		cacheDir: cacheDir,
		logc:     log.With(slog.String("operation", "check")),
		logb:     log.With(slog.String("operation", "bootstrap")),
		logd:     log.With(slog.String("operation", "destroy")),
	}
}

func (s *Service) shutdown(done <-chan struct{}) {
	go func() {
		<-done
		tomb.Shutdown(0)
	}()
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

	app.SSHPrivateKeys = keysPaths
	app.SSHBastionHost = config.SSHConfig.SSHBastionHost
	app.SSHBastionPort = portToString(config.SSHConfig.SSHBastionPort)
	app.SSHBastionUser = config.SSHConfig.SSHBastionUser
	app.SSHUser = config.SSHConfig.SSHUser
	app.SSHHosts = sshHosts
	app.SSHPort = portToString(config.SSHConfig.SSHPort)
	app.SSHExtraArgs = config.SSHConfig.SSHExtraArgs

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
