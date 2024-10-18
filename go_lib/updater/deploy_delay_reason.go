/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package updater

import (
	"fmt"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

type deployDelayReason byte

const (
	noDelay             deployDelayReason = 0
	cooldownDelayReason deployDelayReason = 1 << iota
	canaryDelayReason
	notificationDelayReason
	outOfWindowReason
	manualApprovalRequiredReason
)

const (
	cooldownDelayMsg              = "in cooldown"
	canaryDelayReasonMsg          = "postponed by canary process"
	waitingManualApprovalTemplate = "waiting for the '%s: \"true\"' annotation"
	outOfWindowMsg                = "waiting for the update window"
	notificationDelayMsg          = "postponed by notification"
)

func (r deployDelayReason) String() string {
	reasons := r.splitReasons()
	if len(reasons) != 0 {
		return strings.Join(reasons, " and ")
	}

	return r.GoString()
}

func (r deployDelayReason) Message(release v1alpha1.Release, applyTime time.Time) string {
	if r == noDelay {
		return r.String()
	}

	var (
		reasons []string
		b       strings.Builder
	)
	b.WriteString("Release is ")
	if r.contains(cooldownDelayReason) {
		reasons = append(reasons, cooldownDelayMsg)
	}
	if r.contains(canaryDelayReason) {
		reasons = append(reasons, canaryDelayReasonMsg)
	}
	if r.contains(notificationDelayReason) {
		reasons = append(reasons, notificationDelayMsg)
	}
	if r.contains(outOfWindowReason) {
		reasons = append(reasons, outOfWindowMsg)
	}
	if r.contains(manualApprovalRequiredReason) {
		waitingManualApprovalMsg := fmt.Sprintf(waitingManualApprovalTemplate, v1alpha1.GetReleaseApprovalAnnotation(release))
		reasons = append(reasons, waitingManualApprovalMsg)
	}
	if len(reasons) != 0 {
		b.WriteString(strings.Join(reasons, ", "))
		if applyTime.IsZero() {
			return b.String()
		}
		if r.contains(manualApprovalRequiredReason) {
			b.WriteString(". After approval the release will be delayed")
		}
		b.WriteString(" until ")
		b.WriteString(applyTime.Format(time.RFC822))
		return b.String()
	}

	return r.GoString()
}

func (r deployDelayReason) add(flag deployDelayReason) deployDelayReason {
	return r | flag
}

func (r deployDelayReason) contains(flag deployDelayReason) bool {
	if flag == noDelay {
		return r == noDelay
	}
	return r&flag != 0
}

func (r deployDelayReason) GoString() string {
	reasons := r.splitReasons()
	if len(reasons) != 0 {
		return strings.Join(reasons, "|")
	}

	return fmt.Sprintf("deployDelayReason(0b%b)", byte(r))
}

func (r deployDelayReason) splitReasons() (reasons []string) {
	if r == noDelay {
		return []string{"noDelay"}
	}

	if r.contains(cooldownDelayReason) {
		reasons = append(reasons, "cooldownDelayReason")
	}

	if r.contains(canaryDelayReason) {
		reasons = append(reasons, "canaryDelayReason")
	}

	if r.contains(notificationDelayReason) {
		reasons = append(reasons, "notificationDelayReason")
	}

	if r.contains(outOfWindowReason) {
		reasons = append(reasons, "outOfWindowReason")
	}

	if r.contains(manualApprovalRequiredReason) {
		reasons = append(reasons, "manualApprovalRequiredReason")
	}

	return reasons
}
