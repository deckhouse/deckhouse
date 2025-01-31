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

package releaseupdater

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

type DeployDelayReason byte

const (
	noDelay             DeployDelayReason = 0
	cooldownDelayReason DeployDelayReason = 1 << iota
	canaryDelayReason
	notificationDelayReason
	outOfWindowReason
	manualApprovalRequiredReason
)

var deployDelayReasonsStr = map[DeployDelayReason]string{
	cooldownDelayReason:          "cooldownDelayReason",
	canaryDelayReason:            "canaryDelayReason",
	notificationDelayReason:      "notificationDelayReason",
	outOfWindowReason:            "outOfWindowReason",
	manualApprovalRequiredReason: "manualApprovalRequiredReason",
}

const (
	cooldownDelayMsg              = "in cooldown"
	canaryDelayReasonMsg          = "postponed"
	waitingManualApprovalTemplate = "waiting for the '%s: \"true\"' annotation"
	outOfWindowMsg                = "waiting for the update window"
	notificationDelayMsg          = "postponed by notification"
)

func (r DeployDelayReason) IsNoDelay() bool {
	return r == noDelay
}

func (r DeployDelayReason) String() string {
	reasons := r.splitReasons()
	if len(reasons) != 0 {
		return strings.Join(reasons, ", ")
	}

	return r.GoString()
}

func (r DeployDelayReason) Message(release v1alpha1.Release, applyTime time.Time) string {
	if r.IsNoDelay() {
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

func (r DeployDelayReason) add(flag DeployDelayReason) DeployDelayReason {
	return r | flag
}

func (r DeployDelayReason) contains(flag DeployDelayReason) bool {
	if flag.IsNoDelay() {
		return r.IsNoDelay()
	}
	return r&flag != 0
}

func (r DeployDelayReason) GoString() string {
	reasons := r.splitReasons()
	if len(reasons) != 0 {
		return strings.Join(reasons, "|")
	}

	return fmt.Sprintf("deployDelayReason(0b%b)", byte(r))
}

func (r DeployDelayReason) splitReasons() []string {
	if r.IsNoDelay() {
		return []string{"noDelay"}
	}

	reasons := make([]string, 0)

	for reason, str := range deployDelayReasonsStr {
		if r.contains(reason) {
			reasons = append(reasons, str)
		}
	}

	slices.Sort(reasons)

	return reasons
}
