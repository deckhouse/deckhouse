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

package template

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"
)

func TestKubeletAutoMemoryCommandFailuresDoNotSync(t *testing.T) {
	for _, tc := range []struct{ total, message string }{{"AWK_FAIL", "awk failed"}, {"NO_PYTHON", "Python not found"}} {
		t.Run(tc.total, func(t *testing.T) {
			result := executeKubeletStep(t, map[string]interface{}{"resourceReservation": map[string]interface{}{"mode": "Auto"}}, tc.total, nil)
			result.failure(t, tc.message)
		})
	}
}

func TestKubeletNUMAMemoryDistribution(t *testing.T) {
	const gib = int64(1 << 30)
	// 128Gi total: the existing nearest-byte hard eviction is 1374389535.
	// Set S so the total NUMA reservation is exactly 10Gi.
	const staticReserve = "9363028705"
	cases := []struct {
		name     string
		nodes    []numaFixture
		ids      []int
		reserves []int64
	}{
		{"single", []numaFixture{{id: 0, gib: 128}}, []int{0}, []int64{10 * gib}},
		{"unequal sparse numerically sorted", []numaFixture{{id: 10, gib: 32}, {id: 2, gib: 96}, {id: 0}}, []int{2, 10}, []int64{15 * gib / 2, 5 * gib / 2}},
		{"mixed hugepage sizes", []numaFixture{{id: 2, gib: 96, huge: map[int]int{2048: 512, 1048576: 63}}, {id: 10, gib: 32}}, []int{2, 10}, []int64{5 * gib, 5 * gib}},
		{"all memory hugepages excluded", []numaFixture{{id: 0, gib: 32, huge: map[int]int{1048576: 32}}, {id: 5, gib: 96}}, []int{5}, []int64{10 * gib}},
		{"odd total last receives remainder", []numaFixture{{id: 2, gib: 64}, {id: 5, gib: 64}, {id: 10, gib: 64}}, []int{2, 5, 10}, []int64{10 * gib / 3, 10 * gib / 3, 10*gib - 2*(10*gib/3)}},
	}
	for _, mode := range []string{"BestEffort", "Restricted", "SingleNumaNode"} {
		for _, tc := range cases {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				result := executeKubeletStep(t, reservedKubelet(mode, staticReserve), "131072", func(root string) {
					for _, node := range tc.nodes {
						writeNUMAFixture(t, root, node)
					}
				})
				config := result.success(t)
				require.Equal(t, staticReserve, config["systemReserved"].(map[string]interface{})["memory"])
				reservations := config["reservedMemory"].([]interface{})
				require.Len(t, reservations, len(tc.ids))
				var sum int64
				for i, value := range reservations {
					entry := value.(map[string]interface{})
					require.EqualValues(t, tc.ids[i], entry["numaNode"])
					limits := entry["limits"].(map[string]interface{})
					require.Len(t, limits, 1, "hugepages are capacity, not another reserved resource")
					quantity := resource.MustParse(limits["memory"].(string))
					require.Equal(t, tc.reserves[i], quantity.Value())
					sum += quantity.Value()
				}
				hard := resource.MustParse(config["evictionHard"].(map[string]interface{})["memory.available"].(string))
				system := resource.MustParse(staticReserve)
				require.Equal(t, system.Value()+hard.Value(), sum)
			})
		}
	}
}

