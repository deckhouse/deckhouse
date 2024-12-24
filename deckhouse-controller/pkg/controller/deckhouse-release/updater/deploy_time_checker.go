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
	"net/http"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/go_lib/updater"
	"github.com/deckhouse/deckhouse/pkg/log"
	aoapp "github.com/flant/addon-operator/pkg/app"
)

type DeployTimeChecker struct {
	dc              dependency.Container
	releaseNotifier *ReleaseNotifier
	metricsUpdater  updater.MetricsUpdater

	settings *updater.Settings

	now            time.Time
	deckhousePodIP string

	logger *log.Logger
}

func NewDeployTimeChecker(dc dependency.Container, releaseNotifier *ReleaseNotifier, metricsUpdater updater.MetricsUpdater, settings *updater.Settings, logger *log.Logger) *DeployTimeChecker {
	return &DeployTimeChecker{
		dc:              dc,
		releaseNotifier: releaseNotifier,
		metricsUpdater:  metricsUpdater,

		settings: settings,

		now: dc.GetClock().Now().UTC(),

		logger: logger,
	}
}

func (c *DeployTimeChecker) CheckRelease() error {
	return nil
}

// for patch, we check fewer conditions, then for minor release
// - Canary settings
func (c *DeployTimeChecker) checkPatchReleaseConditions(ctx context.Context, dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) error {
	dtResult, err := c.calculatePatchDeployTime(dr, metricLabels)
	if err != nil {
		return fmt.Errorf("calculate patch result deploy time: %w", err)
	}

	// TODO: return info about DeployTimeAfter to update annotation
	// err := c.kubeAPI.PatchReleaseApplyAfter(dr, newApplyAfter)
	// if err != nil {
	// 	return nil, fmt.Errorf("patch release %s apply after: %w", dr.GetName(), err)
	// }

	// check: Notification
	if !c.settings.NotificationConfig.IsEmpty() && c.settings.NotificationConfig.ReleaseType == updater.ReleaseTypeAll {
		metricLabels.SetFalse(updater.NotificationNotSent)

		err = c.releaseNotifier.sendReleaseNotification(ctx, dr, dtResult.ReleaseApplyTime)
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

	return c.postponeDeploy(dr, dtResult.Reason, dtResult.ReleaseApplyTime)
}

type DeployTimeResult struct {
	ReleaseApplyTime      time.Time
	ReleaseApplyAfterTime time.Time
	Reason                deployDelayReason
}

func (c *DeployTimeChecker) calculatePatchDeployTime(dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) (*DeployTimeResult, error) {
	result := &DeployTimeResult{
		ReleaseApplyTime: c.now,
		Reason:           noDelay,
	}

	if dr.GetApplyNow() {
		return result, nil
	}

	// check: canary settings
	if dr.GetApplyAfter() != nil {
		applyAfter := *dr.GetApplyAfter()

		if c.now.Before(applyAfter) {
			c.logger.Warn("release is postponed by canary process, waiting", slog.String("name", dr.GetName()))

			result = &DeployTimeResult{
				ReleaseApplyTime: applyAfter,
				Reason:           result.Reason.add(canaryDelayReason),
			}
		}
	}

	if !dr.GetNotified() &&
		c.settings.NotificationConfig.MinimalNotificationTime.Duration > 0 {
		minApplyTime := c.now.Add(c.settings.NotificationConfig.MinimalNotificationTime.Duration)

		if minApplyTime.Before(result.ReleaseApplyTime) {
			// TODO: purpose???
			minApplyTime = result.ReleaseApplyTime
		} else {
			result = &DeployTimeResult{
				ReleaseApplyTime:      minApplyTime,
				ReleaseApplyAfterTime: minApplyTime,
				Reason:                result.Reason.add(notificationDelayReason),
			}
		}
	}

	if c.settings.Mode == updater.ModeAutoPatch && !c.settings.Windows.IsAllowed(result.ReleaseApplyTime) {
		result = &DeployTimeResult{
			ReleaseApplyTime: c.settings.Windows.NextAllowedTime(result.ReleaseApplyTime),
			Reason:           result.Reason.add(outOfWindowReason),
		}
	}

	// check: release is approved in Manual mode
	if c.settings.Mode == updater.ModeManual && !dr.GetManuallyApproved() {
		c.logger.Info("release is waiting for manual approval", slog.String("name", dr.GetName()))

		metricLabels.SetTrue(updater.ManualApprovalRequired)

		result = &DeployTimeResult{
			ReleaseApplyTime: c.now,
			Reason:           manualApprovalRequiredReason,
		}
	}

	if !result.ReleaseApplyAfterTime.IsZero() {
		result.Reason = notificationDelayReason

		return result, nil
	}

	return result, nil
}

// for minor release (version change) we check more conditions
// - Release requirements
// - Disruptions
// - Notification
// - Cooldown
// - Canary settings
// - Update windows or manual approval
// - Deckhouse pod is ready
func (c *DeployTimeChecker) checkMinorReleaseConditions(ctx context.Context, dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) error {
	// check: release disruptions (hard lock)
	passed := c.checkReleaseDisruptions(dr)
	if !passed {
		metricLabels.SetTrue(updater.DisruptionApprovalRequired)

		return fmt.Errorf("release %s disruption approval required: %w", dr.GetName(), updater.ErrDeployConditionsNotMet)
	}

	resultDeployTime, err := c.calculateMinorDeployTime(dr, metricLabels)
	if err != nil {
		return fmt.Errorf("calculate minor result deploy time: %w", err)
	}

	// TODO: return info about DeployTimeAfter to update annotation
	// err := c.kubeAPI.PatchReleaseApplyAfter(dr, newApplyAfter)
	// if err != nil {
	// 	return nil, fmt.Errorf("patch release %s apply after: %w", dr.GetName(), err)
	// }

	// check: Notification
	if !c.settings.NotificationConfig.IsEmpty() {
		metricLabels.SetFalse(updater.NotificationNotSent)

		err = c.releaseNotifier.sendReleaseNotification(ctx, dr, resultDeployTime.ReleaseApplyTime)
		if err != nil {
			metricLabels.SetTrue(updater.NotificationNotSent)

			if err := c.updateStatus(dr, "Release blocked: failed to send release notification", v1alpha1.DeckhouseReleasePhasePending); err != nil {
				return fmt.Errorf("update status: %w", err)
			}

			return fmt.Errorf("send release notification: %w", err)
		}
	}

	// check: Deckhouse pod is ready
	if !c.isDeckhousePodReady(ctx) {
		c.logger.Info("Deckhouse is not ready. Skipping upgrade")

		if err := c.updateStatus(dr, "Awaiting for Deckhouse pod to be ready", v1alpha1.DeckhouseReleasePhasePending); err != nil {
			return fmt.Errorf("update status: %w", err)
		}

		return updater.ErrDeployConditionsNotMet
	}

	if dr.GetApplyNow() {
		return nil
	}

	return c.postponeDeploy(dr, resultDeployTime.Reason, resultDeployTime.ReleaseApplyTime)
}

func (c *DeployTimeChecker) checkReleaseDisruptions(rl *v1alpha1.DeckhouseRelease) bool {
	if !c.settings.InDisruptionApprovalMode() {
		return true
	}

	for _, key := range rl.GetDisruptions() {
		hasDisruptionUpdate, reason := requirements.HasDisruption(key)
		if hasDisruptionUpdate && !rl.GetDisruptionApproved() {
			msg := fmt.Sprintf("Release requires disruption approval (`kubectl annotate DeckhouseRelease %s release.deckhouse.io/disruption-approved=true`): %s", rl.GetName(), reason)

			err := c.updateStatus(rl, msg, v1alpha1.DeckhouseReleasePhasePending)
			if err != nil {
				c.logger.Error("update status", log.Err(err))
			}

			return false
		}
	}

	return true
}

func (c *DeployTimeChecker) calculateMinorDeployTime(dr *v1alpha1.DeckhouseRelease, metricLabels updater.MetricLabels) (*DeployTimeResult, error) {
	result := &DeployTimeResult{
		ReleaseApplyTime: c.now,
		Reason:           noDelay,
	}

	if dr.GetApplyNow() {
		return result, nil
	}

	// check: release cooldown
	if dr.GetCooldownUntil() != nil {
		cooldownUntil := *dr.GetCooldownUntil()
		if c.now.Before(cooldownUntil) {
			c.logger.Warn("release in cooldown", slog.String("name", dr.GetName()))

			result = &DeployTimeResult{
				ReleaseApplyTime: *dr.GetCooldownUntil(),
				Reason:           result.Reason.add(cooldownDelayReason),
			}
		}
	}

	// check: canary settings
	if dr.GetApplyAfter() != nil && !c.settings.InManualMode() {
		applyAfter := *dr.GetApplyAfter()
		if c.now.Before(applyAfter) {
			c.logger.Warn("release is postponed by canary process, waiting", slog.String("name", dr.GetName()))

			result = &DeployTimeResult{
				ReleaseApplyTime: applyAfter,
				Reason:           result.Reason.add(canaryDelayReason),
			}
		}
	}

	if !dr.GetNotified() &&
		c.settings.NotificationConfig.MinimalNotificationTime.Duration > 0 {
		minApplyTime := c.now.Add(c.settings.NotificationConfig.MinimalNotificationTime.Duration)

		if minApplyTime.Before(result.ReleaseApplyTime) {
			minApplyTime = result.ReleaseApplyTime
		} else {
			result = &DeployTimeResult{
				ReleaseApplyTime:      minApplyTime,
				ReleaseApplyAfterTime: minApplyTime,
				Reason:                result.Reason.add(notificationDelayReason),
			}
		}
	}

	if c.settings.Mode == updater.ModeAuto && !c.settings.Windows.IsAllowed(result.ReleaseApplyTime) {
		result = &DeployTimeResult{
			ReleaseApplyTime: c.settings.Windows.NextAllowedTime(result.ReleaseApplyTime),
			Reason:           result.Reason.add(outOfWindowReason),
		}
	}

	// check: release is approved in Manual mode
	if c.settings.Mode != updater.ModeAuto && !dr.GetManuallyApproved() {
		c.logger.Info("release is waiting for manual approval", slog.String("name", dr.GetName()))

		metricLabels[updater.ManualApprovalRequired] = "true"

		result = &DeployTimeResult{
			ReleaseApplyTime: c.now,
			Reason:           manualApprovalRequiredReason,
		}
	}

	if !result.ReleaseApplyAfterTime.IsZero() {
		result.Reason = notificationDelayReason
		return result, nil
	}

	return result, nil
}

// postponeDeploy update release status and returns new NotReadyForDeployError if reason not equal to noDelay and nil otherwise.
func (c *DeployTimeChecker) postponeDeploy(release *v1alpha1.DeckhouseRelease, reason deployDelayReason, applyTime time.Time) error {
	if reason == noDelay {
		return nil
	}

	var (
		zeroTime      time.Time
		retryDelay    time.Duration
		statusMessage string
	)

	if !applyTime.IsZero() {
		retryDelay = applyTime.Sub(c.now)
	}

	if applyTime == c.now {
		applyTime = zeroTime
	}

	statusMessage = reason.Message(release, applyTime)

	err := c.updateStatus(release, statusMessage, v1alpha1.DeckhouseReleasePhasePending)
	if err != nil {
		return fmt.Errorf("update release %s status: %w", release.GetName(), err)
	}

	return updater.NewNotReadyForDeployError(statusMessage)
}

func (c *DeployTimeChecker) isDeckhousePodReady(ctx context.Context) bool {
	deckhousePodIP := aoapp.ListenAddress

	url := fmt.Sprintf("http://%s:4222/readyz", deckhousePodIP)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.logger.Error("deckhouse pod readyz create request", log.Err(err))

		return false
	}

	resp, err := c.dc.GetHTTPClient().Do(req)
	if err != nil {
		c.logger.Error("deckhouse pod readyz do request", log.Err(err))

		return false
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("deckhouse pod readyz", slog.Int("status_code", resp.StatusCode), log.Err(err))

		return false
	}

	return true
}
