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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"d8.io/upmeter/pkg/check"
)

func Test_expectedPrometheusRuleName(t *testing.T) {
	assert.Equal(t, "test-ns-test-rules", expectedPrometheusRuleName("test-ns", "test-rules"))
}

func Test_recordingRulesGroupManifest(t *testing.T) {
	manifest := recordingRulesGroupManifest("agent-01", "test-ns", "recording-rules", "upmeter_metric")

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	assert.NoError(t, err)

	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "recording-rules", metadata["name"])
	assert.Equal(t, "test-ns", metadata["namespace"])

	spec := obj["spec"].(map[string]interface{})
	assert.Equal(t, "30s", spec["interval"])

	rules := spec["rules"].([]interface{})
	assert.Len(t, rules, 1)
	rule := rules[0].(map[string]interface{})
	assert.Equal(t, "upmeter_metric", rule["record"])
	assert.Equal(t, "kube_namespace_created", rule["expr"])
}

func Test_alertRulesGroupManifest(t *testing.T) {
	manifest := alertRulesGroupManifest(
		"agent-01",
		"test-ns",
		"alert-rules",
		"UpmeterMiniE2E",
		"upmeter_alert_id",
		"abc-123",
	)

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	assert.NoError(t, err)

	spec := obj["spec"].(map[string]interface{})
	rules := spec["rules"].([]interface{})
	assert.Len(t, rules, 1)

	rule := rules[0].(map[string]interface{})
	assert.Equal(t, "UpmeterMiniE2E", rule["alert"])
	assert.Equal(t, "kube_namespace_created > 0", rule["expr"])

	labels := rule["labels"].(map[string]interface{})
	assert.Equal(t, "warning", labels["severity"])
	assert.Equal(t, "abc-123", labels["upmeter_alert_id"])
}

func Test_observabilitySilenceManifest(t *testing.T) {
	startsAt := time.Date(2026, time.February, 27, 9, 0, 0, 0, time.UTC)
	endsAt := startsAt.Add(10 * time.Minute)

	manifest := observabilitySilenceManifest(
		"agent-01",
		"test-ns",
		"silence-1",
		"upmeter_alert_id",
		"abc-123",
		startsAt,
		endsAt,
	)

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	assert.NoError(t, err)

	spec := obj["spec"].(map[string]interface{})
	assert.Equal(t, startsAt.Format(time.RFC3339), spec["startsAt"])
	assert.Equal(t, endsAt.Format(time.RFC3339), spec["endsAt"])

	selector := spec["selector"].(map[string]interface{})
	matchLabels := selector["matchLabels"].(map[string]interface{})
	assert.Equal(t, "abc-123", matchLabels["upmeter_alert_id"])
}

func Test_isMetricPresentInPrometheusResponse(t *testing.T) {
	t.Run("metric is present", func(t *testing.T) {
		body := []byte(`{
  "status":"success",
  "data":{
    "resultType":"vector",
    "result":[{"metric":{},"value":[123,"2"]}]
  }
}`)
		present, err := isMetricPresentInPrometheusResponse(body)
		assert.NoError(t, err)
		assert.True(t, present)
	})

	t.Run("metric is absent", func(t *testing.T) {
		body := []byte(`{
  "status":"success",
  "data":{"resultType":"vector","result":[]}
}`)
		present, err := isMetricPresentInPrometheusResponse(body)
		assert.NoError(t, err)
		assert.False(t, present)
	})

	t.Run("invalid payload", func(t *testing.T) {
		body := []byte(`{"status":"success","data":{"result":"broken"}}`)
		present, err := isMetricPresentInPrometheusResponse(body)
		assert.Error(t, err)
		assert.False(t, present)
	})
}

func Test_lifecycleStepError(t *testing.T) {
	t.Run("wraps check fail as fail", func(t *testing.T) {
		err := lifecycleStepError("step", check.ErrFail("boom"))
		assert.Equal(t, check.Down, err.Status())
		assert.Contains(t, err.Error(), "step")
	})

	t.Run("wraps check unknown as unknown", func(t *testing.T) {
		err := lifecycleStepError("step", check.ErrUnknown("boom"))
		assert.Equal(t, check.Unknown, err.Status())
		assert.Contains(t, err.Error(), "step")
	})

	t.Run("forbidden is fail", func(t *testing.T) {
		err := lifecycleStepError(
			"step",
			apierrors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "deployments"}, "x", errors.New("forbidden")),
		)
		assert.Equal(t, check.Down, err.Status())
	})

	t.Run("service unavailable is unknown", func(t *testing.T) {
		err := lifecycleStepError("step", apierrors.NewServiceUnavailable("temporarily unavailable"))
		assert.Equal(t, check.Unknown, err.Status())
	})
}
