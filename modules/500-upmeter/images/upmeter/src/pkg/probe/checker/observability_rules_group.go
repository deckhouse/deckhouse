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
	"strings"
	"time"

	"github.com/tidwall/gjson"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	serializeryaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

var (
	errConditionTimeout = errors.New("condition timeout")

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
)

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
		access:                           c.Access,
		preflightChecker:                 c.PreflightChecker,
		agentID:                          fallbackString(c.AgentID, "unknown"),
		namespace:                        c.Namespace,
		rulesGroupName:                   c.RulesGroupName,
		prometheusRuleName:               expectedPrometheusRuleName(c.Namespace, c.RulesGroupName),
		recordingMetric:                  c.RecordingMetric,
		prometheusEndpoint:               c.PrometheusEndpoint,
		requestTimeout:                   fallbackDuration(c.RequestTimeout, 5*time.Second),
		waitPrometheusRuleCreatedTimeout: fallbackDuration(c.WaitPrometheusRuleCreatedTimeout, 90*time.Second),
		waitMetricPresentTimeout:         fallbackDuration(c.WaitMetricPresentTimeout, 90*time.Second),
		waitPrometheusRuleDeletedTimeout: fallbackDuration(c.WaitPrometheusRuleDeletedTimeout, 90*time.Second),
		waitNamespaceDeletedTimeout:      fallbackDuration(c.WaitNamespaceDeletedTimeout, 90*time.Second),
	}

	return withTimeout(checker, fallbackDuration(c.Timeout, 5*time.Minute))
}

type observabilityRulesGroupRecordingLifecycleChecker struct {
	access           kubernetes.Access
	preflightChecker check.Checker

	agentID            string
	namespace          string
	rulesGroupName     string
	prometheusRuleName string

	recordingMetric    string
	prometheusEndpoint string

	requestTimeout                   time.Duration
	waitPrometheusRuleCreatedTimeout time.Duration
	waitMetricPresentTimeout         time.Duration
	waitPrometheusRuleDeletedTimeout time.Duration
	waitNamespaceDeletedTimeout      time.Duration
}

func (c *observabilityRulesGroupRecordingLifecycleChecker) Check() (res check.Error) {
	ctx := context.TODO()

	if c.preflightChecker != nil {
		if err := c.preflightChecker.Check(); err != nil {
			return check.ErrUnknown("preflight: %v", err)
		}
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

	defer func() {
		res = wrapCleanupResult(res, c.cleanup(ctx))
	}()

	if err := c.createNamespace(ctx); err != nil {
		return check.ErrUnknown("creating namespace: %v", err)
	}

	if err := c.createRulesGroup(ctx); err != nil {
		return check.ErrUnknown("creating ObservabilityMetricsRulesGroup: %v", err)
	}

	if err := c.waitPrometheusRulePresent(ctx); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: PrometheusRule did not appear")
		}
		return check.ErrUnknown("waiting for PrometheusRule creation: %v", err)
	}

	if err := c.waitRecordingMetricPresent(); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: recording metric %q is missing in Prometheus", c.recordingMetric)
		}
		return check.ErrUnknown("waiting for recording metric in Prometheus: %v", err)
	}

	if err := c.deleteRulesGroup(ctx); err != nil && !apierrors.IsNotFound(err) {
		return check.ErrUnknown("deleting ObservabilityMetricsRulesGroup: %v", err)
	}

	if err := c.waitPrometheusRuleAbsent(ctx); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: PrometheusRule is still present after deleting ObservabilityMetricsRulesGroup")
		}
		return check.ErrUnknown("waiting for PrometheusRule deletion: %v", err)
	}

	return nil
}

func (c *observabilityRulesGroupRecordingLifecycleChecker) hasGarbage(ctx context.Context) (bool, error) {
	if exists, err := namespaceExists(ctx, c.access, c.namespace); err != nil {
		return false, err
	} else if exists {
		return true, nil
	}

	if exists, err := c.prometheusRuleExists(ctx); err != nil {
		return false, err
	} else if exists {
		return true, nil
	}

	if exists, err := c.rulesGroupExists(ctx); err != nil {
		return false, err
	} else if exists {
		return true, nil
	}

	return false, nil
}

