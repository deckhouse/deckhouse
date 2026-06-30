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
	"time"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
)

func WaitForSSHConnectionOnMaster(ctx context.Context, sshClient libcon.SSHClient) error {
	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Wait for SSH on master to become ready", func(ctx context.Context) error {
		availabilityCheck := sshClient.Check()
		_ = dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Connection string", func(ctx context.Context) error {
			dhlog.FromContext(ctx).InfoContext(ctx, availabilityCheck.String(), dhlog.ConnectionString())
			return nil
		})

		if err := availabilityCheck.WithDelaySeconds(1).AwaitAvailability(ctx, retry.NewEmptyParams(
			retry.WithWait(1*time.Second),
			retry.WithAttempts(250),
			retry.WithLogger(dhlog.NewLibdhctlAdapter(ctx)),
		)); err != nil {
			return fmt.Errorf("await master to become available: %v", err)
		}
		return nil
	})
}
