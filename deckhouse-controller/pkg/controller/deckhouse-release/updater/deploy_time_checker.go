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
	metricsUpdater  updater.MetricsUpdater

	settings *updater.Settings

	now                   time.Time
	deckhousePodReadyFunc func(ctx context.Context) bool

	logger *log.Logger
}

func NewDeployTimeChecker(dc dependency.Container, metricsUpdater updater.MetricsUpdater, settings *updater.Settings, deckhousePodReadyFunc func(ctx context.Context) bool, logger *log.Logger) *DeployTimeChecker {
	return &DeployTimeChecker{
		releaseNotifier: NewReleaseNotifier(settings),
		metricsUpdater:  metricsUpdater,

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

// for patch, we check fewer conditions, then for minor release
// - Canary settings
func (c *DeployTimeChecker) CheckPatchReleaseConditions(ctx context.Context, dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) *DeployTimeReason {
	resultDeployTime := c.calculatePatchDeployTime(dr, metricLabels)

	// check: Notification
	if !c.settings.NotificationConfig.IsEmpty() && c.settings.NotificationConfig.ReleaseType == updater.ReleaseTypeAll {
		metricLabels.SetFalse(updater.NotificationNotSent)

		err := c.releaseNotifier.sendReleaseNotification(ctx, dr, resultDeployTime.ReleaseApplyTime)
		if err != nil {
			metricLabels.SetTrue(updater.NotificationNotSent)

			return &DeployTimeReason{
				Message:               "release blocked, failed to send release notification",
				ReleaseApplyAfterTime: resultDeployTime.ReleaseApplyAfterTime,
			}
		}
	}

	if dr.GetApplyNow() || resultDeployTime.Reason.IsNoDelay() {
		return nil
	}

	if resultDeployTime.ReleaseApplyTime == c.now {
		resultDeployTime.ReleaseApplyTime = time.Time{}
	}

	return &DeployTimeReason{
		Message:               resultDeployTime.Reason.Message(dr, resultDeployTime.ReleaseApplyTime),
		ReleaseApplyAfterTime: resultDeployTime.ReleaseApplyAfterTime,
		Notified:              true,
	}
}

type DeployTimeResult struct {
	ReleaseApplyTime      time.Time
	ReleaseApplyAfterTime time.Time
	Reason                deployDelayReason
}

func (c *DeployTimeChecker) checkCanary(dtr *DeployTimeResult, dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) {
	if dr.GetApplyAfter() != nil {
		applyAfter := *dr.GetApplyAfter()

		if c.now.Before(applyAfter) {
			c.logger.Warn("release is postponed by canary process, waiting", slog.String("name", dr.GetName()))

			dtr.ReleaseApplyTime = applyAfter
			dtr.Reason = dtr.Reason.add(canaryDelayReason)
		}
	}
}

func (c *DeployTimeChecker) checkCanaryNotInManualMode(dtr *DeployTimeResult, dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) {
	if dr.GetApplyAfter() != nil && !c.settings.InManualMode() {
		applyAfter := *dr.GetApplyAfter()

		if c.now.Before(applyAfter) {
			c.logger.Warn("release is postponed by canary process, waiting", slog.String("name", dr.GetName()))

			dtr.ReleaseApplyTime = applyAfter
			dtr.Reason = dtr.Reason.add(canaryDelayReason)
		}
	}
}

func (c *DeployTimeChecker) checkNotify(dtr *DeployTimeResult, dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) {
	if !dr.GetNotified() &&
		c.settings.NotificationConfig.MinimalNotificationTime.Duration > 0 {
		minApplyTime := c.now.Add(c.settings.NotificationConfig.MinimalNotificationTime.Duration)

		if minApplyTime.Before(dtr.ReleaseApplyTime) {
			// TODO: purpose???
			minApplyTime = dtr.ReleaseApplyTime
		} else {
			dtr.ReleaseApplyTime = minApplyTime
			dtr.ReleaseApplyAfterTime = minApplyTime
			dtr.Reason = dtr.Reason.add(notificationDelayReason)
		}
	}
}

func (c *DeployTimeChecker) checkWindowModeAutoPatch(dtr *DeployTimeResult, dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) {
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

func (c *DeployTimeChecker) checkWindowModeAuto(dtr *DeployTimeResult, dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) {
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

func (c *DeployTimeChecker) checkCooldown(dtr *DeployTimeResult, dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) {
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

func (c *DeployTimeChecker) calculatePatchDeployTime(dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) *DeployTimeResult {
	result := &DeployTimeResult{
		Reason:           noDelay,
		ReleaseApplyTime: c.now,
	}

	if dr.GetApplyNow() {
		return result
	}

	c.checkCanary(result, dr, metricLabels)
	c.checkNotify(result, dr, metricLabels)
	c.checkWindowModeAutoPatch(result, dr, metricLabels)
	c.checkManualApproved(result, dr, metricLabels)

	if !result.ReleaseApplyAfterTime.IsZero() {
		result.Reason = notificationDelayReason

		return result
	}

	return result
}

// for minor release (version change) we check more conditions
// - Release requirements
// - Disruptions
// - Notification
// - Cooldown
// - Canary settings
// - Update windows or manual approval
// - Deckhouse pod is ready
func (c *DeployTimeChecker) CheckMinorReleaseConditions(ctx context.Context, dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) *DeployTimeReason {
	// check: release disruptions (hard lock)
	err := c.checkReleaseDisruptions(dr)
	if err != nil {
		metricLabels.SetTrue(updater.DisruptionApprovalRequired)

		return &DeployTimeReason{
			Message: fmt.Sprintf("release blocked, disruption approval required: %v", err),
		}
	}

	resultDeployTime := c.calculateMinorDeployTime(dr, metricLabels)

	// check: Notification
	if !c.settings.NotificationConfig.IsEmpty() {
		metricLabels.SetFalse(updater.NotificationNotSent)

		err := c.releaseNotifier.sendReleaseNotification(ctx, dr, resultDeployTime.ReleaseApplyTime)
		if err != nil {
			metricLabels.SetTrue(updater.NotificationNotSent)

			return &DeployTimeReason{
				Message:               "release blocked, failed to send release notification",
				ReleaseApplyAfterTime: resultDeployTime.ReleaseApplyAfterTime,
			}
		}
	}

	// check: Deckhouse pod is ready
	if !c.deckhousePodReadyFunc(ctx) {
		c.logger.Info("Deckhouse is not ready. Skipping upgrade")

		return &DeployTimeReason{
			Message:               "awaiting for Deckhouse pod to be ready",
			ReleaseApplyAfterTime: resultDeployTime.ReleaseApplyAfterTime,
			Notified:              true,
		}
	}

	if dr.GetApplyNow() || resultDeployTime.Reason.IsNoDelay() {
		return nil
	}

	if resultDeployTime.ReleaseApplyTime == c.now {
		resultDeployTime.ReleaseApplyTime = time.Time{}
	}

	return &DeployTimeReason{
		Message:               resultDeployTime.Reason.Message(dr, resultDeployTime.ReleaseApplyTime),
		ReleaseApplyAfterTime: resultDeployTime.ReleaseApplyAfterTime,
		Notified:              true,
	}
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

func (c *DeployTimeChecker) calculateMinorDeployTime(dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) *DeployTimeResult {
	result := &DeployTimeResult{
		Reason:           noDelay,
		ReleaseApplyTime: c.now,
	}

	if dr.GetApplyNow() {
		return result
	}

	c.checkCooldown(result, dr, metricLabels)
	c.checkCanaryNotInManualMode(result, dr, metricLabels)
	c.checkNotify(result, dr, metricLabels)
	c.checkWindowModeAuto(result, dr, metricLabels)
	c.checkManualApprovedNotModeAuto(result, dr, metricLabels)

	if !result.ReleaseApplyAfterTime.IsZero() {
		result.Reason = notificationDelayReason

		return result
	}

	return result
}
