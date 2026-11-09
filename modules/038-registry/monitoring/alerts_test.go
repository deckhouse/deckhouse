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
// duration it is meant to be, and — most importantly — that every metric an expression names is one
// the code actually registers.
//
// What they do NOT check: the PromQL itself, which needs the Prometheus query engine this repository
// does not depend on. Those semantics are covered on a cluster, by scenario-alerts.sh.
//
// Why it matters: an alert can be inert for reasons no expression review would catch — almost every
// expression here opens with `max(d8_registry_managed) > 0`, so a component whose kube-rbac-proxy
// cannot create a TokenReview publishes no series and nothing ever fires, with nothing looking
// broken. An alert that never fires is indistinguishable from a problem that never happened, so what
// it depends on needs checking by something other than the absence of complaints. The exceptions
// are the availability rules, which exist for exactly that case and so cannot depend on it.
package monitoring_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
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
	"D8RegistryStoreWritesRefused":   "2m",
	"D8RegistryStoreNearlyFull":      "5m",
	"D8RegistryStoreUnbounded":       "1h",
	"D8RegistryControllerDown":       "10m",
	"D8RegistryStorageLeaderAbsent":  "10m",
	"D8RegistryStorageUpdateBlocked": "1h",
}

// The files these tests read. Named rather than globbed: the other rule files of this module read
// metrics the module's hooks export, which are declared as constants rather than as collector options
// and so are invisible to registeredMetrics. Those are checked by monitoring_tests, which reads the
// hooks as well.
var ruleFiles = []string{"registry.yaml", "availability.yaml", "storage-update.yaml"}