func TestKubeletMemoryQuantizationAndAuto(t *testing.T) {
	var autoCPU string
	for _, tc := range []struct{ mib, reserve string }{
		{"3072", "257698038"}, {"4095", "257698038"},
		{"4096", "343597384"}, {"4097", "343597384"}, {"5119", "343597384"},
		{"8192", "606873846"}, {"8193", "606873846"},
		{"16384", "871114998"}, {"16385", "871114998"},
		{"131072", "3276548342"}, {"131073", "3276548342"},
		{"132096", "3230914314"},
	} {
		t.Run(tc.mib, func(t *testing.T) {
			result := executeKubeletStep(t, map[string]interface{}{"resourceReservation": map[string]interface{}{"mode": "Auto"}}, tc.mib, nil)
			config := result.success(t)
			require.Equal(t, tc.reserve, config["systemReserved"].(map[string]interface{})["memory"])
			cpu := config["systemReserved"].(map[string]interface{})["cpu"].(string)
			require.Regexp(t, `^[0-9]+m$`, cpu)
			if autoCPU == "" {
				autoCPU = cpu
			} else {
				require.Equal(t, autoCPU, cpu, "Auto CPU reserve must not depend on memory")
			}
		})
	}
	// Same bucket must produce the identical full config, even on a second run.
	var previous []byte
	for _, mib := range []string{"4096", "4097", "5119", "4097"} {
		result := executeKubeletStep(t, reservedKubelet("SingleNumaNode", "1Gi"), mib, func(root string) {
			writeNUMAFixture(t, root, numaFixture{id: 0, gib: 4})
		})
		result.success(t)
		if previous != nil {
			require.Equal(t, previous, result.config)
		}
		previous = result.config
	}
}

func TestKubeletMemoryFailuresDoNotSync(t *testing.T) {
	cases := []struct {
		name, total, memory, errorPart string
		arrange                        func(*testing.T, string)
	}{
		{name: "empty topology", errorPart: "no NUMA nodes with ordinary memory"},
		{name: "free command fails", total: "FAIL", errorPart: "free failed"},
		{name: "missing meminfo", errorPart: "cannot read MemTotal", arrange: func(t *testing.T, root string) { require.NoError(t, os.Mkdir(filepath.Join(root, "node0"), 0700)) }},
		{name: "no ordinary memory", errorPart: "no NUMA nodes with ordinary memory", arrange: func(t *testing.T, root string) {
			writeNUMAFixture(t, root, numaFixture{id: 0, gib: 4, huge: map[int]int{1048576: 4}})
		}},
		{name: "exceeds zone", memory: "8Gi", errorPart: "exhausts NUMA node", arrange: func(t *testing.T, root string) { writeNUMAFixture(t, root, numaFixture{id: 0, gib: 4}) }},
		// 1063004406 + hard eviction of 1024Mi (10737418) == 1Gi zone exactly.
		{name: "exactly exhausts zone", total: "1024", memory: "1063004406", errorPart: "exhausts NUMA node", arrange: func(t *testing.T, root string) { writeNUMAFixture(t, root, numaFixture{id: 0, gib: 1}) }},
		{name: "invalid Static memory", memory: "bad", errorPart: "cannot calculate kubelet memory reservations"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.total == "" {
				tc.total = "4096"
			}
			if tc.memory == "" {
				tc.memory = "1Gi"
			}
			result := executeKubeletStep(t, reservedKubelet("SingleNumaNode", tc.memory), tc.total, func(root string) {
				if tc.arrange != nil {
					tc.arrange(t, root)
				}
			})
			result.failure(t, tc.errorPart)
		})
	}
}

func TestKubeletReservationJustFitsZone(t *testing.T) {
	result := executeKubeletStep(t, reservedKubelet("SingleNumaNode", "1063004405"), "1024", func(root string) {
		writeNUMAFixture(t, root, numaFixture{id: 0, gib: 1})
	})
	config := result.success(t)
	require.Equal(t, "1073741823", config["reservedMemory"].([]interface{})[0].(map[string]interface{})["limits"].(map[string]interface{})["memory"])
}

