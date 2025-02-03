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

		if dri == nil {
			return &DeployTimeReason{
				Message:               "can not find deployed version, awaiting",
				ReleaseApplyAfterTime: res.ReleaseApplyAfterTime,
			}
		}

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

func (c *DeployTimeChecker) processManualApproved(dtr *DeployTimeResult, dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) {
	c.logger.Info("release is waiting for manual approval", slog.String("name", dr.GetName()))

	metricLabels.SetTrue(updater.ManualApprovalRequired)

	dtr.ReleaseApplyTime = c.now
	dtr.Reason = manualApprovalRequiredReason
}

func (c *DeployTimeChecker) processWindow(dtr *DeployTimeResult) {
	dtr.ReleaseApplyTime = c.settings.Windows.NextAllowedTime(dtr.ReleaseApplyTime)
	dtr.Reason = dtr.Reason.add(outOfWindowReason)
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

// CalculatePatchDeployTime calculates deploy time, returns deploy time or postpone time and reason.
// To calculate deploy time, we need to check:
//
// 1) Canary
// 2) Notify
// 3) Window (only in "AutoPatch" mode)
// 4) Manual approve (only in "Manual" mode)
//
// Notify reason must override any other reason
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

	if c.settings.Mode == updater.ModeAutoPatch && !c.settings.Windows.IsAllowed(result.ReleaseApplyTime) {
		c.processWindow(result)
	}

	if c.settings.Mode == updater.ModeManual && !dr.GetManuallyApproved() {
		c.processManualApproved(result, dr, metricLabels)
	}

	if !result.ReleaseApplyAfterTime.IsZero() {
		result.Reason = notificationDelayReason

		return result
	}

	return result
}

// CalculatePatchDeployTime calculates deploy time, returns deploy time or postpone time and reason.
// To calculate deploy time, we need to check:
//
// 1) Cooldown (TODO: deprecated?)
// 1) Canary (in any mode, except "Manual")
// 2) Notify
// 3) Window (only in "Auto" mode)
// 4) Manual approve (in any mode, except "Auto")
//
// Notify reason must override any other reason
func (c *DeployTimeChecker) CalculateMinorDeployTime(dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) *DeployTimeResult {
	result := &DeployTimeResult{
		Reason:           noDelay,
		ReleaseApplyTime: c.now,
	}

	if dr.GetApplyNow() {
		return result
	}

	c.checkCooldown(result, dr)

	if !c.settings.InManualMode() {
		c.checkCanary(result, dr)
	}

	c.checkNotify(result, dr)

	if c.settings.Mode == updater.ModeAuto && !c.settings.Windows.IsAllowed(result.ReleaseApplyTime) {
		c.processWindow(result)
	}

	if c.settings.Mode != updater.ModeAuto && !dr.GetManuallyApproved() {
		c.processManualApproved(result, dr, metricLabels)
	}

	if !result.ReleaseApplyAfterTime.IsZero() {
		result.Reason = notificationDelayReason

		return result
	}

	return result
}
