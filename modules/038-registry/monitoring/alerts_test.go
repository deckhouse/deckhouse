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

// Tests for the alerting rules of this module.
//
// What these DO check: that every alert is complete enough to be actionable, that its `for:` is the
// duration it is meant to be, and — the one that matters most — that every metric an expression
// names is a metric the code actually registers.
//
// What they do NOT check: the PromQL itself. Evaluating an expression needs the Prometheus query
// engine, which this repository does not depend on, and pulling it in for a test is a decision
// bigger than a test. So the semantics of these rules are covered where they can be: on a cluster,
// by scenario-alerts.sh, which breaks four things and waits for the ClusterAlert objects to appear.
//
// The reason any of this exists: all nine alerts of this module were inert for the whole of its
// life. Not because an expression was wrong — because the kube-rbac-proxy beside each component
// could not create a TokenReview, so no `d8_registry_*` series ever reached Prometheus at all, and
// every expression opens with `max(d8_registry_managed) > 0`. Nothing looked broken. The lesson
// generalises: an alert that never fires is indistinguishable from a problem that never happened,
// so the things it depends on need checking by something other than the absence of complaints.
package monitoring_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// The file is a bare sequence of groups, which is the shape Deckhouse's prometheus-rules take —
// the `groups:` key belongs to the PrometheusRule object the templates build around them.
type ruleGroup struct {
	Name  string `yaml:"name"`
	Rules []struct {
		Alert       string            `yaml:"alert"`
		Expr        string            `yaml:"expr"`
		For         string            `yaml:"for"`
		Labels      map[string]string `yaml:"labels"`
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"rules"`
}

type ruleFile struct {
	Groups []ruleGroup
}

// The durations these alerts are meant to wait, written down so that changing one is a deliberate
// act rather than a diff nobody reads. They are also what decides which alerts a cluster test can
// catch at all: anything above twenty minutes is not worth holding a cluster open for.
var expectedFor = map[string]string{
	"D8RegistryConfigInvalid":        "5m",
	"D8RegistryNodeNotConverged":     "20m",
	"D8RegistryNodeRunningFromDisk":  "1h",
	"D8RegistryStorageIncomplete":    "2h",
	"D8RegistryAirGapTransitionHeld": "6h",
	"D8RegistryUpstreamProbeFailing": "15m",
	"D8RegistryUpstreamRejected":     "15m",
	"D8RegistryStaleCacheData":       "24h",
	"D8RegistryStorageNotReclaimed":  "1h",
}

func loadRules(t *testing.T) ruleFile {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("prometheus-rules", "registry.yaml"))
	require.NoError(t, err, "the module's alerting rules must be readable")

	var groups []ruleGroup
	require.NoError(t, yaml.Unmarshal(raw, &groups))
	return ruleFile{Groups: groups}
}

// TestEveryAlertIsActionable: an alert an operator cannot act on is noise, and noise is how real
// alerts come to be ignored.
func TestEveryAlertIsActionable(t *testing.T) {
	rules := loadRules(t)

	seen := map[string]bool{}
	for _, group := range rules.Groups {
		for _, rule := range group.Rules {
			if rule.Alert == "" {
				continue
			}
			seen[rule.Alert] = true

			t.Run(rule.Alert, func(t *testing.T) {
				assert.NotEmpty(t, rule.Expr, "an alert without an expression can never fire")
				assert.NotEmpty(t, rule.For,
					"without `for:` a single scrape of a transient state pages somebody")

				// Severity is what decides whether this blocks a Deckhouse update: the release
				// controller refuses to deploy while an alert above a threshold is firing.
				assert.Contains(t, rule.Labels, "severity_level",
					"severity decides whether this blocks an update, so it cannot be left out")

				assert.Contains(t, rule.Annotations, "summary")
				assert.Contains(t, rule.Annotations, "description",
					"the description is where the operator is told what to do about it")
			})
		}
	}

	// Every alert this module is documented to have, and no silent disappearances.
	var missing []string
	for name := range expectedFor {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	assert.Empty(t, missing, "these alerts are expected to exist and were not found")
}

// TestTheWaitsAreWhatTheyAreMeantToBe pins the `for:` durations.
//
// They are load-bearing in two directions: too short and a rolling update pages somebody, too long
// and the condition is over before anybody hears about it. They also decide what a cluster test can
// verify, so a change here changes what is covered and what is not.
func TestTheWaitsAreWhatTheyAreMeantToBe(t *testing.T) {
	rules := loadRules(t)

	for _, group := range rules.Groups {
		for _, rule := range group.Rules {
			want, known := expectedFor[rule.Alert]
			if !known {
				continue
			}
			assert.Equal(t, want, rule.For, "the wait before %s fires changed", rule.Alert)
		}
	}
}

// TestExpressionsNameMetricsThatExist is the one that catches an alert dying quietly.
//
// A metric renamed or removed in the code leaves the rule referring to a series that will never
// appear again. Nothing fails, nothing logs, and the alert is simply gone — which is exactly what
// happened to all nine of these for a different reason, and took a deliberate hunt to notice.
func TestExpressionsNameMetricsThatExist(t *testing.T) {
	rules := loadRules(t)

	registered := registeredMetrics(t)
	require.NotEmpty(t, registered, "no metric names were found in the controller's code")

	referenced := regexp.MustCompile(`d8_registry_[a-z0-9_]+`)

	for _, group := range rules.Groups {
		for _, rule := range group.Rules {
			if rule.Alert == "" {
				continue
			}
			for _, name := range referenced.FindAllString(rule.Expr, -1) {
				assert.Contains(t, registered, name,
					"%s refers to %s, which the code does not register — the alert can never fire",
					rule.Alert, name)
			}
		}
	}
}

// TestTheReclaimAlertKnowsHowLongTheReplicaHasBeenAble guards the one alert that has been observed
// firing on a healthy cluster.
//
// `d8_registry_storage_last_collection_timestamp_seconds` is a registered gauge, so it is exposed as
// 0 from the first scrape and stays 0 until a collection finishes. `time() - 0` is the whole Unix
// epoch, which passes any staleness threshold — so a store that had never been collected yet looked
// exactly like one that stopped a week ago, and the alert arrived an hour after boot on a cluster
// whose schedule had not come round once. Measured on a stand, twice.
//
// The fix is a term asking how long the replica has been able to collect. It is a single line and
// deleting it would restore the false alarm silently, which is what this test is for.
func TestTheReclaimAlertKnowsHowLongTheReplicaHasBeenAble(t *testing.T) {
	rules := loadRules(t)

	const guard = "d8_registry_storage_started_timestamp_seconds"

	var found bool
	for _, group := range rules.Groups {
		for _, rule := range group.Rules {
			if rule.Alert != "D8RegistryStorageNotReclaimed" {
				continue
			}
			found = true
			assert.Contains(t, rule.Expr, guard,
				"without %s this alert cannot tell a store that has never been collected from one "+
					"collected in 1970, and it fires on every healthy new cluster", guard)
		}
	}
	require.True(t, found, "D8RegistryStorageNotReclaimed was not found at all")
}

// registeredMetrics reads the metric names out of the controller's registry.
//
// From the source rather than from a list kept here, so that the two cannot drift apart: a list
// would have to be updated by the same person who renamed the metric, which is the failure this
// guards against.
func registeredMetrics(t *testing.T) map[string]bool {
	t.Helper()

	// Both components that are scraped, because the rules draw on both: the controller describes
	// the cluster, a storage replica describes itself. Reading only the controller made this test
	// report the garbage-collection alert as broken on its first run, when what was incomplete was
	// the list of places it looked.
	//
	// The node agent is deliberately absent: it is a static pod with no kube-rbac-proxy, nothing
	// scrapes it, and what it knows reaches Prometheus through the controller instead.
	sources := []string{
		filepath.Join("..", "images", "registry-controller", "src", "internal", "metrics", "metrics.go"),
		filepath.Join("..", "images", "registry-syncer", "src", "internal", "metrics", "metrics.go"),
	}

	names := map[string]bool{}
	pattern := regexp.MustCompile(`Name:\s*"(d8_registry_[a-z0-9_]+)"`)

	for _, path := range sources {
		raw, err := os.ReadFile(path)
		require.NoError(t, err, "the metrics of this module must be readable at %s", path)
		for _, match := range pattern.FindAllStringSubmatch(string(raw), -1) {
			names[match[1]] = true
		}
	}
	return names
}
