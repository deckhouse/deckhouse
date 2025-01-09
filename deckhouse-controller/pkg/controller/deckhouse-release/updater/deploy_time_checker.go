/*
Copyright 2022 Flant JSC

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

package d8updater

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/go_lib/updater"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type DeployTimeChecker struct {
	releaseNotifier *ReleaseNotifier

	settings *updater.Settings

	now                   time.Time
	deckhousePodReadyFunc func(ctx context.Context) bool

	logger *log.Logger
}

func NewDeployTimeChecker(dc dependency.Container, settings *updater.Settings, deckhousePodReadyFunc func(ctx context.Context) bool, logger *log.Logger) *DeployTimeChecker {
	return &DeployTimeChecker{
		releaseNotifier: NewReleaseNotifier(settings),

		settings: settings,

		now:                   dc.GetClock().Now().UTC(),
		deckhousePodReadyFunc: deckhousePodReadyFunc,

		logger: logger,
	}
}

type DeployTimeReason struct {
	Reason                deployDelayReason
	Message               string
	ReleaseApplyAfterTime time.Time
	Notified              bool
}

// ProcessPatchReleaseDeployTime
// for patch release we check:
// - No delay from calculated deploy time
func (c *DeployTimeChecker) ProcessPatchReleaseDeployTime(dr *v1alpha1.DeckhouseRelease, res *DeployTimeResult) *DeployTimeReason {
	if dr.GetApplyNow() || res.Reason.IsNoDelay() {
		return nil
	}

	if res.ReleaseApplyTime == c.now {
		res.ReleaseApplyTime = time.Time{}
	}

	return &DeployTimeReason{
		Message:               res.Reason.Message(dr, res.ReleaseApplyTime),
		ReleaseApplyAfterTime: res.ReleaseApplyAfterTime,
	}
}

// ProcessMinorReleaseDeployTime
// for minor release we check:
// - Deckhouse pod is ready
// - No delay from calculated deploy time
func (c *DeployTimeChecker) ProcessMinorReleaseDeployTime(ctx context.Context, dr *v1alpha1.DeckhouseRelease, res *DeployTimeResult, dri *ReleaseInfo) *DeployTimeReason {
	// check: Deckhouse pod is ready
	if !c.deckhousePodReadyFunc(ctx) {
		c.logger.Info("Deckhouse is not ready. Skipping upgrade")

		return &DeployTimeReason{
			Message:               fmt.Sprintf("awaiting for Deckhouse v%s pod to be ready", dri.Version.String()),
			ReleaseApplyAfterTime: res.ReleaseApplyAfterTime,
		}
	}

	if dr.GetApplyNow() || res.Reason.IsNoDelay() {
		return nil
	}

	if res.ReleaseApplyTime == c.now {
		res.ReleaseApplyTime = time.Time{}
	}

	return &DeployTimeReason{
		Message:               res.Reason.Message(dr, res.ReleaseApplyTime),
		ReleaseApplyAfterTime: res.ReleaseApplyAfterTime,
	}
}

type DeployTimeResult struct {
	ReleaseApplyTime      time.Time
	ReleaseApplyAfterTime time.Time
	Reason                deployDelayReason
}

func (c *DeployTimeChecker) checkCanary(dtr *DeployTimeResult, dr *v1alpha1.DeckhouseRelease) {
	if dr.GetApplyAfter() != nil {
		applyAfter := *dr.GetApplyAfter()

		if c.now.Before(applyAfter) {
			c.logger.Warn("release is postponed by canary process, waiting", slog.String("name", dr.GetName()))

			dtr.ReleaseApplyTime = applyAfter
			dtr.Reason = dtr.Reason.add(canaryDelayReason)
		}
	}
}

func (c *DeployTimeChecker) checkCanaryNotInManualMode(dtr *DeployTimeResult, dr *v1alpha1.DeckhouseRelease) {
	if dr.GetApplyAfter() != nil && !c.settings.InManualMode() {
		applyAfter := *dr.GetApplyAfter()

		if c.now.Before(applyAfter) {
			c.logger.Warn("release is postponed by canary process, waiting", slog.String("name", dr.GetName()))

			dtr.ReleaseApplyTime = applyAfter
			dtr.Reason = dtr.Reason.add(canaryDelayReason)
		}
	}
}

func (c *DeployTimeChecker) checkNotify(dtr *DeployTimeResult, dr *v1alpha1.DeckhouseRelease) {
	if !dr.GetNotified() &&
		c.settings.NotificationConfig.MinimalNotificationTime.Duration > 0 {
		minApplyTime := c.now.Add(c.settings.NotificationConfig.MinimalNotificationTime.Duration)

		dtr.ReleaseApplyAfterTime = dtr.ReleaseApplyTime

		if !minApplyTime.Before(dtr.ReleaseApplyTime) {
			dtr.ReleaseApplyTime = minApplyTime
			dtr.ReleaseApplyAfterTime = minApplyTime
			dtr.Reason = dtr.Reason.add(notificationDelayReason)
		}
	}
}

func (c *DeployTimeChecker) checkWindowModeAutoPatch(dtr *DeployTimeResult) {
	if c.settings.Mode == updater.ModeAutoPatch && !c.settings.Windows.IsAllowed(dtr.ReleaseApplyTime) {
		dtr.ReleaseApplyTime = c.settings.Windows.NextAllowedTime(dtr.ReleaseApplyTime)
		dtr.Reason = dtr.Reason.add(outOfWindowReason)
	}
}

func (c *DeployTimeChecker) checkManualApproved(dtr *DeployTimeResult, dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) {
	// check: release is approved in Manual mode
	if c.settings.Mode == updater.ModeManual && !dr.GetManuallyApproved() {
		c.logger.Info("release is waiting for manual approval", slog.String("name", dr.GetName()))

		metricLabels.SetTrue(updater.ManualApprovalRequired)

		dtr.ReleaseApplyTime = c.now
		dtr.Reason = manualApprovalRequiredReason
	}
}

func (c *DeployTimeChecker) checkWindowModeAuto(dtr *DeployTimeResult) {
	if c.settings.Mode == updater.ModeAuto && !c.settings.Windows.IsAllowed(dtr.ReleaseApplyTime) {
		dtr.ReleaseApplyTime = c.settings.Windows.NextAllowedTime(dtr.ReleaseApplyTime)
		dtr.Reason = dtr.Reason.add(outOfWindowReason)
	}
}

func (c *DeployTimeChecker) checkManualApprovedNotModeAuto(dtr *DeployTimeResult, dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) {
	// check: release is approved in Manual mode
	if c.settings.Mode != updater.ModeAuto && !dr.GetManuallyApproved() {
		c.logger.Info("release is waiting for manual approval", slog.String("name", dr.GetName()))

		metricLabels.SetTrue(updater.ManualApprovalRequired)

		dtr.ReleaseApplyTime = c.now
		dtr.Reason = manualApprovalRequiredReason
	}
}

func (c *DeployTimeChecker) checkCooldown(dtr *DeployTimeResult, dr *v1alpha1.DeckhouseRelease) {
	// check: release cooldown
	if dr.GetCooldownUntil() != nil {
		cooldownUntil := *dr.GetCooldownUntil()
		if c.now.Before(cooldownUntil) {
			c.logger.Warn("release in cooldown", slog.String("name", dr.GetName()))

			dtr.ReleaseApplyTime = *dr.GetCooldownUntil()
			dtr.Reason = dtr.Reason.add(cooldownDelayReason)
		}
	}
}

func (c *DeployTimeChecker) CalculatePatchDeployTime(dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) *DeployTimeResult {
	result := &DeployTimeResult{
		Reason:           noDelay,
		ReleaseApplyTime: c.now,
	}

	if dr.GetApplyNow() {
		return result
	}

	c.checkCanary(result, dr)
	c.checkNotify(result, dr)
	c.checkWindowModeAutoPatch(result)
	c.checkManualApproved(result, dr, metricLabels)

	if !result.ReleaseApplyAfterTime.IsZero() {
		result.Reason = notificationDelayReason

		return result
	}

	return result
}

func (c *DeployTimeChecker) CalculateMinorDeployTime(dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) *DeployTimeResult {
	result := &DeployTimeResult{
		Reason:           noDelay,
		ReleaseApplyTime: c.now,
	}

	if dr.GetApplyNow() {
		return result
	}

	c.checkCooldown(result, dr)
	c.checkCanaryNotInManualMode(result, dr)
	c.checkNotify(result, dr)
	c.checkWindowModeAuto(result)
	c.checkManualApprovedNotModeAuto(result, dr, metricLabels)

	if !result.ReleaseApplyAfterTime.IsZero() {
		result.Reason = notificationDelayReason

		return result
	}

	return result
}

func (c *DeployTimeChecker) checkReleaseDisruptions(dr *v1alpha1.DeckhouseRelease) error {
	if !c.settings.InDisruptionApprovalMode() {
		return nil
	}

	// TODO: we save only last disruption condition
	for _, key := range dr.GetDisruptions() {
		hasDisruptionUpdate, reason := requirements.HasDisruption(key)
		if hasDisruptionUpdate && !dr.GetDisruptionApproved() {
			return fmt.Errorf("(`kubectl annotate DeckhouseRelease %s release.deckhouse.io/disruption-approved=true`): %s", dr.GetName(), reason)
		}
	}

	return nil
}