func (c *observabilityRulesGroupRecordingLifecycleChecker) cleanup(ctx context.Context) error {
	errs := make([]string, 0)

	if err := c.deleteRulesGroup(ctx); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Sprintf("delete ObservabilityMetricsRulesGroup: %v", err))
	}
	if err := c.deletePrometheusRule(ctx); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Sprintf("delete PrometheusRule: %v", err))
	}
	if err := c.waitPrometheusRuleAbsent(ctx); err != nil {
		errs = append(errs, fmt.Sprintf("wait PrometheusRule deletion: %v", err))
	}
	if err := c.deleteNamespace(ctx); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Sprintf("delete namespace: %v", err))
	}
	if err := waitNamespaceAbsent(
		ctx,
		c.access,
		c.namespace,
		c.waitNamespaceDeletedTimeout,
		pollingInterval(c.waitNamespaceDeletedTimeout),
	); err != nil {
		errs = append(errs, fmt.Sprintf("wait namespace deletion: %v", err))
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, "; "))
}

func (c *observabilityRulesGroupRecordingLifecycleChecker) createNamespace(ctx context.Context) error {
	ns := &v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: c.namespace,
			Labels: map[string]string{
				"heritage":      "upmeter",
				agentLabelKey:   c.agentID,
				"upmeter-group": "monitoring-and-autoscaling",
				"upmeter-probe": "prometheus",
			},
		},
	}
	_, err := c.access.Kubernetes().CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	return err
}

func (c *observabilityRulesGroupRecordingLifecycleChecker) deleteNamespace(ctx context.Context) error {
	return c.access.Kubernetes().CoreV1().Namespaces().Delete(ctx, c.namespace, metav1.DeleteOptions{})
}

func (c *observabilityRulesGroupRecordingLifecycleChecker) createRulesGroup(ctx context.Context) error {
	manifest := recordingRulesGroupManifest(c.agentID, c.namespace, c.rulesGroupName, c.recordingMetric)
	obj, err := decodeManifestToUnstructured(manifest)
	if err != nil {
		return err
	}

	_, err = c.access.Kubernetes().Dynamic().
		Resource(observabilityMetricsRulesGroupGVR).
		Namespace(c.namespace).
		Create(ctx, obj, metav1.CreateOptions{})
	return err
}

func (c *observabilityRulesGroupRecordingLifecycleChecker) deleteRulesGroup(ctx context.Context) error {
	return c.access.Kubernetes().Dynamic().
		Resource(observabilityMetricsRulesGroupGVR).
		Namespace(c.namespace).
		Delete(ctx, c.rulesGroupName, metav1.DeleteOptions{})
}

