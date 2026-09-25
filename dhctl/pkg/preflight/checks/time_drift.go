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
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	libcon "github.com/deckhouse/lib-connection/pkg"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

const maxTimeDriftSeconds int64 = 600 // 10 minutes

var timestampRegexp = regexp.MustCompile(`^(\d+)$`)

type TimeDriftCheck struct {
	// NodeInterface is resolved when the check runs, not when its suite is built: see
	// NodeInterfaceFunc.
	NodeInterface NodeInterfaceFunc
}

const TimeDriftCheckName preflight.CheckName = "time-drift"

func (TimeDriftCheck) Description() string {
	return "the node clock matches the clock on this host"
}

func (TimeDriftCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (TimeDriftCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c TimeDriftCheck) Run(ctx context.Context) (string, error) {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}

	// The error used to be discarded and the check reported as passed: a node where `date` could
	// not be run at all — no shell, no sudo, a broken connection — was indistinguishable from one
	// whose clock is correct, on a check whose whole point is that a wrong clock breaks the
	// certificates issued during bootstrap.
	remote, err := getRemoteTimeStamp(ctx, nodeInterface)
	if err != nil {
		return "", scriptFailure("read the clock", nodeInterface, nil, err)
	}
	local := time.Now().Unix()

	diff := remote - local
	if diff < 0 {
		diff = -diff
	}
	if diff > maxTimeDriftSeconds {
		// A clock is not going to correct itself between two attempts of the same check.
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the clock on %s against this host", hostPhrase(nodeInterface)),
			Observed: fmt.Sprintf("%s on the node, %s here: %s apart", time.Unix(remote, 0).Format(time.RFC3339), time.Unix(local, 0).Format(time.RFC3339), time.Duration(diff)*time.Second),
			Expected: fmt.Sprintf("the two clocks within %s of each other", time.Duration(maxTimeDriftSeconds)*time.Second),
			Fix:      "enable NTP or chrony on the node and sync its clock",
		})
	}

	return fmt.Sprintf("the clock on %s is within %s of this host", hostPhrase(nodeInterface), time.Duration(diff)*time.Second), nil
}

func getRemoteTimeStamp(ctx context.Context, nodeInterface libcon.Interface) (int64, error) {
	cmd := nodeInterface.Command("date", "+%s")
	dateOutput, _, err := cmd.Output(ctx)
	if err != nil {
		return 0, fmt.Errorf("execute the date command on the node: %w", err)
	}
	out := strings.TrimSpace(string(dateOutput))
	match := timestampRegexp.FindStringSubmatch(out)
	if match == nil {
		return 0, errors.New("the node printed something other than a Unix timestamp")
	}
	timeStamp, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse the timestamp the node printed: %w", err)
	}
	return timeStamp, nil
}

func TimeDrift(nodeInterface NodeInterfaceFunc) preflight.Check {
	check := TimeDriftCheck{NodeInterface: nodeInterface}
	return preflight.Check{
		Name:        TimeDriftCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}
