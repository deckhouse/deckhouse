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

package releaseupdater

import (
	"log/slog"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type DeployTimeService struct {
	releaseNotifier *ReleaseNotifier

	settings *Settings

	now time.Time

	logger *log.Logger
}

func NewDeployTimeService(dc dependency.Container, settings *Settings, logger *log.Logger) *DeployTimeService {
	return &DeployTimeService{
		releaseNotifier: NewReleaseNotifier(settings),

		settings: settings,

		now: dc.GetClock().Now().UTC(),

		logger: logger,
	}
}

type ProcessedDeployTimeResult struct {
	Reason                DeployDelayReason
	Message               string
	ReleaseApplyAfterTime time.Time
}

// ProcessPatchReleaseDeployTime
// for patch release we check:
// - No delay from calculated deploy time
func (c *DeployTimeService) ProcessPatchReleaseDeployTime(release v1alpha1.Release, res *DeployTimeResult) *ProcessedDeployTimeResult {
	if release.GetApplyNow() || res.Reason.IsNoDelay() {
		return nil
	}

	if res.ReleaseApplyTime.Equal(c.now) {
		res.ReleaseApplyTime = time.Time{}
	}

	return &ProcessedDeployTimeResult{
		Message:               res.Reason.Message(release, res.ReleaseApplyTime),
		ReleaseApplyAfterTime: res.ReleaseApplyAfterTime,
	}
}

// ProcessMinorReleaseDeployTime
// for minor release we check:
// - Deckhouse pod is ready
// - No delay from calculated deploy time
func (c *DeployTimeService) ProcessMinorReleaseDeployTime(release v1alpha1.Release, res *DeployTimeResult) *ProcessedDeployTimeResult {
	if release.GetApplyNow() || res.Reason.IsNoDelay() {
		return nil
	}

	if res.ReleaseApplyTime.Equal(c.now) {
		res.ReleaseApplyTime = time.Time{}
	}

	return &ProcessedDeployTimeResult{
		Message:               res.Reason.Message(release, res.ReleaseApplyTime),
		ReleaseApplyAfterTime: res.ReleaseApplyAfterTime,
	}
}

type DeployTimeResult struct {
	ReleaseApplyTime      time.Time
	ReleaseApplyAfterTime time.Time
	Reason                DeployDelayReason
}

func (c *DeployTimeService) checkCanary(dtr *DeployTimeResult, release v1alpha1.Release) {
	if release.GetApplyAfter() != nil {
		applyAfter := *release.GetApplyAfter()

		if c.now.Before(applyAfter) {
			c.logger.Warn("release is postponed by canary process, waiting", slog.String("name", release.GetName()))

			dtr.ReleaseApplyTime = applyAfter
			dtr.Reason = dtr.Reason.add(canaryDelayReason)
		}
	}
}

func (c *DeployTimeService) checkNotify(dtr *DeployTimeResult, release v1alpha1.Release) {
	if !release.GetNotified() &&
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

func (c *DeployTimeService) processManualApproved(dtr *DeployTimeResult, release v1alpha1.Release, metricLabels MetricLabels) {
	c.logger.Info("release is waiting for manual approval", slog.String("name", release.GetName()))

	metricLabels.SetTrue(ManualApprovalRequired)

	dtr.ReleaseApplyTime = c.now
	dtr.Reason = manualApprovalRequiredReason
}

func (c *DeployTimeService) processWindow(dtr *DeployTimeResult) {
	dtr.ReleaseApplyTime = c.settings.Windows.NextAllowedTime(dtr.ReleaseApplyTime)
	dtr.Reason = dtr.Reason.add(outOfWindowReason)
}

// CalculatePatchDeployTime calculates deploy time, returns deploy time or postpone time and reason.
// To calculate deploy time, we need to check:
//
// 1) Canary
// 2) Notify
// 3) Window (in not "Manual" mode)
// 4) Manual approve (only in "Manual" mode)
//
// Notify reason must override any other reason
func (c *DeployTimeService) CalculatePatchDeployTime(release v1alpha1.Release, metricLabels MetricLabels) *DeployTimeResult {
	result := &DeployTimeResult{
		Reason:           noDelay,
		ReleaseApplyTime: c.now,
	}

	if release.GetApplyNow() {
		return result
	}

	c.checkCanary(result, release)
	c.checkNotify(result, release)

	if c.settings.Mode != v1alpha2.UpdateModeManual && !c.settings.Windows.IsAllowed(result.ReleaseApplyTime) {
		c.processWindow(result)
	}

	if c.settings.Mode == v1alpha2.UpdateModeManual && !release.GetManuallyApproved() {
		c.processManualApproved(result, release, metricLabels)
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
func (c *DeployTimeService) CalculateMinorDeployTime(release v1alpha1.Release, metricLabels MetricLabels) *DeployTimeResult {
	result := &DeployTimeResult{
		Reason:           noDelay,
		ReleaseApplyTime: c.now,
	}

	if release.GetApplyNow() {
		return result
	}

	if !c.settings.InManualMode() {
		c.checkCanary(result, release)
	}

	c.checkNotify(result, release)

	if c.settings.Mode == v1alpha2.UpdateModeAuto && !c.settings.Windows.IsAllowed(result.ReleaseApplyTime) {
		c.processWindow(result)
	}

	if c.settings.Mode != v1alpha2.UpdateModeAuto && !release.GetManuallyApproved() {
		c.processManualApproved(result, release, metricLabels)
	}

	if !result.ReleaseApplyAfterTime.IsZero() {
		result.Reason = notificationDelayReason

		return result
	}

	return result
}