func (c *observabilityRulesGroupRecordingLifecycleChecker) rulesGroupExists(ctx context.Context) (bool, error) {
	_, err := c.access.Kubernetes().Dynamic().
		Resource(observabilityMetricsRulesGroupGVR).
		Namespace(c.namespace).
		Get(ctx, c.rulesGroupName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *observabilityRulesGroupRecordingLifecycleChecker) waitPrometheusRulePresent(ctx context.Context) error {
	return waitForCondition(
		c.waitPrometheusRuleCreatedTimeout,
		pollingInterval(c.waitPrometheusRuleCreatedTimeout),
		func() (bool, error) {
			_, err := c.access.Kubernetes().Dynamic().
				Resource(prometheusRuleGVR).
				Namespace("d8-observability").
				Get(ctx, c.prometheusRuleName, metav1.GetOptions{})
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

func (c *observabilityRulesGroupRecordingLifecycleChecker) waitPrometheusRuleAbsent(ctx context.Context) error {
	return waitForCondition(
		c.waitPrometheusRuleDeletedTimeout,
		pollingInterval(c.waitPrometheusRuleDeletedTimeout),
		func() (bool, error) {
			_, err := c.access.Kubernetes().Dynamic().
				Resource(prometheusRuleGVR).
				Namespace("d8-observability").
				Get(ctx, c.prometheusRuleName, metav1.GetOptions{})
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

func (c *observabilityRulesGroupRecordingLifecycleChecker) deletePrometheusRule(ctx context.Context) error {
	return c.access.Kubernetes().Dynamic().
		Resource(prometheusRuleGVR).
		Namespace("d8-observability").
		Delete(ctx, c.prometheusRuleName, metav1.DeleteOptions{})
}

func (c *observabilityRulesGroupRecordingLifecycleChecker) prometheusRuleExists(ctx context.Context) (bool, error) {
	_, err := c.access.Kubernetes().Dynamic().
		Resource(prometheusRuleGVR).
		Namespace("d8-observability").
		Get(ctx, c.prometheusRuleName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
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

			present, err := isMetricPresentInPrometheusResponse(body)
			if err != nil {
				return false, err
			}

			return present, nil
		},
	)
}

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

	AlertmanagerEndpoint string

	RequestTimeout                   time.Duration
	WaitPrometheusRuleCreatedTimeout time.Duration
	WaitAlertPresentTimeout          time.Duration
	WaitPrometheusRuleDeletedTimeout time.Duration
	WaitNamespaceDeletedTimeout      time.Duration
	Timeout                          time.Duration
}

func (c ObservabilityRulesGroupAlertLifecycle) Checker() check.Checker {
	checker := &observabilityRulesGroupAlertLifecycleChecker{
		access:                           c.Access,
		preflightChecker:                 c.PreflightChecker,
		agentID:                          fallbackString(c.AgentID, "unknown"),
		namespace:                        c.Namespace,
		rulesGroupName:                   c.RulesGroupName,
		prometheusRuleName:               expectedPrometheusRuleName(c.Namespace, c.RulesGroupName),
		silenceName:                      c.SilenceName,
		alertName:                        c.AlertName,
		alertLabelKey:                    c.AlertLabelKey,
		alertLabelValue:                  c.AlertLabelValue,
		alertmanagerEndpoint:             c.AlertmanagerEndpoint,
		requestTimeout:                   fallbackDuration(c.RequestTimeout, 5*time.Second),
		waitPrometheusRuleCreatedTimeout: fallbackDuration(c.WaitPrometheusRuleCreatedTimeout, 90*time.Second),
		waitAlertPresentTimeout:          fallbackDuration(c.WaitAlertPresentTimeout, 90*time.Second),
		waitPrometheusRuleDeletedTimeout: fallbackDuration(c.WaitPrometheusRuleDeletedTimeout, 90*time.Second),
		waitNamespaceDeletedTimeout:      fallbackDuration(c.WaitNamespaceDeletedTimeout, 90*time.Second),
	}

	return withTimeout(checker, fallbackDuration(c.Timeout, 5*time.Minute))
}

type observabilityRulesGroupAlertLifecycleChecker struct {
	access           kubernetes.Access
	preflightChecker check.Checker

	agentID            string
	namespace          string
	rulesGroupName     string
	prometheusRuleName string
	silenceName        string

	alertName       string
	alertLabelKey   string
	alertLabelValue string

	alertmanagerEndpoint string

	requestTimeout                   time.Duration
	waitPrometheusRuleCreatedTimeout time.Duration
	waitAlertPresentTimeout          time.Duration
	waitPrometheusRuleDeletedTimeout time.Duration
	waitNamespaceDeletedTimeout      time.Duration
}

func (c *observabilityRulesGroupAlertLifecycleChecker) Check() (res check.Error) {
	ctx := context.TODO()

	if c.preflightChecker != nil {
		if err := c.preflightChecker.Check(); err != nil {
			return check.ErrUnknown("preflight: %v", err)
		}
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

	defer func() {
		res = wrapCleanupResult(res, c.cleanup(ctx))
	}()

	if err := c.createNamespace(ctx); err != nil {
		return check.ErrUnknown("creating namespace: %v", err)
	}

	if err := c.createRulesGroup(ctx); err != nil {
		return check.ErrUnknown("creating ObservabilityMetricsRulesGroup: %v", err)
	}

	if err := c.createSilence(ctx); err != nil {
		return check.ErrUnknown("creating ObservabilityNotificationSilence: %v", err)
	}

	if err := c.waitPrometheusRulePresent(ctx); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: PrometheusRule did not appear")
		}
		return check.ErrUnknown("waiting for PrometheusRule creation: %v", err)
	}

	if err := c.waitAlertPresentAndSilenced(); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: alert %q did not appear as silenced in Alertmanager", c.alertName)
		}
		return check.ErrUnknown("waiting for alert in Alertmanager: %v", err)
	}

	if err := c.deleteRulesGroup(ctx); err != nil && !apierrors.IsNotFound(err) {
		return check.ErrUnknown("deleting ObservabilityMetricsRulesGroup: %v", err)
	}

	if err := c.waitPrometheusRuleAbsent(ctx); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: PrometheusRule is still present after deleting ObservabilityMetricsRulesGroup")
		}
		return check.ErrUnknown("waiting for PrometheusRule deletion: %v", err)
	}

	return nil
}

