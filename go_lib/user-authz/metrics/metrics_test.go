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
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/deckhouse/deckhouse/go_lib/user-authz/rules"
)

// The series this package publishes are the contract the alerts are written against, and both
// consumers export them under their own prefix. A gauge added to the struct but forgotten in
// collectors() would simply never appear, with the registry reporting success - so the first thing
// worth pinning is that every field of the struct is actually collected.
func TestEveryCollectorIsRegistered(t *testing.T) {
	m := New("test")
	if got, want := len(m.collectors()), reflect.TypeOf(*m).NumField(); got != want {
		t.Fatalf("collectors() returns %d collectors for %d fields; a metric added to the struct and not to the slice is silently never exported", got, want)
	}
	for i, c := range m.collectors() {
		if c == nil || reflect.ValueOf(c).IsNil() {
			t.Errorf("collector %d is nil", i)
		}
	}
}

func gather(t *testing.T, m *Metrics) map[string]*dto.MetricFamily {
	t.Helper()
	reg := prometheus.NewPedanticRegistry()
	if err := m.Register(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	byName := make(map[string]*dto.MetricFamily, len(families))
	for _, f := range families {
		byName[f.GetName()] = f
	}
	return byName
}

func TestNamesAndValues(t *testing.T) {
	m := New("user_authz_webhook")

	m.DirectoryRebuilt(rules.Stats{
		Rules:              7,
		Subjects:           11,
		Quarantined:        map[string]error{"broken": errors.New("bad pattern")},
		MaxResourceVersion: 4242,
	}, 250*time.Millisecond)
	m.SyncedChanged(true)
	m.WatchError(errors.New("connection reset"))

	families := gather(t, m)

	// The names, spelled out. An alert refers to them by string, so a rename here is a silent
	// break of the alerting rules and of the FAQ table that documents them.
	want := []string{
		"user_authz_webhook_rules_observed",
		"user_authz_webhook_rules_max_resource_version",
		"user_authz_webhook_rules_subjects",
		"user_authz_webhook_rules_quarantined",
		"user_authz_webhook_rules_directory_updated_timestamp_seconds",
		"user_authz_webhook_rules_informer_synced",
		"user_authz_webhook_rules_directory_rebuild_duration_seconds",
		"user_authz_webhook_rules_directory_rebuilds_total",
		"user_authz_webhook_rules_watch_errors_total",
	}
	for _, name := range want {
		if _, ok := families[name]; !ok {
			t.Errorf("%s is not exported", name)
		}
	}
	if len(families) != len(want) {
		t.Errorf("exported %d families, expected %d: %v", len(families), len(want), families)
	}

	gaugeValue := func(name string) float64 {
		t.Helper()
		f, ok := families[name]
		if !ok || len(f.GetMetric()) == 0 {
			t.Fatalf("%s is missing", name)
		}
		return f.GetMetric()[0].GetGauge().GetValue()
	}

	if got := gaugeValue("user_authz_webhook_rules_observed"); got != 7 {
		t.Errorf("rules_observed = %v, want 7", got)
	}
	if got := gaugeValue("user_authz_webhook_rules_subjects"); got != 11 {
		t.Errorf("rules_subjects = %v, want 11", got)
	}
	// Quarantined is the COUNT of broken rules, not the map. It is what the alert thresholds on.
	if got := gaugeValue("user_authz_webhook_rules_quarantined"); got != 1 {
		t.Errorf("rules_quarantined = %v, want 1", got)
	}
	// The watermark an operator compares between masters to see that one of them is behind.
	if got := gaugeValue("user_authz_webhook_rules_max_resource_version"); got != 4242 {
		t.Errorf("rules_max_resource_version = %v, want 4242", got)
	}
	if got := gaugeValue("user_authz_webhook_rules_informer_synced"); got != 1 {
		t.Errorf("informer_synced = %v, want 1", got)
	}
	if got := gaugeValue("user_authz_webhook_rules_directory_updated_timestamp_seconds"); got == 0 {
		t.Error("directory_updated_timestamp_seconds was not set")
	}

	first := func(name string) *dto.Metric {
		t.Helper()
		f, ok := families[name]
		if !ok || len(f.GetMetric()) == 0 {
			t.Fatalf("%s is missing", name)
		}
		return f.GetMetric()[0]
	}

	if got := first("user_authz_webhook_rules_directory_rebuilds_total").GetCounter().GetValue(); got != 1 {
		t.Errorf("rebuilds_total = %v, want 1", got)
	}
	if got := first("user_authz_webhook_rules_watch_errors_total").GetCounter().GetValue(); got != 1 {
		t.Errorf("watch_errors_total = %v, want 1", got)
	}
	hist := first("user_authz_webhook_rules_directory_rebuild_duration_seconds").GetHistogram()
	if hist.GetSampleCount() != 1 || hist.GetSampleSum() != 0.25 {
		t.Errorf("rebuild_duration: count=%d sum=%v, want 1 and 0.25", hist.GetSampleCount(), hist.GetSampleSum())
	}
}

// informer_synced going back to zero is what tells an operator the directory has stopped tracking
// the cluster. A gauge that only ever goes up would make the alert unable to fire twice.
func TestSyncedChangedGoesBothWays(t *testing.T) {
	m := New("test")
	m.SyncedChanged(true)
	m.SyncedChanged(false)

	families := gather(t, m)
	f, ok := families["test_rules_informer_synced"]
	if !ok || len(f.GetMetric()) == 0 {
		t.Fatal("test_rules_informer_synced is missing")
	}
	if got := f.GetMetric()[0].GetGauge().GetValue(); got != 0 {
		t.Errorf("informer_synced = %v after losing sync, want 0", got)
	}
}

// Both consumers publish the same series under their own prefix, which is what lets one alert
// expression cover both and lets the two be compared against each other.
func TestPrefixIsTheOnlyDifference(t *testing.T) {
	webhook := gather(t, New("user_authz_webhook"))
	browser := gather(t, New("user_authz_permission_browser"))

	if len(webhook) != len(browser) {
		t.Fatalf("the two consumers export %d and %d series", len(webhook), len(browser))
	}
	for name := range webhook {
		twin := "user_authz_permission_browser" + name[len("user_authz_webhook"):]
		if _, ok := browser[twin]; !ok {
			t.Errorf("%s has no counterpart %s", name, twin)
		}
	}
}
