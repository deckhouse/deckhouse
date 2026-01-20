// Copyright 2025 Flant JSC
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

	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
)

const maxTimeDriftSeconds int64 = 600 // 10 minutes

var timestampRegexp = regexp.MustCompile(`^(\d+)$`)

type TimeDriftCheck struct {
	Node node.Interface
}

const TimeDriftCheckName preflightnew.CheckName = "time-drift"

func (TimeDriftCheck) Description() string {
	return "server time drift has an acceptable value"
}

func (TimeDriftCheck) Phase() preflightnew.Phase {
	return preflightnew.PhasePostInfra
}

func (TimeDriftCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.DefaultRetryPolicy
}

func (TimeDriftCheck) Enabled() bool {
	return true
}

func (c TimeDriftCheck) Run(ctx context.Context) error {
	remote, err := getRemoteTimeStamp(ctx, c.Node)
	if err != nil {
		// keep behaviour: log and skip failure
		return nil
	}
	local := time.Now().Unix()
	diff := remote - local
	if diff < 0 {
		diff = -diff
	}
	if diff > maxTimeDriftSeconds {
		localTime := time.Unix(local, 0).Format(time.RFC3339)
		remoteTime := time.Unix(remote, 0).Format(time.RFC3339)
		drift := time.Duration(diff) * time.Second
		// intentionally no error, only informational logs
		_ = fmt.Sprintf("time drift between local (%s) and remote server (%s) is too high: (%s)\n", localTime, remoteTime, drift.String())
	}
	return nil
}

func getRemoteTimeStamp(ctx context.Context, sshCl node.Interface) (int64, error) {
	cmd := sshCl.Command("date", "+%s")
	dateOutput, _, err := cmd.Output(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to execute date command: %w", err)
	}
	out := strings.TrimSpace(string(dateOutput))
	match := timestampRegexp.FindStringSubmatch(out)
	if match == nil {
		return 0, errors.New("invalid timestamp format received")
	}
	timeStamp, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse timestamp: %w", err)
	}
	return timeStamp, nil
}

func TimeDrift(nodeInterface node.Interface) preflightnew.Check {
	check := TimeDriftCheck{Node: nodeInterface}
	return preflightnew.Check{
		Name:        TimeDriftCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Enabled:     check.Enabled,
		Run:         check.Run,
	}
}