func (c *observabilityRulesGroupAlertLifecycleChecker) hasGarbage(ctx context.Context) (bool, error) {
	if exists, err := namespaceExists(ctx, c.access, c.namespace); err != nil {
		return false, err
	} else if exists {
		return true, nil
	}

	if exists, err := c.prometheusRuleExists(ctx); err != nil {
		return false, err
	} else if exists {
		return true, nil
	}

	if exists, err := c.rulesGroupExists(ctx); err != nil {
		return false, err
	} else if exists {
		return true, nil
	}

	if exists, err := c.silenceExists(ctx); err != nil {
		return false, err
	} else if exists {
		return true, nil
	}

	return false, nil
}

func (c *observabilityRulesGroupAlertLifecycleChecker) cleanup(ctx context.Context) error {
	errs := make([]string, 0)

	if err := c.deleteSilence(ctx); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Sprintf("delete ObservabilityNotificationSilence: %v", err))
	}
	if err := c.deleteRulesGroup(ctx); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Sprintf("delete ObservabilityMetricsRulesGroup: %v", err))
	}
	if err := c.deletePrometheusRule(ctx); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Sprintf("delete PrometheusRule: %v", err))
	}
	if err := c.waitPrometheusRuleAbsent(ctx); err != nil {
		errs = append(errs, fmt.Sprintf("wait PrometheusRule deletion: %v", err))
	}
	if err := c.deleteNamespace(ctx); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Sprintf("delete namespace: %v", err))
	}
	if err := waitNamespaceAbsent(
		ctx,
		c.access,
		c.namespace,
		c.waitNamespaceDeletedTimeout,
		pollingInterval(c.waitNamespaceDeletedTimeout),
	); err != nil {
		errs = append(errs, fmt.Sprintf("wait namespace deletion: %v", err))
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, "; "))
}

func (c *observabilityRulesGroupAlertLifecycleChecker) createNamespace(ctx context.Context) error {
	ns := &v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: c.namespace,
			Labels: map[string]string{
				"heritage":      "upmeter",
				agentLabelKey:   c.agentID,
				"upmeter-group": "monitoring-and-autoscaling",
				"upmeter-probe": "alertmanager",
			},
		},
	}
	_, err := c.access.Kubernetes().CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	return err
}

func (c *observabilityRulesGroupAlertLifecycleChecker) deleteNamespace(ctx context.Context) error {
	return c.access.Kubernetes().CoreV1().Namespaces().Delete(ctx, c.namespace, metav1.DeleteOptions{})
}

func (c *observabilityRulesGroupAlertLifecycleChecker) createRulesGroup(ctx context.Context) error {
	manifest := alertRulesGroupManifest(c.agentID, c.namespace, c.rulesGroupName, c.alertName, c.alertLabelKey, c.alertLabelValue)
	obj, err := decodeManifestToUnstructured(manifest)
	if err != nil {
		return err
	}

	_, err = c.access.Kubernetes().Dynamic().
		Resource(observabilityMetricsRulesGroupGVR).
		Namespace(c.namespace).
		Create(ctx, obj, metav1.CreateOptions{})
	return err
}

func (c *observabilityRulesGroupAlertLifecycleChecker) deleteRulesGroup(ctx context.Context) error {
	return c.access.Kubernetes().Dynamic().
		Resource(observabilityMetricsRulesGroupGVR).
		Namespace(c.namespace).
		Delete(ctx, c.rulesGroupName, metav1.DeleteOptions{})
}

