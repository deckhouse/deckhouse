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
	"github.com/deckhouse/deckhouse/go_lib/updater"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type DeployTimeChecker struct {
	metricsUpdater updater.MetricsUpdater

	settings *updater.Settings

	now time.Time

	logger *log.Logger
}

func NewDeployTimeChecker(metricsUpdater updater.MetricsUpdater, logger *log.Logger) *DeployTimeChecker {
	// now:            dc.GetClock().Now().UTC(),
	return &DeployTimeChecker{
		metricsUpdater: metricsUpdater,
		logger:         logger,
	}
}

func (c *DeployTimeChecker) CheckRelease() error {
	return nil
}

// for patch, we check fewer conditions, then for minor release
// - Canary settings
func (c *DeployTimeChecker) checkPatchReleaseConditions(ctx context.Context, dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) error {
	applyTime, reason, err := c.calculatePatchDeployTime(dr, metricLabels)
	if err != nil {
		return fmt.Errorf("calculate patch result deploy time: %w", err)
	}

	// check: Notification
	if !c.settings.NotificationConfig.IsEmpty() && c.settings.NotificationConfig.ReleaseType == updater.ReleaseTypeAll {
		metricLabels.SetFalse(updater.NotificationNotSent)
		err = c.sendReleaseNotification(dr, applyTime)
		if err != nil {
			metricLabels.SetTrue(updater.NotificationNotSent)

			err := c.updateReleaseStatus(ctx, dr, &v1alpha1.DeckhouseReleaseStatus{
				Phase:   v1alpha1.DeckhouseReleasePhasePending,
				Message: "release blocked: failed to send release notification",
			})
			if err != nil {
				c.logger.Warn("met requirements status update ", slog.String("name", dr.GetName()), log.Err(err))
			}

			return fmt.Errorf("send release notification: %w", err)
		}
	}

	if dr.GetApplyNow() {
		return nil
	}

	return c.postponeDeploy(dr, reason, applyTime)
}

func (c *DeployTimeChecker) calculatePatchDeployTime(release *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) (time.Time, deployDelayReason, error) {
	var (
		newApplyAfter    time.Time
		releaseApplyTime = c.now
		reason           deployDelayReason
	)

	if release.GetApplyNow() {
		return releaseApplyTime, reason, nil
	}

	// check: canary settings
	if release.GetApplyAfter() != nil {
		applyAfter := *release.GetApplyAfter()
		if c.now.Before(applyAfter) {
			c.logger.Warnf("Release %s is postponed by canary process. Waiting", release.GetName())
			releaseApplyTime, reason = applyAfter, reason.add(canaryDelayReason)
		}
	}

	if !c.releaseData.Notified &&
		c.settings.NotificationConfig.MinimalNotificationTime.Duration > 0 {
		minApplyTime := c.now.Add(c.settings.NotificationConfig.MinimalNotificationTime.Duration)
		if minApplyTime.Before(releaseApplyTime) {
			minApplyTime = releaseApplyTime
		} else {
			releaseApplyTime, newApplyAfter, reason = minApplyTime, minApplyTime, reason.add(notificationDelayReason)
		}
	}

	if c.settings.Mode == updater.ModeAutoPatch && !c.settings.Windows.IsAllowed(releaseApplyTime) {
		releaseApplyTime, reason = c.settings.Windows.NextAllowedTime(releaseApplyTime), reason.add(outOfWindowReason)
	}

	if c.settings.Mode == updater.ModeManual && !release.GetManuallyApproved() {
		c.logger.Infof("Release %s is waiting for manual approval", release.GetName())
		metricLabels[updater.ManualApprovalRequired] = "true"
		releaseApplyTime, reason = c.now, manualApprovalRequiredReason
	}

	if !newApplyAfter.IsZero() {
		err := c.kubeAPI.PatchReleaseApplyAfter(release, newApplyAfter)
		if err != nil {
			return time.Time{}, 0, fmt.Errorf("patch release %s apply after: %w", release.GetName(), err)
		}

		return releaseApplyTime, notificationDelayReason, nil
	}

	return releaseApplyTime, reason, nil
}

