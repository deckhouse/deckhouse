/*
Copyright 2026 Flant JSC

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

package checker

import (
	"context"
	"errors"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

const observabilityNamespace = "d8-observability"

var (
	observabilityMetricsRulesGroupGVR = schema.GroupVersionResource{
		Group:    "observability.deckhouse.io",
		Version:  "v1alpha1",
		Resource: "observabilitymetricsrulesgroups",
	}
	observabilityNotificationSilenceGVR = schema.GroupVersionResource{
		Group:    "observability.deckhouse.io",
		Version:  "v1alpha1",
		Resource: "observabilitynotificationsilences",
	}
	prometheusRuleGVR = schema.GroupVersionResource{
		Group:    "monitoring.coreos.com",
		Version:  "v1",
		Resource: "prometheusrules",
	}
	observabilityAlertGVR = schema.GroupVersionResource{
		Group:    "alerts.observability.deckhouse.io",
		Version:  "v1alpha1",
		Resource: "observabilityalerts",
	}
)

// ObservabilityRulesGroupRecordingLifecycle is a checker constructor and configurator
type ObservabilityRulesGroupRecordingLifecycle struct {
	Access           kubernetes.Access
	PreflightChecker check.Checker

	AgentID        string
	Namespace      string
	RulesGroupName string

	RecordingMetric    string
	PrometheusEndpoint string

	RequestTimeout                   time.Duration
	WaitPrometheusRuleCreatedTimeout time.Duration
	WaitMetricPresentTimeout         time.Duration
	WaitPrometheusRuleDeletedTimeout time.Duration
	WaitNamespaceDeletedTimeout      time.Duration
	Timeout                          time.Duration
}

func (c ObservabilityRulesGroupRecordingLifecycle) Checker() check.Checker {
	checker := &observabilityRulesGroupRecordingLifecycleChecker{
		observabilityLifecycleBase: observabilityLifecycleBase{
			access:                           c.Access,
			preflightChecker:                 c.PreflightChecker,
			agentID:                          fallbackString(c.AgentID, "unknown"),
			namespace:                        c.Namespace,
			probeName:                        "observability-recording",
			rulesGroupName:                   c.RulesGroupName,
			prometheusRuleName:               expectedPrometheusRuleName(c.Namespace, c.RulesGroupName),
			requestTimeout:                   fallbackDuration(c.RequestTimeout, 5*time.Second),
			waitPrometheusRuleCreatedTimeout: fallbackDuration(c.WaitPrometheusRuleCreatedTimeout, 60*time.Second),
			waitPrometheusRuleDeletedTimeout: fallbackDuration(c.WaitPrometheusRuleDeletedTimeout, 60*time.Second),
			waitNamespaceDeletedTimeout:      fallbackDuration(c.WaitNamespaceDeletedTimeout, 60*time.Second),
		},
		recordingMetric:          c.RecordingMetric,
		prometheusEndpoint:       c.PrometheusEndpoint,
		waitMetricPresentTimeout: fallbackDuration(c.WaitMetricPresentTimeout, 120*time.Second),
	}

	return withTimeout(checker, fallbackDuration(c.Timeout, 5*time.Minute))
}

// ObservabilityRulesGroupAlertLifecycle is a checker constructor and configurator
type ObservabilityRulesGroupAlertLifecycle struct {
	Access           kubernetes.Access
	PreflightChecker check.Checker

	AgentID        string
	Namespace      string
	RulesGroupName string
	SilenceName    string

	AlertName       string
	AlertLabelKey   string
	AlertLabelValue string

	RequestTimeout                   time.Duration
	WaitPrometheusRuleCreatedTimeout time.Duration
	WaitAlertPresentTimeout          time.Duration
	WaitPrometheusRuleDeletedTimeout time.Duration
	WaitNamespaceDeletedTimeout      time.Duration
	Timeout                          time.Duration
}

func (c ObservabilityRulesGroupAlertLifecycle) Checker() check.Checker {
	checker := &observabilityRulesGroupAlertLifecycleChecker{
		observabilityLifecycleBase: observabilityLifecycleBase{
			access:                           c.Access,
			preflightChecker:                 c.PreflightChecker,
			agentID:                          fallbackString(c.AgentID, "unknown"),
			namespace:                        c.Namespace,
			probeName:                        "alertmanager",
			rulesGroupName:                   c.RulesGroupName,
			prometheusRuleName:               expectedPrometheusRuleName(c.Namespace, c.RulesGroupName),
			requestTimeout:                   fallbackDuration(c.RequestTimeout, 5*time.Second),
			waitPrometheusRuleCreatedTimeout: fallbackDuration(c.WaitPrometheusRuleCreatedTimeout, 60*time.Second),
			waitPrometheusRuleDeletedTimeout: fallbackDuration(c.WaitPrometheusRuleDeletedTimeout, 60*time.Second),
			waitNamespaceDeletedTimeout:      fallbackDuration(c.WaitNamespaceDeletedTimeout, 60*time.Second),
		},
		silenceName:             c.SilenceName,
		alertName:               c.AlertName,
		alertLabelKey:           c.AlertLabelKey,
		alertLabelValue:         c.AlertLabelValue,
		waitAlertPresentTimeout: fallbackDuration(c.WaitAlertPresentTimeout, 60*time.Second),
	}

	return withTimeout(checker, fallbackDuration(c.Timeout, 5*time.Minute))
}

// observabilityLifecycleBase contains shared fields and methods for observability lifecycle checkers.
type observabilityLifecycleBase struct {
	access           kubernetes.Access
	preflightChecker check.Checker

	agentID            string
	namespace          string
	probeName          string
	rulesGroupName     string
	prometheusRuleName string

	requestTimeout                   time.Duration
	waitPrometheusRuleCreatedTimeout time.Duration
	waitPrometheusRuleDeletedTimeout time.Duration
	waitNamespaceDeletedTimeout      time.Duration
}

func (b *observabilityLifecycleBase) preflight() check.Error {
	if b.preflightChecker != nil {
		if err := b.preflightChecker.Check(); err != nil {
			return check.ErrUnknown("preflight: %v", err)
		}
	}
	return nil
}

func (b *observabilityLifecycleBase) createNamespace(ctx context.Context) error {
	ns := &v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: b.namespace,
			Labels: map[string]string{
				"heritage":      "upmeter",
				agentLabelKey:   b.agentID,
				"upmeter-group": "monitoring-and-autoscaling",
				"upmeter-probe": b.probeName,
			},
		},
	}
	_, err := b.access.Kubernetes().CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	return err
}

func (b *observabilityLifecycleBase) deleteNamespace(ctx context.Context) error {
	return b.access.Kubernetes().CoreV1().Namespaces().Delete(ctx, b.namespace, metav1.DeleteOptions{})
}

func (b *observabilityLifecycleBase) createRulesGroupFromManifest(ctx context.Context, manifest string) error {
	obj, err := decodeManifestToUnstructured(manifest)
	if err != nil {
		return err
	}
	_, err = b.access.Kubernetes().Dynamic().
		Resource(observabilityMetricsRulesGroupGVR).
		Namespace(b.namespace).
		Create(ctx, obj, metav1.CreateOptions{})
	return err
}

func (b *observabilityLifecycleBase) deleteRulesGroup(ctx context.Context) error {
	return b.access.Kubernetes().Dynamic().
		Resource(observabilityMetricsRulesGroupGVR).
		Namespace(b.namespace).
		Delete(ctx, b.rulesGroupName, metav1.DeleteOptions{})
}

func (b *observabilityLifecycleBase) rulesGroupExists(ctx context.Context) (bool, error) {
	_, err := b.access.Kubernetes().Dynamic().
		Resource(observabilityMetricsRulesGroupGVR).
		Namespace(b.namespace).
		Get(ctx, b.rulesGroupName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (b *observabilityLifecycleBase) waitPrometheusRulePresent(ctx context.Context) error {
	return waitForCondition(
		b.waitPrometheusRuleCreatedTimeout,
		pollingInterval(b.waitPrometheusRuleCreatedTimeout),
		func() (bool, error) {
			_, err := b.access.Kubernetes().Dynamic().
				Resource(prometheusRuleGVR).
				Namespace(observabilityNamespace).
				Get(ctx, b.prometheusRuleName, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			return true, nil
		},
	)
}

func (b *observabilityLifecycleBase) waitPrometheusRuleAbsent(ctx context.Context) error {
	return waitForCondition(
		b.waitPrometheusRuleDeletedTimeout,
		pollingInterval(b.waitPrometheusRuleDeletedTimeout),
		func() (bool, error) {
			_, err := b.access.Kubernetes().Dynamic().
				Resource(prometheusRuleGVR).
				Namespace(observabilityNamespace).
				Get(ctx, b.prometheusRuleName, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			if err != nil {
				return false, err
			}
			return false, nil
		},
	)
}

func (b *observabilityLifecycleBase) deletePrometheusRule(ctx context.Context) error {
	return b.access.Kubernetes().Dynamic().
		Resource(prometheusRuleGVR).
		Namespace(observabilityNamespace).
		Delete(ctx, b.prometheusRuleName, metav1.DeleteOptions{})
}

func (b *observabilityLifecycleBase) prometheusRuleExists(ctx context.Context) (bool, error) {
	_, err := b.access.Kubernetes().Dynamic().
		Resource(prometheusRuleGVR).
		Namespace(observabilityNamespace).
		Get(ctx, b.prometheusRuleName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (b *observabilityLifecycleBase) hasBaseGarbage(ctx context.Context) (bool, error) {
	if exists, err := namespaceExists(ctx, b.access, b.namespace); err != nil {
		return false, err
	} else if exists {
		return true, nil
	}

	if exists, err := b.prometheusRuleExists(ctx); err != nil {
		return false, err
	} else if exists {
		return true, nil
	}

	if exists, err := b.rulesGroupExists(ctx); err != nil {
		return false, err
	} else if exists {
		return true, nil
	}

	return false, nil
}

func (b *observabilityLifecycleBase) baseCleanup(ctx context.Context) error {
	var errs []error

	if err := b.deleteRulesGroup(ctx); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("delete ObservabilityMetricsRulesGroup: %w", err))
	}
	if err := b.deletePrometheusRule(ctx); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("delete PrometheusRule: %w", err))
	}
	if err := b.waitPrometheusRuleAbsent(ctx); err != nil {
		errs = append(errs, fmt.Errorf("wait PrometheusRule deletion: %w", err))
	}
	if err := b.deleteNamespace(ctx); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("delete namespace: %w", err))
	}
	if err := waitNamespaceNotFound(
		ctx,
		b.access,
		b.namespace,
		b.waitNamespaceDeletedTimeout,
		pollingInterval(b.waitNamespaceDeletedTimeout),
	); err != nil {
		errs = append(errs, fmt.Errorf("wait namespace deletion: %w", err))
	}

	return errors.Join(errs...)
}

type observabilityRulesGroupRecordingLifecycleChecker struct {
	observabilityLifecycleBase

	recordingMetric          string
	prometheusEndpoint       string
	waitMetricPresentTimeout time.Duration
}

func (c *observabilityRulesGroupRecordingLifecycleChecker) Check() check.Error {
	ctx := context.Background()

	if err := c.preflight(); err != nil {
		return err
	}

	hasGarbage, err := c.hasBaseGarbage(ctx)
	if err != nil {
		return check.ErrUnknown("checking garbage: %v", err)
	}
	if hasGarbage {
		if cleanupErr := c.cleanup(ctx); cleanupErr != nil {
			return check.ErrUnknown("cleaning garbage: %v", cleanupErr)
		}
		return check.ErrUnknown("cleaned garbage")
	}

	result := c.doRecordingLifecycle(ctx)
	return wrapCleanupResult(result, c.cleanup(ctx))
}

func (c *observabilityRulesGroupRecordingLifecycleChecker) doRecordingLifecycle(ctx context.Context) check.Error {
	if err := c.createNamespace(ctx); err != nil {
		return lifecycleStepError("creating namespace", err)
	}

	manifest := recordingRulesGroupManifest(c.agentID, c.namespace, c.rulesGroupName, c.recordingMetric)
	if err := c.createRulesGroupFromManifest(ctx, manifest); err != nil {
		return lifecycleStepError("creating ObservabilityMetricsRulesGroup", err)
	}

	if err := c.waitPrometheusRulePresent(ctx); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: PrometheusRule did not appear")
		}
		return lifecycleStepError("waiting for PrometheusRule creation", err)
	}

	if err := c.waitRecordingMetricPresent(); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: recording metric %q is missing in Prometheus", c.recordingMetric)
		}
		return lifecycleStepError("waiting for recording metric in Prometheus", err)
	}

	if err := c.deleteRulesGroup(ctx); err != nil && !apierrors.IsNotFound(err) {
		return lifecycleStepError("deleting ObservabilityMetricsRulesGroup", err)
	}

	if err := c.waitPrometheusRuleAbsent(ctx); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: PrometheusRule is still present after deleting ObservabilityMetricsRulesGroup")
		}
		return lifecycleStepError("waiting for PrometheusRule deletion", err)
	}

	return nil
}

func (c *observabilityRulesGroupRecordingLifecycleChecker) cleanup(ctx context.Context) error {
	return c.baseCleanup(ctx)
}

func (c *observabilityRulesGroupRecordingLifecycleChecker) waitRecordingMetricPresent() error {
	endpoint := addMetricQuery(c.prometheusEndpoint, c.recordingMetric)

	return waitForCondition(
		c.waitMetricPresentTimeout,
		pollingInterval(c.waitMetricPresentTimeout),
		func() (bool, error) {
			body, err := queryEndpoint(c.access, endpoint, c.requestTimeout)
			if err != nil {
				return false, err
			}

			return isMetricPresentInPrometheusResponse(body)
		},
	)
}

type observabilityRulesGroupAlertLifecycleChecker struct {
	observabilityLifecycleBase

	silenceName     string
	alertName       string
	alertLabelKey   string
	alertLabelValue string

	waitAlertPresentTimeout time.Duration
}

func (c *observabilityRulesGroupAlertLifecycleChecker) Check() check.Error {
	ctx := context.Background()

	if err := c.preflight(); err != nil {
		return err
	}

	if err := c.alertKubeAPIAvailable(ctx); err != nil {
		return err
	}

	hasGarbage, err := c.hasGarbage(ctx)
	if err != nil {
		return check.ErrUnknown("checking garbage: %v", err)
	}
	if hasGarbage {
		if cleanupErr := c.cleanup(ctx); cleanupErr != nil {
			return check.ErrUnknown("cleaning garbage: %v", cleanupErr)
		}
		return check.ErrUnknown("cleaned garbage")
	}

	result := c.doAlertLifecycle(ctx)
	return wrapCleanupResult(result, c.cleanup(ctx))
}

func (c *observabilityRulesGroupAlertLifecycleChecker) doAlertLifecycle(ctx context.Context) check.Error {
	if err := c.createNamespace(ctx); err != nil {
		return lifecycleStepError("creating namespace", err)
	}

	if err := c.createSilence(ctx); err != nil {
		return lifecycleStepError("creating ObservabilityNotificationSilence", err)
	}

	manifest := alertRulesGroupManifest(c.agentID, c.namespace, c.rulesGroupName, c.alertName, c.alertLabelKey, c.alertLabelValue)
	if err := c.createRulesGroupFromManifest(ctx, manifest); err != nil {
		return lifecycleStepError("creating ObservabilityMetricsRulesGroup", err)
	}

	if err := c.waitPrometheusRulePresent(ctx); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: PrometheusRule did not appear")
		}
		return lifecycleStepError("waiting for PrometheusRule creation", err)
	}

	if err := c.waitAlertPresentAndSilenced(ctx); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: alert %q did not appear as silenced in Alertmanager", c.alertName)
		}
		return lifecycleStepError("waiting for alert in Alertmanager", err)
	}

	if err := c.deleteRulesGroup(ctx); err != nil && !apierrors.IsNotFound(err) {
		return lifecycleStepError("deleting ObservabilityMetricsRulesGroup", err)
	}

	if err := c.waitPrometheusRuleAbsent(ctx); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: PrometheusRule is still present after deleting ObservabilityMetricsRulesGroup")
		}
		return lifecycleStepError("waiting for PrometheusRule deletion", err)
	}

	return nil
}

func (c *observabilityRulesGroupAlertLifecycleChecker) hasGarbage(ctx context.Context) (bool, error) {
	if has, err := c.hasBaseGarbage(ctx); has || err != nil {
		return has, err
	}
	return c.silenceExists(ctx)
}

func (c *observabilityRulesGroupAlertLifecycleChecker) cleanup(ctx context.Context) error {
	baseErr := c.baseCleanup(ctx)
	var silenceErr error
	if err := c.deleteSilence(ctx); err != nil && !apierrors.IsNotFound(err) {
		silenceErr = fmt.Errorf("delete ObservabilityNotificationSilence: %w", err)
	}
	return errors.Join(baseErr, silenceErr)
}

func (c *observabilityRulesGroupAlertLifecycleChecker) createSilence(ctx context.Context) error {
	startsAt := time.Now().UTC().Add(-1 * time.Minute)
	endsAt := time.Now().UTC().Add(20 * time.Minute)
	manifest := observabilitySilenceManifest(c.agentID, c.namespace, c.silenceName, c.alertLabelKey, c.alertLabelValue, startsAt, endsAt)

	obj, err := decodeManifestToUnstructured(manifest)
	if err != nil {
		return err
	}

	_, err = c.access.Kubernetes().Dynamic().
		Resource(observabilityNotificationSilenceGVR).
		Namespace(c.namespace).
		Create(ctx, obj, metav1.CreateOptions{})
	return err
}

func (c *observabilityRulesGroupAlertLifecycleChecker) deleteSilence(ctx context.Context) error {
	return c.access.Kubernetes().Dynamic().
		Resource(observabilityNotificationSilenceGVR).
		Namespace(c.namespace).
		Delete(ctx, c.silenceName, metav1.DeleteOptions{})
}

func (c *observabilityRulesGroupAlertLifecycleChecker) silenceExists(ctx context.Context) (bool, error) {
	_, err := c.access.Kubernetes().Dynamic().
		Resource(observabilityNotificationSilenceGVR).
		Namespace(c.namespace).
		Get(ctx, c.silenceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *observabilityRulesGroupAlertLifecycleChecker) alertKubeAPIAvailable(ctx context.Context) check.Error {
	podList, err := c.access.Kubernetes().CoreV1().
		Pods(observabilityNamespace).
		List(ctx, metav1.ListOptions{LabelSelector: "app=alert-kube-api"})
	if err != nil {
		return check.ErrUnknown("cannot check alert-kube-api pods: %v", err)
	}
	for _, pod := range podList.Items {
		if isPodReady(&pod) {
			return nil
		}
	}
	return check.ErrUnknown("alert-kube-api is not available")
}

func (c *observabilityRulesGroupAlertLifecycleChecker) waitAlertPresentAndSilenced(ctx context.Context) error {
	return waitForCondition(
		c.waitAlertPresentTimeout,
		pollingInterval(c.waitAlertPresentTimeout),
		func() (bool, error) {
			list, err := c.access.Kubernetes().Dynamic().
				Resource(observabilityAlertGVR).
				Namespace(c.namespace).
				List(ctx, metav1.ListOptions{})
			if err != nil {
				return false, err
			}

			for _, item := range list.Items {
				labels, found, err := unstructured.NestedStringMap(item.Object, "alert", "labels")
				if err != nil || !found {
					continue
				}
				if labels["alertname"] != c.alertName {
					continue
				}
				if labels[c.alertLabelKey] != c.alertLabelValue {
					continue
				}

				silencedBy, _, _ := unstructured.NestedStringSlice(item.Object, "status", "silencedBy")
				return len(silencedBy) > 0, nil
			}

			return false, nil
		},
	)
}

// wrapCleanupResult combines the check result with cleanup error, preserving the original status.
func wrapCleanupResult(res check.Error, cleanupErr error) check.Error {
	if cleanupErr == nil {
		return res
	}

	if res == nil {
		return check.ErrUnknown("cleanup: %v", cleanupErr)
	}

	if res.Status() == check.Down {
		return check.ErrFail("%s; cleanup: %v", res.Error(), cleanupErr)
	}

	return check.ErrUnknown("%s; cleanup: %v", res.Error(), cleanupErr)
}

// lifecycleStepError wraps an error with step context, mapping API errors to check.ErrFail.
func lifecycleStepError(step string, err error) check.Error {
	if err == nil {
		return nil
	}

	if checkErr, ok := err.(check.Error); ok {
		if checkErr.Status() == check.Down {
			return check.ErrFail("%s: %v", step, err)
		}
		return check.ErrUnknown("%s: %v", step, err)
	}

	if apierrors.IsForbidden(err) ||
		apierrors.IsUnauthorized(err) ||
		apierrors.IsInvalid(err) ||
		apierrors.IsBadRequest(err) ||
		apierrors.IsAlreadyExists(err) ||
		apierrors.IsNotFound(err) {
		return check.ErrFail("%s: %v", step, err)
	}

	return check.ErrUnknown("%s: %v", step, err)
}

func expectedPrometheusRuleName(namespace, rulesGroupName string) string {
	return fmt.Sprintf("%s-%s", namespace, rulesGroupName)
}

func recordingRulesGroupManifest(agentID, namespace, name, metric string) string {
	return fmt.Sprintf(`
apiVersion: observability.deckhouse.io/v1alpha1
kind: ObservabilityMetricsRulesGroup
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %s
    upmeter-group: monitoring-and-autoscaling
    upmeter-probe: observability-recording
  name: %s
  namespace: %s
spec:
  interval: "30s"
  rules:
  - record: %s
    expr: kube_namespace_created
`, agentID, name, namespace, metric)
}

func alertRulesGroupManifest(agentID, namespace, name, alertName, alertLabelKey, alertLabelValue string) string {
	return fmt.Sprintf(`
apiVersion: observability.deckhouse.io/v1alpha1
kind: ObservabilityMetricsRulesGroup
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %s
    upmeter-group: monitoring-and-autoscaling
    upmeter-probe: alertmanager
  name: %s
  namespace: %s
spec:
  interval: "30s"
  rules:
  - alert: %s
    expr: kube_namespace_created > 0
    labels:
      severity: warning
      %s: %s
    annotations:
      summary: "upmeter observability mini e2e alert"
`, agentID, name, namespace, alertName, alertLabelKey, alertLabelValue)
}

func observabilitySilenceManifest(agentID, namespace, name, alertLabelKey, alertLabelValue string, startsAt, endsAt time.Time) string {
	return fmt.Sprintf(`
apiVersion: observability.deckhouse.io/v1alpha1
kind: ObservabilityNotificationSilence
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %s
    upmeter-group: monitoring-and-autoscaling
    upmeter-probe: alertmanager
  name: %s
  namespace: %s
spec:
  startsAt: "%s"
  endsAt: "%s"
  selector:
    matchLabels:
      %s: %s
`,
		agentID,
		name,
		namespace,
		startsAt.Format(time.RFC3339),
		endsAt.Format(time.RFC3339),
		alertLabelKey,
		alertLabelValue,
	)
}