func (c *observabilityRulesGroupAlertLifecycleChecker) rulesGroupExists(ctx context.Context) (bool, error) {
	_, err := c.access.Kubernetes().Dynamic().
		Resource(observabilityMetricsRulesGroupGVR).
		Namespace(c.namespace).
		Get(ctx, c.rulesGroupName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *observabilityRulesGroupAlertLifecycleChecker) createSilence(ctx context.Context) error {
	startsAt := time.Now().UTC().Add(-1 * time.Minute)
	endsAt := time.Now().UTC().Add(10 * time.Minute)
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

func (c *observabilityRulesGroupAlertLifecycleChecker) waitPrometheusRulePresent(ctx context.Context) error {
	return waitForCondition(
		c.waitPrometheusRuleCreatedTimeout,
		pollingInterval(c.waitPrometheusRuleCreatedTimeout),
		func() (bool, error) {
			_, err := c.access.Kubernetes().Dynamic().
				Resource(prometheusRuleGVR).
				Namespace("d8-observability").
				Get(ctx, c.prometheusRuleName, metav1.GetOptions{})
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

func (c *observabilityRulesGroupAlertLifecycleChecker) waitPrometheusRuleAbsent(ctx context.Context) error {
	return waitForCondition(
		c.waitPrometheusRuleDeletedTimeout,
		pollingInterval(c.waitPrometheusRuleDeletedTimeout),
		func() (bool, error) {
			_, err := c.access.Kubernetes().Dynamic().
				Resource(prometheusRuleGVR).
				Namespace("d8-observability").
				Get(ctx, c.prometheusRuleName, metav1.GetOptions{})
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

func (c *observabilityRulesGroupAlertLifecycleChecker) deletePrometheusRule(ctx context.Context) error {
	return c.access.Kubernetes().Dynamic().
		Resource(prometheusRuleGVR).
		Namespace("d8-observability").
		Delete(ctx, c.prometheusRuleName, metav1.DeleteOptions{})
}

func (c *observabilityRulesGroupAlertLifecycleChecker) prometheusRuleExists(ctx context.Context) (bool, error) {
	_, err := c.access.Kubernetes().Dynamic().
		Resource(prometheusRuleGVR).
		Namespace("d8-observability").
		Get(ctx, c.prometheusRuleName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *observabilityRulesGroupAlertLifecycleChecker) waitAlertPresentAndSilenced() error {
	return waitForCondition(
		c.waitAlertPresentTimeout,
		pollingInterval(c.waitAlertPresentTimeout),
		func() (bool, error) {
			body, err := queryEndpoint(c.access, c.alertmanagerEndpoint, c.requestTimeout)
			if err != nil {
				return false, err
			}

			found, silenced, err := hasAlertInAlertmanagerResponse(body, c.alertName, c.alertLabelKey, c.alertLabelValue)
			if err != nil {
				return false, err
			}

			return found && silenced, nil
		},
	)
}

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

func waitForCondition(timeout, interval time.Duration, condition func() (bool, error)) error {
	if interval <= 0 {
		interval = time.Second
	}

	deadline := time.Now().Add(timeout)
	for {
		done, err := condition()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		if time.Now().After(deadline) {
			return errConditionTimeout
		}
		time.Sleep(interval)
	}
}

func waitNamespaceAbsent(ctx context.Context, access kubernetes.Access, namespace string, timeout, interval time.Duration) error {
	return waitForCondition(timeout, interval, func() (bool, error) {
		_, err := access.Kubernetes().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	})
}

func namespaceExists(ctx context.Context, access kubernetes.Access, namespace string) (bool, error) {
	_, err := access.Kubernetes().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func queryEndpoint(access kubernetes.Access, endpoint string, timeout time.Duration) ([]byte, error) {
	req, err := newGetRequest(endpoint, access.ServiceAccountToken(), access.UserAgent())
	if err != nil {
		return nil, err
	}

	client := newInsecureClient(3 * timeout)
	body, reqErr := doRequest(client, req)
	if reqErr != nil {
		return nil, reqErr
	}

	return body, nil
}

func isMetricPresentInPrometheusResponse(body []byte) (bool, error) {
	resultPath := "data.result"
	result := gjson.GetBytes(body, resultPath)
	if !result.IsArray() {
		return false, fmt.Errorf("cannot parse path %q in Prometheus response", resultPath)
	}

	if len(result.Array()) == 0 {
		return false, nil
	}

	countPath := "data.result.0.value.1"
	count := gjson.GetBytes(body, countPath)
	if !count.Exists() {
		return false, nil
	}

	return count.String() != "0", nil
}

func hasAlertInAlertmanagerResponse(body []byte, alertName, labelKey, labelValue string) (bool, bool, error) {
	alerts := gjson.ParseBytes(body)
	if !alerts.IsArray() {
		return false, false, fmt.Errorf("cannot parse Alertmanager response as array")
	}

	found := false
	labelPath := "labels." + labelKey
	for _, alert := range alerts.Array() {
		if alert.Get("labels.alertname").String() != alertName {
			continue
		}
		if alert.Get(labelPath).String() != labelValue {
			continue
		}

		found = true
		silencedBy := alert.Get("status.silencedBy")
		if silencedBy.IsArray() && len(silencedBy.Array()) > 0 {
			return true, true, nil
		}
	}

	return found, false, nil
}

func decodeManifestToUnstructured(manifest string) (*unstructured.Unstructured, error) {
	dec := serializeryaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}

	if _, _, err := dec.Decode([]byte(manifest), nil, obj); err != nil {
		return nil, err
	}

	return obj, nil
}

func expectedPrometheusRuleName(namespace, rulesGroupName string) string {
	return fmt.Sprintf("%s-%s", namespace, rulesGroupName)
}

func recordingRulesGroupManifest(agentID, namespace, name, metric string) string {
	tpl := `
apiVersion: observability.deckhouse.io/v1alpha1
kind: ObservabilityMetricsRulesGroup
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %q
    upmeter-group: monitoring-and-autoscaling
    upmeter-probe: prometheus
  name: %q
  namespace: %q
spec:
  interval: "30s"
  rules:
  - record: %q
    expr: kube_namespace_created
`

	return fmt.Sprintf(tpl, agentID, name, namespace, metric)
}

func alertRulesGroupManifest(agentID, namespace, name, alertName, alertLabelKey, alertLabelValue string) string {
	tpl := `
apiVersion: observability.deckhouse.io/v1alpha1
kind: ObservabilityMetricsRulesGroup
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %q
    upmeter-group: monitoring-and-autoscaling
    upmeter-probe: alertmanager
  name: %q
  namespace: %q
spec:
  interval: "30s"
  rules:
  - alert: %q
    expr: kube_namespace_created > 0
    labels:
      severity: warning
      %s: %q
    annotations:
      summary: "upmeter observability mini e2e alert"
`

	return fmt.Sprintf(tpl, agentID, name, namespace, alertName, alertLabelKey, alertLabelValue)
}

func observabilitySilenceManifest(agentID, namespace, name, alertLabelKey, alertLabelValue string, startsAt, endsAt time.Time) string {
	tpl := `
apiVersion: observability.deckhouse.io/v1alpha1
kind: ObservabilityNotificationSilence
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %q
    upmeter-group: monitoring-and-autoscaling
    upmeter-probe: alertmanager
  name: %q
  namespace: %q
spec:
  startsAt: %q
  endsAt: %q
  selector:
    matchLabels:
      %s: %q
`

	return fmt.Sprintf(
		tpl,
		agentID,
		name,
		namespace,
		startsAt.Format(time.RFC3339),
		endsAt.Format(time.RFC3339),
		alertLabelKey,
		alertLabelValue,
	)
}

func fallbackDuration(actual, fallback time.Duration) time.Duration {
	if actual <= 0 {
		return fallback
	}
	return actual
}

func fallbackString(actual, fallback string) string {
	if actual == "" {
		return fallback
	}
	return actual
}

func pollingInterval(timeout time.Duration) time.Duration {
	interval := timeout / 10
	if interval < time.Second {
		return time.Second
	}
	if interval > 5*time.Second {
		return 5 * time.Second
	}
	return interval
}