// for minor release (version change) we check more conditions
// - Release requirements
// - Disruptions
// - Notification
// - Cooldown
// - Canary settings
// - Update windows or manual approval
// - Deckhouse pod is ready
func (c *DeployTimeChecker) checkMinorDeployTime(release R, metricLabels MetricLabels) error {
	// check: release disruptions (hard lock)
	passed := c.checkReleaseDisruptions(release)
	if !passed {
		metricLabels.SetTrue(updater.DisruptionApprovalRequired)
		return fmt.Errorf("release %s disruption approval required: %w", release.GetName(), updater.ErrDeployConditionsNotMet)
	}

	resultDeployTime, delayReason, err := c.calculateMinorDeployTime(release, metricLabels)
	if err != nil {
		return fmt.Errorf("calculate minor result deploy time: %w", err)
	}

	// check: Notification
	if !c.settings.NotificationConfig.IsEmpty() {
		metricLabels.SetFalse(updater.NotificationNotSent)
		err = c.sendReleaseNotification(release, resultDeployTime)
		if err != nil {
			metricLabels.SetTrue(updater.NotificationNotSent)
			if err := c.updateStatus(release, "Release blocked: failed to send release notification", v1alpha1.DeckhouseReleasePhasePending); err != nil {
				return fmt.Errorf("update status: %w", err)
			}
			return fmt.Errorf("send release notification: %w", err)
		}
	}

	// check: Deckhouse pod is ready
	if !c.deckhousePodIsReady {
		c.logger.Info("Deckhouse is not ready. Skipping upgrade")
		if err := c.updateStatus(release, "Awaiting for Deckhouse pod to be ready", v1alpha1.DeckhouseReleasePhasePending); err != nil {
			return fmt.Errorf("update status: %w", err)
		}
		return updater.ErrDeployConditionsNotMet
	}

	if release.GetApplyNow() {
		return nil
	}

	return c.postponeDeploy(release, delayReason, resultDeployTime)
}

func (c *DeployTimeChecker) calculateMinorDeployTime(release *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) (time.Time, deployDelayReason, error) {
	var (
		newApplyAfter    time.Time
		releaseApplyTime = c.now
		reason           deployDelayReason
	)

	if release.GetApplyNow() {
		return releaseApplyTime, reason, nil
	}

	// check: release cooldown
	if release.GetCooldownUntil() != nil {
		cooldownUntil := *release.GetCooldownUntil()
		if c.now.Before(cooldownUntil) {
			c.logger.Warnf("Release %s in cooldown", release.GetName())
			releaseApplyTime, reason = *release.GetCooldownUntil(), reason.add(cooldownDelayReason)
		}
	}

	// check: canary settings
	if release.GetApplyAfter() != nil && !c.InManualMode() {
		applyAfter := *release.GetApplyAfter()
		if c.now.Before(applyAfter) {
			c.logger.Warnf("Release %s is postponed by canary process. Waiting", release.GetName())
			releaseApplyTime, reason = applyAfter, reason.add(canaryDelayReason)
		}
	}

	if !c.releaseData.Notified &&
		c.settings.NotificationConfig.MinimalNotificationTime.Duration > 0 {
		minApplyTime := c.now.Add(c.settings.NotificationConfig.MinimalNotificationTime.Duration)
		if minApplyTime.Before(releaseApplyTime) {
			minApplyTime = releaseApplyTime
		} else {
			releaseApplyTime, newApplyAfter, reason = minApplyTime, minApplyTime, reason.add(notificationDelayReason)
		}
	}

	if c.settings.Mode == updater.ModeAuto && !c.settings.Windows.IsAllowed(releaseApplyTime) {
		releaseApplyTime, reason = c.settings.Windows.NextAllowedTime(releaseApplyTime), reason.add(outOfWindowReason)
	}

	// check: release is approved in Manual mode
	if c.settings.Mode != updater.ModeAuto && !release.GetManuallyApproved() {
		c.logger.Infof("Release %s is waiting for manual approval ", release.GetName())
		metricLabels[updater.ManualApprovalRequired] = "true"
		releaseApplyTime, reason = c.now, manualApprovalRequiredReason
	}

	if !newApplyAfter.IsZero() {
		err := c.kubeAPI.PatchReleaseApplyAfter(release, newApplyAfter)
		if err != nil {
			return time.Time{}, 0, fmt.Errorf("patch release %s apply after: %w", release.GetName(), err)
		}

		return releaseApplyTime, notificationDelayReason, nil
	}

	return releaseApplyTime, reason, nil
}
