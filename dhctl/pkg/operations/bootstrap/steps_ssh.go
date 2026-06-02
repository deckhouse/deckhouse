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

// TODO structure these functions into classes
// TODO move states saving to operations/bootstrap/state.go

package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func readRemoteFile(ctx context.Context, nodeInterface libcon.Interface, path string) (string, error) {
	cmd := nodeInterface.Command("cat", path)
	cmd.Sudo(ctx)
	cmd.WithTimeout(10 * time.Second)

	stdout, stderr, err := cmd.Output(ctx)
	if err != nil {
		return "", fmt.Errorf("read remote file %s: %w; stderr: %s", path, err, string(stderr))
	}

	output := string(stdout)
	// Sudo-wrapped commands prefix their stdout with the SUDO-SUCCESS marker;
	// strip everything up to and including the last occurrence so we keep only
	// the actual file payload. For non-sudo paths the marker is absent and
	// output stays untouched.
	if idx := strings.LastIndex(output, "SUDO-SUCCESS"); idx >= 0 {
		output = output[idx+len("SUDO-SUCCESS"):]
	}

	return strings.TrimSpace(output), nil
}

func readRemoteFileWithRetry(ctx context.Context, nodeInterface libcon.Interface, path string) (string, error) {
	extLogger := log.ExternalLoggerProvider(log.GetDefaultLogger())
	p := retry.NewEmptyParams(
		retry.WithName("Read remote file %s", path),
		retry.WithAttempts(5),
		retry.WithWait(3*time.Second),
		retry.WithLogger(extLogger()),
	)
	var value string
	err := retry.NewLoopWithParams(p).
		RunContext(ctx, func() error {
			v, err := readRemoteFile(ctx, nodeInterface, path)
			if err != nil {
				return err
			}
			value = v
			return nil
		})
	if err != nil {
		return "", err
	}
	return value, nil
}

func WaitForSSHConnectionOnMaster(ctx context.Context, sshClient libcon.SSHClient) error {
	return log.ProcessCtx(ctx, "bootstrap", "Wait for SSH on Master become Ready", func(ctx context.Context) error {
		availabilityCheck := sshClient.Check()
		_ = log.ProcessCtx(ctx, "default", "Connection string", func(ctx context.Context) error {
			log.InfoLn(availabilityCheck.String())
			return nil
		})

		extLogger := log.ExternalLoggerProvider(log.GetDefaultLogger())

		if err := availabilityCheck.WithDelaySeconds(1).AwaitAvailability(ctx, retry.NewEmptyParams(
			retry.WithWait(5*time.Second),
			retry.WithAttempts(50),
			retry.WithLogger(extLogger()),
		)); err != nil {
			return fmt.Errorf("await master to become available: %v", err)
		}
		return nil
	})
}
