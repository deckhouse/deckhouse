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
	"net"
	"time"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	"github.com/deckhouse/lib-dhctl/pkg/retry"
)

// sshWaitAttempts is how long the master is given to answer SSH, one second apart. The machine
// was created moments ago, so most of this budget is boot time.
const sshWaitAttempts = 250

// sshWaitAdvice says what the connection was, and what the four things that break it are.
//
// The preflight checks ask these questions before the infrastructure exists, where they can be
// answered cheaply — but on a cloud cluster the master is created after them, and this wait is
// the first time anything talks to it. When it fails, the reader needs to know which connection
// failed before they can start looking.
func sshWaitAdvice(sess *session.Session) string {
	if sess == nil {
		return ""
	}

	host := sess.Host()
	if sess.Port != "" {
		host = net.JoinHostPort(host, sess.Port)
	}

	advice := fmt.Sprintf("dhctl was connecting to %s as %q", host, sess.User)
	if sess.BastionHost != "" {
		bastionUser := sess.BastionUser
		if bastionUser == "" {
			bastionUser = sess.User
		}
		advice += fmt.Sprintf(", through the bastion %s as %q", sess.BastionHost, bastionUser)
	}

	return advice + ".\nCheck that the machine booted, that its security groups allow 22/TCP from " +
		"this host, that the SSH user exists on it, and that the key the cloud installed is the one " +
		"dhctl is offering (cloud-ssh-key-matches-public-key checks the last one before anything is created)."
}

func WaitForSSHConnectionOnMaster(ctx context.Context, sshClient libcon.SSHClient) error {
	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Wait for SSH on master to become ready", func(ctx context.Context) error {
		availabilityCheck := sshClient.Check()
		_ = dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Connection string", func(ctx context.Context) error {
			dhlog.FromContext(ctx).InfoContext(ctx, availabilityCheck.String(), dhlog.ConnectionString())
			return nil
		})

		if err := availabilityCheck.WithDelaySeconds(1).AwaitAvailability(ctx, retry.NewEmptyParams(
			retry.WithWait(1*time.Second),
			retry.WithAttempts(sshWaitAttempts),
			retry.WithLogger(dhlog.FromContext(ctx)),
		)); err != nil {
			// Four minutes of silence end here, and the message used to be "await master to
			// become available" — which names neither the address, nor the user, nor the bastion
			// the connection goes through, and so reads as "the machine never booted" for the
			// three causes that are not that.
			return fmt.Errorf("%w\n\n%s", err, sshWaitAdvice(sshClient.Session()))
		}
		return nil
	})
}