func loadRules(t *testing.T) ruleFile {
	t.Helper()

	var groups []ruleGroup
	for _, name := range ruleFiles {
		raw, err := os.ReadFile(filepath.Join("prometheus-rules", name))
		require.NoError(t, err, "the module's alerting rules must be readable: %s", name)

		var parsed []ruleGroup
		require.NoError(t, yaml.Unmarshal(raw, &parsed), "parsing %s", name)
		groups = append(groups, parsed...)
	}
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

// TestTheReclaimAlertKnowsHowLongTheReplicaHasBeenAble guards the alert most likely to fire on a
// healthy cluster.
//
// `d8_registry_storage_last_collection_timestamp_seconds` is a registered gauge, so it is exposed as
// 0 from the first scrape and stays 0 until a collection finishes. `time() - 0` is the whole Unix
// epoch, which passes any staleness threshold — so a store that has never been collected reads
// exactly like one that stopped a week ago, and the alert fires before the schedule has come round
// once.
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

// TestTheAvailabilityAlertsDoNotNeedWhatTheyWatch pins the terms that keep the two availability
// alerts alive in the very states they exist to report.
//
// Each of them is a single term away from being inert, and none of the failures would show:
//
//   - The leader alert sums a gauge that only ready, scraped replicas export. With every replica
//     down there is nothing to sum, `sum(...) < 1` is empty rather than true, and the alert goes
//     quiet at the worst moment unless `absent(...)` stands beside it.
//   - The controller alert asks whether anything answers on the job the PodMonitor scrapes. The
//     job is taken from the pod's `app` label (`jobLabel: app`), so a typo here asks about a job
//     that never exists and fires on every cluster with the module on — or, with `== 1` dropped,
//     never fires at all, since a target that answers 0 is still present.
//   - Both are guarded by the workload existing, as kube-state-metrics sees it, and not by
//     `d8_registry_managed`: that gauge is the controller's, so a guard built on it would silence
//     the controller alert precisely when the controller is gone.
func TestTheAvailabilityAlertsDoNotNeedWhatTheyWatch(t *testing.T) {
	rules := loadRules(t)

	want := map[string][]string{
		"D8RegistryControllerDown": {
			`kube_deployment_spec_replicas{namespace="d8-system",deployment="registry-controller"}`,
			`absent(up{job="registry-controller",namespace="d8-system"} == 1)`,
		},
		"D8RegistryStorageLeaderAbsent": {
			`kube_statefulset_replicas{namespace="d8-system",statefulset="registry-storage"}`,
			`sum(d8_registry_storage_leader) < 1`,
			`absent(d8_registry_storage_leader)`,
		},
	}

	found := map[string]bool{}
	for _, group := range rules.Groups {
		for _, rule := range group.Rules {
			terms, ok := want[rule.Alert]
			if !ok {
				continue
			}
			found[rule.Alert] = true

			for _, term := range terms {
				assert.Contains(t, rule.Expr, term, "%s lost a term it cannot work without", rule.Alert)
			}
			assert.NotContains(t, rule.Expr, "d8_registry_managed",
				"%s cannot be guarded by a gauge the controller exports: it has to fire when the "+
					"controller does not", rule.Alert)
		}
	}
	for name := range want {
		assert.True(t, found[name], "%s was not found at all", name)
	}
}

// TestAStoreThatIsNotWritableIsNotSaidToBeFilling: such a replica reports an error, and a replica with
// an error is not full — so two hours later D8RegistryStorageIncomplete fired beside
// D8RegistryStoreWritesRefused, saying the replica was still filling while it held the whole set and
// could not take a write. The term that leaves those nodes out is a single line.
func TestAStoreThatIsNotWritableIsNotSaidToBeFilling(t *testing.T) {
	const guard = `unless on (node) (max by (node) (d8_registry_store_writable{reason="NotWritable"}) == 0)`

	for _, group := range loadRules(t).Groups {
		for _, rule := range group.Rules {
			if rule.Alert != "D8RegistryStorageIncomplete" {
				continue
			}
			assert.Contains(t, strings.Join(strings.Fields(rule.Expr), " "), guard,
				"without it a store the registry cannot write to is reported as still filling")
			assert.Contains(t, rule.Annotations["description"], "D8RegistryStoreWritesRefused",
				"the description says where such a replica is reported")
			return
		}
	}
	t.Fatal("D8RegistryStorageIncomplete was not found")
}

// TestTheUpdateAlertExplainsEveryReason: the reason is a label, and the description is where an
// operator learns what each value means and whether to do anything. A reason added to the controller
// and not here arrives as a word nobody has explained — and the one thing these reasons exist to say
// is which of them need an operator and which do not.
func TestTheUpdateAlertExplainsEveryReason(t *testing.T) {
	rules := loadRules(t)

	raw, err := os.ReadFile(filepath.Join("..", "images", "registry-controller", "src", "internal",
		"controller", "storageupdate", "refusal.go"))
	require.NoError(t, err)
	reasons := regexp.MustCompile(`BlockReason = "([A-Za-z]+)"`).FindAllStringSubmatch(string(raw), -1)
	require.NotEmpty(t, reasons, "no reasons were found in the controller's code")

	var description string
	for _, group := range rules.Groups {
		for _, rule := range group.Rules {
			if rule.Alert == "D8RegistryStorageUpdateBlocked" {
				description = rule.Annotations["description"]
			}
		}
	}
	require.NotEmpty(t, description, "D8RegistryStorageUpdateBlocked was not found at all")

	for _, reason := range reasons {
		assert.Contains(t, description, "`"+reason[1]+"`",
			"the controller can report %s, and the alert does not say what it means", reason[1])
	}
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

// TestEveryRefusalIsExplained: D8RegistryStoreWritesRefused fires for whatever reason a replica's store
// refuses writes on, and the reason is all the operator gets in the summary. A reason the CRD allows and
// the description does not explain is an alert nobody knows what to do about — the state NotWritable
// was in when it was added: published, alerting, and described as a limit doing its job.
func TestEveryRefusalIsExplained(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "crds", "registrystorage.yaml"))
	require.NoError(t, err)
	// The enum under store.reason, which is the only one in the file that lists BudgetExhausted.
	item := regexp.MustCompile(`^\s+- (\w+)$`)
	var reasons []string
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "enum:" {
			continue
		}
		var values []string
		for _, next := range lines[i+1:] {
			match := item.FindStringSubmatch(next)
			if match == nil {
				break
			}
			values = append(values, match[1])
		}
		if slices.Contains(values, "BudgetExhausted") {
			reasons = values
		}
	}
	require.Contains(t, reasons, "NotWritable", "the store's reasons were not found in the CRD")

	for _, group := range loadRules(t).Groups {
		for _, rule := range group.Rules {
			if rule.Alert != "D8RegistryStoreWritesRefused" {
				continue
			}
			assert.NotContains(t, rule.Expr, "reason=",
				"the alert is for every reason: one left out of the expression would never fire")
			for _, reason := range reasons {
				assert.Contains(t, rule.Annotations["description"], "`"+reason+"` — ",
					"the description says nothing about %s", reason)
			}
			return
		}
	}
	t.Fatal("D8RegistryStoreWritesRefused was not found")
}
