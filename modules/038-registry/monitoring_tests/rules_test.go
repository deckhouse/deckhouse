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

// Package monitoring_tests checks that the alerts and the code agree about what exists.
//
// The failure this exists to prevent is silent and permanent: an alert whose expression
// names a metric nobody exports never fires, and nothing anywhere says so. It looks exactly
// like a healthy cluster. The reverse — a metric exported and never alerted on — is fine and
// often deliberate, so only one direction is checked.
package monitoring_tests

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// Where the two sides live, relative to this package.
const (
	rulesDir = "../monitoring/prometheus-rules"

	// The components that export metrics of their own. The module hook that exports the
	// migration metric is in the fourth.
	controllerDir = "../images/registry-controller/src"
	syncerDir     = "../images/registry-syncer/src"
	agentDir      = "../images/registry-agent/src"
	hooksDir      = "../hooks"
)

// metricName finds the names our own code declares, in either of the two forms it uses: a
// Prometheus collector option, or the string a module hook passes to MetricsCollector.
var metricName = regexp.MustCompile(`"(d8_registry[a-z0-9_]*)"`)

// referenced finds the names an expression reads.
var referenced = regexp.MustCompile(`\b(d8_registry[a-z0-9_]*)\b`)

type rule struct {
	Alert string `json:"alert"`
	Expr  string `json:"expr"`
}

type group struct {
	Name  string `json:"name"`
	Rules []rule `json:"rules"`
}

func loadRules(t *testing.T) []group {
	t.Helper()

	entries, err := os.ReadDir(rulesDir)
	require.NoError(t, err)

	var groups []group
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(rulesDir, entry.Name()))
		require.NoErrorf(t, err, "reading %s", entry.Name())

		var parsed []group
		require.NoErrorf(t, yaml.Unmarshal(content, &parsed), "parsing %s", entry.Name())
		groups = append(groups, parsed...)
	}

	require.NotEmpty(t, groups, "no rule groups found, so this test would pass on anything")
	return groups
}

// exportedMetrics reads the metric names out of the sources, as text.
//
// Text rather than reflection or a running process, because the three components are
// separate Go modules that this test cannot import, and because building all of them to ask
// them would make a fast check slow enough to skip.
func exportedMetrics(t *testing.T) map[string]string {
	t.Helper()

	found := map[string]string{}
	for _, dir := range []string{controllerDir, syncerDir, agentDir, hooksDir} {
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			// Test files may name a metric that only a test invents.
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, match := range metricName.FindAllStringSubmatch(string(content), -1) {
				found[match[1]] = path
			}
			return nil
		})
		require.NoErrorf(t, err, "walking %s", dir)
	}

	require.NotEmpty(t, found, "no metrics found in the sources, so this test would pass on anything")
	return found
}

// TestEveryAlertReadsAMetricThatExists is the check itself.
func TestEveryAlertReadsAMetricThatExists(t *testing.T) {
	exported := exportedMetrics(t)

	for _, g := range loadRules(t) {
		for _, r := range g.Rules {
			for _, match := range referenced.FindAllStringSubmatch(r.Expr, -1) {
				name := match[1]
				_, ok := exported[name]
				assert.Truef(t, ok,
					"alert %s reads %q, which nothing exports: the alert would never fire and "+
						"nothing would say so. Exported: %s",
					r.Alert, name, strings.Join(sorted(exported), ", "))
			}
		}
	}
}

// TestEveryAlertIsUsable covers the parts of a rule that are easy to leave half-written and
// impossible to notice afterwards: an alert with no expression fires never, one with no
// `for` fires on a single scrape, and one with no description arrives as a name.
func TestEveryAlertIsUsable(t *testing.T) {
	type raw struct {
		Name  string `json:"name"`
		Rules []struct {
			Alert       string            `json:"alert"`
			Expr        string            `json:"expr"`
			For         string            `json:"for"`
			Labels      map[string]string `json:"labels"`
			Annotations map[string]string `json:"annotations"`
		} `json:"rules"`
	}

	entries, err := os.ReadDir(rulesDir)
	require.NoError(t, err)

	seen := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(rulesDir, entry.Name()))
		require.NoError(t, err)

		var groups []raw
		require.NoError(t, yaml.Unmarshal(content, &groups))

		for _, g := range groups {
			assert.True(t, strings.HasPrefix(g.Name, "d8.registry."),
				"group %q is not namespaced to this module", g.Name)

			for _, r := range g.Rules {
				seen++
				assert.NotEmptyf(t, r.Expr, "alert %s has no expression", r.Alert)
				assert.NotEmptyf(t, r.For, "alert %s has no `for`, so a single scrape raises it", r.Alert)
				assert.Equalf(t, "registry", r.Labels["d8_module"],
					"alert %s is not attributed to this module", r.Alert)
				assert.NotEmptyf(t, r.Labels["severity_level"],
					"alert %s has no severity", r.Alert)
				assert.NotEmptyf(t, r.Annotations["summary"], "alert %s has no summary", r.Alert)
				assert.NotEmptyf(t, r.Annotations["description"],
					"alert %s has no description, so it arrives as a name", r.Alert)
			}
		}
	}

	assert.Positive(t, seen, "no alerts were checked")
}

