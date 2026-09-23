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

package checks

import (
	"context"
	"fmt"
	"net"
	"time"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

const ImmutableAPIReachableCheckName preflight.CheckName = "immutable-api-reachable"

// MasterAPIEndpoint says how to reach the API port of the first master, and what that stands for.
// The dialable address is a local one — the near end of the bastion tunnel — so without the rest
// of this a failure could only name a loopback port, which tells the reader nothing about which
// machine or which hop is at fault.
type MasterAPIEndpoint struct {
	// Dial is the address to connect to: the local end of the tunnel, or the master itself
	// where no bastion is configured.
	Dial string
	// Master is the address the cloud reported for the machine, as the reader knows it.
	Master string
	// Bastion names the hop, or is empty when the master is reached directly.
	Bastion string
	// Stop closes whatever was opened to make Dial reachable. Never nil.
	Stop func()
}

// masterAPIBudget is how long the master is given to answer its API port. An immutable machine
// boots, downloads its system extensions and starts a control plane before anything listens
// there, and that is minutes rather than seconds — so the budget is generous and the point of the
// check is only to tell "still coming up" from "will never arrive".
var masterAPIBudget = struct {
	attempts int
	wait     time.Duration
	dial     time.Duration
}{attempts: 90, wait: 5 * time.Second, dial: 5 * time.Second}

// ImmutableAPIReachable asks whether the API port of the first master answers at all.
//
// Nothing asked before: a security group that does not allow 6443 from the bastion surfaced as
// the credentials handoff waiting in silence, and the wait says only that the node has not
// finished installing — which is also what it says when the node is fine and the packets are
// being dropped. The two are indistinguishable to the reader and take the same twenty minutes.
//
// resolve is supplied by the bootstrapper, which owns the address and the tunnel; on any other
// cloud bootstrap it is nil and the check does not apply.
func ImmutableAPIReachable(resolve func(context.Context) (*MasterAPIEndpoint, error)) preflight.Check {
	return preflight.Check{
		Name:        ImmutableAPIReachableCheckName,
		Description: "the API port of the first master answers",
		Phase:       preflight.PhasePostInfra,
		// The waiting is inside, against a budget shaped to a machine that is booting.
		// Retrying the whole check on top of that would multiply it.
		Retry:   preflight.NoRetry,
		Timeout: preflight.LongCheckTimeout,
		Run: func(ctx context.Context) (string, error) {
			if resolve == nil {
				return "", preflight.NotApplicable("the master NodeGroup of this cluster does not ask for systemType: Immutable")
			}
			return dialMasterAPI(ctx, resolve)
		},
	}
}

func dialMasterAPI(ctx context.Context, resolve func(context.Context) (*MasterAPIEndpoint, error)) (string, error) {
	endpoint, err := resolve(ctx)
	if err != nil {
		return "", err
	}
	if endpoint == nil {
		return "", preflight.NotApplicable("the address of the first master is not known yet")
	}
	if endpoint.Stop != nil {
		defer endpoint.Stop()
	}

	lastErr := waitForTCP(ctx, endpoint.Dial)
	if lastErr == nil {
		return fmt.Sprintf("the API port of %s answers%s", endpoint.Master, throughBastion(endpoint)), nil
	}

	return "", &preflight.Failure{
		Checked:  fmt.Sprintf("a TCP connection to the API port of %s%s", endpoint.Master, throughBastion(endpoint)),
		Observed: classifyNetworkError(lastErr),
		Expected: "an API port on the first master that accepts connections",
		Fix: fmt.Sprintf(
			"allow the API port to %s from %s in the security group or firewall. Then "+
				"check in the cloud console that the machine finished booting",
			endpoint.Master, bastionOrHere(endpoint),
		),
		Err: lastErr,
	}
}

// waitForTCP dials until something answers or the budget runs out, and returns the last failure.
func waitForTCP(ctx context.Context, address string) error {
	dialer := net.Dialer{Timeout: masterAPIBudget.dial}

	var lastErr error
	for attempt := range masterAPIBudget.attempts {
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err

		if attempt == 0 {
			dhlog.FromContext(ctx).InfoContext(ctx,
				fmt.Sprintf("Waiting for the first master to answer on its API port (%s)", address))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(masterAPIBudget.wait):
		}
	}

	return lastErr
}

func throughBastion(endpoint *MasterAPIEndpoint) string {
	if endpoint.Bastion == "" {
		return ""
	}
	return " through the bastion " + endpoint.Bastion
}

// bastionOrHere names where the connection comes from, which is what the security group has to
// allow.
func bastionOrHere(endpoint *MasterAPIEndpoint) string {
	if endpoint.Bastion == "" {
		return "this host"
	}
	return "the bastion " + endpoint.Bastion
}
