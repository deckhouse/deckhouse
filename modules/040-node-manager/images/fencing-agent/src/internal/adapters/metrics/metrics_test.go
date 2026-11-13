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

package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"fencing-agent/internal/domain"
)

// series returns "label=value,..." of every series of the metric, with the value.
func series(t *testing.T, m *Metrics, name string) []string {
	t.Helper()

	families, err := m.registry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	var got []string

	for _, family := range families {
		if family.GetName() != name {
			continue
		}

		for _, metric := range family.GetMetric() {
			var labels []string
			for _, l := range metric.GetLabel() {
				labels = append(labels, l.GetName()+"="+l.GetValue())
			}

			value := metric.GetGauge().GetValue() + metric.GetCounter().GetValue()
			got = append(got, strings.Join(labels, ",")+" "+strconv.FormatFloat(value, 'f', -1, 64))
		}
	}

	slices.Sort(got)

	return got
}

func expect(t *testing.T, m *Metrics, name string, want ...string) {
	t.Helper()

	if got := series(t, m, name); !slices.Equal(got, want) {
		t.Errorf("%s = %q, want %q", name, got, want)
	}
}

func TestConditionHasOneSeries(t *testing.T) {
	m := New("w1")

	m.Condition(domain.ReasonWaitingForControlPlane)
	m.Condition(domain.ReasonKernelPanicMismatch)

	expect(t, m, "d8_fencing_agent_condition", "node=w1,reason=KernelPanicMismatch 1")
}

func TestCPSourceHasOneSeries(t *testing.T) {
	m := New("w1")

	m.CPSource(domain.CPSourceSelf)
	expect(t, m, "d8_fencing_agent_cp_source", "node=w1,source=self 1")

	m.CPSource(domain.CPSourceProxy)
	expect(t, m, "d8_fencing_agent_cp_source", "node=w1,source=proxy 1")
}

func TestDecisionsAndPolls(t *testing.T) {
	m := New("w1")

	m.Decision(domain.DecisionStop)
	m.Decision(domain.DecisionStop)
	m.Decision(domain.DecisionFeed)
	m.Poll()
	m.PollAnswer(domain.NetworkMain, domain.AnswerSilent)
	m.PollAnswer(domain.NetworkMain, domain.AnswerSupport)

	expect(t, m, "d8_fencing_agent_decisions_total", "decision=feed,node=w1 1", "decision=stop,node=w1 2")
	expect(t, m, "d8_fencing_agent_polls_total", "node=w1 1")
	expect(t, m, "d8_fencing_agent_poll_answers_total",
		"class=silent,network=main,node=w1 1", "class=support,network=main,node=w1 1")
}

func TestConfigInvalidSeriesIsRemovedWhenValueIsValid(t *testing.T) {
	m := New("w1")

	m.ConfigInvalid("minFullPollInterval", true)
	m.ConfigInvalid("key", true)
	expect(t, m, "d8_fencing_agent_config_invalid", "node=w1,parameter=key 1", "node=w1,parameter=minFullPollInterval 1")

	m.ConfigInvalid("minFullPollInterval", false)
	expect(t, m, "d8_fencing_agent_config_invalid", "node=w1,parameter=key 1")
}

func TestHandlerServesMetrics(t *testing.T) {
	m := New("w1")
	m.Condition(domain.ReasonArmed)

	srv := httptest.NewServer(m.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("get metrics: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	want := `d8_fencing_agent_condition{node="w1",reason="Armed"} 1`
	if !strings.Contains(string(body), want) {
		t.Errorf("metrics output does not contain %s:\n%s", want, body)
	}
}
