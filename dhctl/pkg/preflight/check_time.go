// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
)

const maxTimeDrift int64 = 600 // 10 minutes
var timestampRegexp = regexp.MustCompile(`^(\d+)$`)

func getLocalTimeStamp() int64 {
	return time.Now().Unix()
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

func (pc *Checker) CheckTimeDrift(ctx context.Context) error {
	if app.PreflightSkipTimeDrift {
		log.InfoLn("Checking Time Drift was skipped (via skip flag)")
		return nil
	}

	remoteTimeStamp, err := getRemoteTimeStamp(ctx, pc.nodeInterface)
	if err != nil {
		log.InfoF("Checking Time Drift was skipped, check cannot be performed: %v\n", err)
		return nil
	}
	localTimeStamp := getLocalTimeStamp()

	timeDrift := remoteTimeStamp - localTimeStamp
	if timeDrift < 0 {
		timeDrift = -timeDrift
	}

	if timeDrift > maxTimeDrift {
		localTime := time.Unix(localTimeStamp, 0).Format(time.RFC3339)
		remoteTime := time.Unix(remoteTimeStamp, 0).Format(time.RFC3339)
		driftDuration := time.Duration(timeDrift) * time.Second
		log.ErrorF("time drift between local (%s) and remote server (%s) is too high: (%s)\n", localTime, remoteTime, driftDuration.String())
		log.InfoLn("please make sure the time on the remote server is correct")
	}
	return nil
}