func TestKubeletNUMAWeightsIgnoreSmallDrift(t *testing.T) {
	// Kernel update moves MemTotal, partial 2Mi hugepage allocation moves ordinary memory.
	// Neither may change reservedMemory: that change means a drain.
	const base = int64(4*(1<<20) + 512*1024)
	var previous []byte
	for _, driftKiB := range []int64{0, 4096, 300 * 1024} {
		result := executeKubeletStep(t, reservedKubelet("SingleNumaNode", "1Gi"), "8192", func(root string) {
			writeFixtureFile(t, root, "node0/meminfo", fmt.Sprintf("Node 0 MemTotal: %d kB\n", base-driftKiB))
			writeFixtureFile(t, root, "node1/meminfo", fmt.Sprintf("Node 1 MemTotal: %d kB\n", base))
			writeFixtureFile(t, root, "node1/hugepages/hugepages-2048kB/nr_hugepages", fmt.Sprintf("%d\n", driftKiB/2048))
		})
		result.success(t)
		if previous != nil {
			require.Equal(t, string(previous), string(result.config))
		}
		previous = result.config
	}
}

func TestKubeletReservationModesAtExecution(t *testing.T) {
	cpuPinning := func(mode string) bool { return mode == "BestEffort" || mode == "SingleNumaNode" }
	memoryPinning := func(mode string) bool { return mode == "BestEffort" || mode == "SingleNumaNode" }
	for _, mode := range []string{"Disabled", "None", "BestEffort", "SingleNumaNode"} {
		t.Run(mode+"/Static without memory", func(t *testing.T) {
			kubelet := reservedKubelet(mode, "0")
			delete(kubelet["resourceReservation"].(map[string]interface{})["static"].(map[string]interface{}), "memory")
			result := executeKubeletStep(t, kubelet, "4096", func(root string) { writeNUMAFixture(t, root, numaFixture{id: 2, gib: 4}) })
			config := result.success(t)
			require.NotContains(t, config["systemReserved"].(map[string]interface{}), "memory")
			if memoryPinning(mode) {
				require.Equal(t, "42949673", config["reservedMemory"].([]interface{})[0].(map[string]interface{})["limits"].(map[string]interface{})["memory"])
			}
		})
		t.Run(mode+"/Static without CPU", func(t *testing.T) {
			kubelet := reservedKubelet(mode, "1Gi")
			delete(kubelet["resourceReservation"].(map[string]interface{})["static"].(map[string]interface{}), "cpu")
			result := executeKubeletStep(t, kubelet, "4096", func(root string) { writeNUMAFixture(t, root, numaFixture{id: 0, gib: 4}) })
			if cpuPinning(mode) {
				result.failure(t, "CPU pinning requires")
			} else {
				result.success(t)
			}
		})
		t.Run(mode+"/Off", func(t *testing.T) {
			kubelet := reservedKubelet(mode, "1Gi")
			kubelet["resourceReservation"] = map[string]interface{}{"mode": "Off"}
			result := executeKubeletStep(t, kubelet, "4096", nil)
			if cpuPinning(mode) {
				result.failure(t, "CPU pinning requires")
			} else {
				require.NotContains(t, result.success(t), "systemReserved")
			}
		})
	}
	// Without pinning NUMA data is not read at all.
	for _, mode := range []string{"Disabled", "None"} {
		t.Run(mode+"/missing sysfs", func(t *testing.T) {
			result := executeKubeletStep(t, reservedKubelet(mode, "1Gi"), "4096", func(root string) { require.NoError(t, os.Remove(root)) })
			result.success(t)
		})
	}
}

type numaFixture struct {
	id   int
	gib  int64
	huge map[int]int
}

func writeNUMAFixture(t *testing.T, root string, node numaFixture) {
	t.Helper()
	writeFixtureFile(t, root, fmt.Sprintf("node%d/meminfo", node.id), fmt.Sprintf("Node %d MemTotal: %d kB\nNode %d MemFree: 0 kB\n", node.id, node.gib*(1<<20), node.id))
	for size, count := range node.huge {
		writeFixtureFile(t, root, fmt.Sprintf("node%d/hugepages/hugepages-%dkB/nr_hugepages", node.id, size), strconv.Itoa(count)+"\n")
		// Deliberately invalid: only nr_hugepages must participate in capacity.
		writeFixtureFile(t, root, fmt.Sprintf("node%d/hugepages/hugepages-%dkB/free_hugepages", node.id, size), "unused\n")
	}
}

func writeFixtureFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
}

// policy "Disabled" is a group without topologyManager.
func reservedKubelet(policy, memory string) map[string]interface{} {
	kubelet := map[string]interface{}{
		"resourceReservation": map[string]interface{}{"mode": "Static", "static": map[string]interface{}{"cpu": "70m", "memory": memory}},
	}
	if policy != "Disabled" {
		kubelet["topologyManager"] = map[string]interface{}{"policy": policy}
	}
	return kubelet
}

type kubeletExecutionResult struct {
	err    error
	output string
	config []byte
	synced bool
	events string
}

func (r kubeletExecutionResult) success(t *testing.T) map[string]interface{} {
	t.Helper()
	require.NoError(t, r.err, "%s", r.output)
	require.True(t, r.synced)
	var config map[string]interface{}
	require.NoError(t, yaml.UnmarshalStrict(r.config, &config), "%s", r.config)
	return config
}

func (r kubeletExecutionResult) failure(t *testing.T, message string) {
	t.Helper()
	require.Error(t, r.err)
	require.Contains(t, r.output, message)
	require.False(t, r.synced, "failure must happen before bb-sync-file")
	require.Equal(t, "original kubelet config\n", string(r.config), "existing file must remain intact")
}

func executeKubeletStep(t *testing.T, kubelet map[string]interface{}, total string, arrange func(string)) kubeletExecutionResult {
	return executeKubeletUpdate(t, kubelet, total, arrange, kubeletUpdateFixture{oldConfig: "original kubelet config\n"})
}

type kubeletUpdateFixture struct {
	oldConfig       string
	bootstrap       bool
	pods            string
	apiFailure      bool
	approvalFailure bool
	nodeFailure     bool
	firstRun        bool
	newConfigSuffix string
}