// TestNothingAlertsOnAModuleManagingNothing is the mistake most likely to be made here and
// the hardest to notice: the module's default is to manage nothing, so an unguarded alert
// about an empty cache or an unconverged node would fire on every cluster that simply left
// it alone.
//
// The exceptions are listed rather than inferred, so that adding one is a decision.
func TestNothingAlertsOnAModuleManagingNothing(t *testing.T) {
	// Alerts that legitimately describe a cluster managing nothing.
	unguarded := map[string]string{
		"D8RegistryStaleCacheData": "it is about precisely the unmanaged case, and guarding it " +
			"would silence the only state it describes",
		"D8RegistryMigrationPending": "the cluster is on the previous implementation, which is " +
			"the opposite of managing nothing",
		"D8RegistryMigrationPreflightBlocked": "the series only exists on a cluster that still " +
			"has the previous implementation's state, so it cannot fire on one managing nothing",
		"D8RegistryNodeForeignRegistryConfig": "the series is published only while this " +
			"implementation manages the registry, which is the opposite of managing nothing",
		"D8RegistryDrainStuck": "the series exists only while the module is leaving the pull path, " +
			"which is a cluster that WAS managing and has not finished stopping — the state this " +
			"alert is the only report of",
		"D8RegistryAirGapTransitionHeld": "the gauge only exists while the controller is running",
		"D8RegistryUpstreamRejected":     "the gauge only exists while the controller is running",
		"D8RegistryControllerDown": "guarded by the registry-controller Deployment existing, as " +
			"kube-state-metrics sees it; d8_registry_managed is exported by the controller itself, " +
			"so it would silence this alert exactly when the controller is gone",
		"D8RegistryStorageLeaderAbsent": "guarded by the registry-storage StatefulSet existing, as " +
			"kube-state-metrics sees it, which is rendered only while the module keeps a cache; " +
			"d8_registry_managed is the controller's and would make this depend on it",
	}

	for _, g := range loadRules(t) {
		for _, r := range g.Rules {
			if why, ok := unguarded[r.Alert]; ok {
				assert.NotEmpty(t, why)
				continue
			}

			assert.Containsf(t, r.Expr, "d8_registry_managed",
				"alert %s would fire on a cluster where the module manages nothing, which is "+
					"the default. Guard it, or add it to the list in this test with a reason.",
				r.Alert)
		}
	}
}

// TestTheAgentAlertsSelectTheAgent is the same silent failure from the other side: the agent's
// alerts read kube-state-metrics series, which exist whatever the selector says, so a selector that
// stopped matching the agent's mirror pod would leave one alert never firing and the other firing
// on every node. The pod is read from dhctl's copy of the Engine manifest, which a template test
// holds equal to the module's and a bashible test holds the bashible node's to.
func TestTheAgentAlertsSelectTheAgent(t *testing.T) {
	content, err := os.ReadFile("../../../dhctl/pkg/immutable/registry-agent.yaml")
	require.NoError(t, err)

	var pod struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Spec struct {
			Containers []struct {
				Name string `json:"name"`
			} `json:"containers"`
		} `json:"spec"`
	}
	require.NoError(t, yaml.Unmarshal(content, &pod))
	require.Len(t, pod.Spec.Containers, 1)

	// The kubelet names a mirror pod after the manifest's pod and the node. The name is quoted
	// twice: once as the expression's own regular expression would spell it, once to find that
	// spelling in the expression's text.
	selector := regexp.MustCompile(`namespace="` + regexp.QuoteMeta(pod.Metadata.Namespace) +
		`", pod=~"` + regexp.QuoteMeta(regexp.QuoteMeta(pod.Metadata.Name)+"-") + `\.\+"`)
	container := `container="` + pod.Spec.Containers[0].Name + `"`

	expressions := map[string]string{}
	for _, g := range loadRules(t) {
		for _, r := range g.Rules {
			expressions[r.Alert] = r.Expr
		}
	}

	// Every pod series is checked, not just the expression as a whole: one series still naming the
	// agent would otherwise hide another that had lost its selector.
	podSeries := regexp.MustCompile(`kube_pod_\w+\{[^}]*\}`)
	for _, alert := range []string{"D8RegistryAgentNotReady", "D8RegistryAgentRestartingTooOften"} {
		expr, ok := expressions[alert]
		require.Truef(t, ok, "there is no alert %s", alert)
		series := podSeries.FindAllString(expr, -1)
		require.NotEmptyf(t, series, "alert %s reads no pod series", alert)
		for _, s := range series {
			assert.Regexpf(t, selector, s, "alert %s reads %s, which is not the agent's mirror pod", alert, s)
		}
	}
	assert.Containsf(t, expressions["D8RegistryAgentRestartingTooOften"], container,
		"D8RegistryAgentRestartingTooOften does not count the agent container's restarts")
}

func sorted(m map[string]string) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