func executeKubeletUpdate(t *testing.T, kubelet map[string]interface{}, total string, arrange func(string), update kubeletUpdateFixture) kubeletExecutionResult {
	t.Helper()
	root := t.TempDir()
	sysfs := filepath.Join(root, "sysfs")
	require.NoError(t, os.Mkdir(sysfs, 0700))
	if arrange != nil {
		arrange(sysfs)
	}
	body := renderKubeletStep(t, kubeletTemplateContext("1.34", "Normal", kubelet))
	// Keep file reads and all destinations inside the fixture. bb-sync-file
	// captures the real expanded heredoc; no kubelet or host writes execute.
	body = strings.ReplaceAll(body, "/var/lib/", filepath.ToSlash(root)+"/var-lib/")
	yq, err := exec.LookPath("yq")
	require.NoError(t, err, "execution tests require the real mikefarah/yq v4")
	body = strings.ReplaceAll(body, "/opt/deckhouse/bin/yq", `"$TEST_YQ"`)
	python, err := exec.LookPath("python3")
	require.NoError(t, err)
	jq, err := exec.LookPath("jq")
	require.NoError(t, err, "execution tests require the real jq")
	outputPath := filepath.Join(root, "var-lib/kubelet/config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(outputPath), 0700))
	if !update.bootstrap {
		require.NoError(t, os.WriteFile(outputPath, []byte(update.oldConfig), 0600))
	}
	if update.pods == "" {
		update.pods = `{"kind":"PodList","metadata":{},"items":[]}`
	}
	if update.newConfigSuffix != "" {
		body = strings.Replace(body, "\nEOF\n", "\n"+update.newConfigSuffix+"\nEOF\n", 1)
	}
	const harness = `set -eEo pipefail
bb-event-on() { :; }
bb-log-error() { echo "$*" >&2; }
bb-log-warning() { echo "$*" >&2; }
check_python() {
  if [[ "$TEST_TOTAL_MIB" == NO_PYTHON ]]; then echo 'Python not found' >&2; return 1; fi
  python_binary="$TEST_PYTHON"
}
grep() { return 1; }
mkdir() { :; }
df() { printf 'header\n1000000\n'; }
crictl() { printf '{}\n'; }
jq() {
  if [[ "$*" == '-r .config.containerdRootDir' ]]; then cat > /dev/null; printf 'null\n'; return 0; fi
  "$TEST_JQ" "$@"
}
bb-tmp-file() { touch "$TEST_CONFIG_PATH.new" || return 1; printf '%s\n' "$TEST_CONFIG_PATH.new"; }
bb-d8-node-name() {
  if [[ "$TEST_NODE_FAIL" == true ]]; then return 1; fi
  printf 'worker-1\n'
}
bb-curl-kube() {
  printf 'pods\n' >> "$TEST_CONFIG_PATH.events"
  printf '%s\n' "$@" > "$TEST_CONFIG_PATH.request"
  if [[ "$TEST_API_FAIL" == true ]]; then return 1; fi
  printf '%s\n' "$TEST_PODS"
}
bb-deckhouse-get-disruptive-update-approval() {
  printf 'approval\n' >> "$TEST_CONFIG_PATH.events"
  if [[ "$TEST_APPROVAL_FAIL" == true ]]; then return 1; fi
}
free() {
  if [[ "$TEST_TOTAL_MIB" == FAIL ]]; then echo 'free failed' >&2; return 1; fi
  if [[ "$TEST_TOTAL_MIB" == AWK_FAIL ]]; then printf 'Mem: 4096 0 0\n'; return 0; fi
  printf '              total used free\nMem: %s 0 0\n' "$TEST_TOTAL_MIB"
}
awk() {
  if [[ "$TEST_TOTAL_MIB" == AWK_FAIL ]]; then echo 'awk failed' >&2; return 1; fi
  command awk "$@"
}
bb-sync-file() {
  printf 'sync\n' >> "$TEST_CONFIG_PATH.events"
  printf 'called\n' > "$TEST_CONFIG_PATH.sync"
  if ! cmp -s "$2" "$TEST_CONFIG_PATH"; then printf 'kubelet-need-restart\n' >> "$TEST_CONFIG_PATH.events"; fi
  cat "$2" > "$TEST_CONFIG_PATH"
}
`
	cmd := exec.Command("bash", "-c", harness+body)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "TEST_PYTHON="+python, "TEST_TOTAL_MIB="+total, "D8_NUMA_SYSFS_DIR="+sysfs, "TEST_CONFIG_PATH="+outputPath,
		"TEST_YQ="+yq, "TEST_JQ="+jq, "TEST_PODS="+update.pods, "TEST_API_FAIL="+strconv.FormatBool(update.apiFailure),
		"TEST_APPROVAL_FAIL="+strconv.FormatBool(update.approvalFailure), "TEST_NODE_FAIL="+strconv.FormatBool(update.nodeFailure))
	if update.firstRun {
		cmd.Env = append(cmd.Env, "FIRST_BASHIBLE_RUN=yes")
	}
	output, runErr := cmd.CombinedOutput()
	config, err := os.ReadFile(outputPath)
	if !(update.bootstrap && os.IsNotExist(err)) {
		require.NoError(t, err)
	}
	_, syncErr := os.Stat(outputPath + ".sync")
	events, err := os.ReadFile(outputPath + ".events")
	if !os.IsNotExist(err) {
		require.NoError(t, err)
	}
	if strings.Contains(string(events), "pods") {
		request, err := os.ReadFile(outputPath + ".request")
		require.NoError(t, err)
		require.Equal(t, "/api/v1/pods\n--get\n--data-urlencode\nfieldSelector=spec.nodeName=worker-1\n", string(request))
	}
	return kubeletExecutionResult{err: runErr, output: string(output), config: config, synced: syncErr == nil, events: string(events)}
}
