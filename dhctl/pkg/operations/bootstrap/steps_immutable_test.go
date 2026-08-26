// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bootstrap

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable/immutabletest"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

// TestBuildImmutableMasterPayloadIsBase64CloudInit pins the contract: the
// payload travels in the "cloudConfig" tfvar, which every provider's terraform
// base64decodes, and what it carries is the cloud-init the node reads. The
// envelope is not decoration — the provider's own #cloud-config block is glued
// after it, and outside one those keys land on the last payload document.
func TestBuildImmutableMasterPayloadIsBase64CloudInit(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)

	payload, nodeConfigDocument, err := b.buildImmutableMasterPayload(t.Context(), bctx, "example-master-0")
	require.NoError(t, err)

	document, err := base64.StdEncoding.DecodeString(payload)
	require.NoError(t, err, "the payload must be base64: terraform base64decodes it")
	require.True(t, strings.HasPrefix(string(document), "#cloud-config\n"),
		"the provider appends its own block, and only an envelope keeps it off the documents")

	documents := immutabletest.PayloadDocuments(t, document)
	require.Len(t, documents, 2, "the first master is handed a NodeConfig and a ControlPlaneConfig")

	// What the machine's hardware is checked against is byte-for-byte the document
	// the machine is handed.
	require.Equal(t, string(nodeConfigDocument), documents[0],
		"the NodeConfig checked against the machine must be the one the machine gets")

	// Both documents are parsed on the node, so they have to survive the round
	// trip as YAML rather than as an opaque blob.
	var nodeConfig, controlPlaneConfig map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(documents[0]), &nodeConfig))
	require.NoError(t, yaml.Unmarshal([]byte(documents[1]), &controlPlaneConfig))
	require.Equal(t, "NodeConfig", nodeConfig["kind"])
	require.Equal(t, "ControlPlaneConfig", controlPlaneConfig["kind"])
}

// A document nobody named is a typo in the node name: the machine it was written
// for would boot with the defaults the document exists to replace.
func TestImmutableInputRefusesACustomizationWithoutHost(t *testing.T) {
	hosts := map[string]string{"master-0": "10.0.0.11"}
	customizations := []immutable.Customization{{NodeName: "master-0"}, {NodeName: "master-9"}}

	_, err := matchCustomizationsToHosts(staticConfig(), hosts, customizations)

	require.ErrorContains(t, err, "master-9")
	require.ErrorContains(t, err, "--master-host")
}

// Two documents about one machine: the map they land in keeps the last, so the
// machine boots with the half nobody meant and nothing says a document was
// dropped.
func TestImmutableInputRefusesTwoDocumentsForOneNode(t *testing.T) {
	hosts := map[string]string{"master-0": "10.0.0.11"}
	customizations := []immutable.Customization{{NodeName: "master-0"}, {NodeName: "master-0"}}

	_, err := matchCustomizationsToHosts(staticConfig(), hosts, customizations)

	require.ErrorContains(t, err, "master-0")
	require.ErrorContains(t, err, "twice")
}

func TestImmutableInputMatchesByName(t *testing.T) {
	hosts := map[string]string{"master-0": "10.0.0.11"}
	customizations := []immutable.Customization{{NodeName: "master-0"}}

	matched, err := matchCustomizationsToHosts(staticConfig(), hosts, customizations)

	require.NoError(t, err)
	require.Contains(t, matched, "master-0")
}

// Two names for one machine is the same class the static path already checks
// (pkg/preflight/checks/static_instances_ip_duplication.go). Here it would push a
// second master's payload at a machine that is already the first one.
func TestImmutableInputRefusesTwoNamesForOneMachine(t *testing.T) {
	err := refuseSharedAddresses(map[string]string{"master-0": "10.0.0.11", "master-1": "10.0.0.11"})

	require.ErrorContains(t, err, "master-0")
	require.ErrorContains(t, err, "master-1")
	require.ErrorContains(t, err, "10.0.0.11")
}

// The payload the machine boots with has to carry what the operator said about
// it: this is the only place a bare-metal node learns its own addresses, and a
// silently dropped document is a machine that never comes up.
func TestImmutableMasterPayloadCarriesTheCustomization(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)

	customizations, err := immutable.ParseCustomizations(t.Context(), []string{`
apiVersion: internal.deckhouse.io/v1alpha1
kind: NodeConfig
metadata:
  name: example-master-0
spec:
  kubelet:
    nodeIP: 10.99.0.11
`})
	require.NoError(t, err)
	bctx.immutable.customizations = map[string]immutable.Customization{"example-master-0": customizations[0]}

	payload, _, err := b.buildImmutableMasterPayload(t.Context(), bctx, "example-master-0")
	require.NoError(t, err)

	document, err := base64.StdEncoding.DecodeString(payload)
	require.NoError(t, err)
	require.Contains(t, string(document), "10.99.0.11", "the operator's document must reach the machine")
}

// A machine that answers 401 is installed already and its agent holds the port;
// no amount of waiting changes that, and spending the ten-minute budget on it
// buries the one answer that says what is wrong.
func TestPushImmutablePayloadStopsOnAnInstalledNode(t *testing.T) {
	immutabletest.NoRetryCollapse(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	host, port := splitTestServerAddress(t, server)
	b := &ClusterBootstrapper{Params: &Params{Options: options.New()}}
	bctx := &bootstrapContext{immutable: &immutableBootstrap{maintenancePort: port}}

	// Bounded so that a loop which keeps retrying fails the assertion below
	// instead of sitting out the whole budget.
	ctx, cancel := context.WithTimeout(t.Context(), 2*waitMaintenancePort.interval)
	defer cancel()

	started := time.Now()
	err := b.pushImmutablePayload(ctx, bctx, "master-0", host, []byte("#cloud-config\n"), nil)

	require.ErrorIs(t, err, immutable.ErrMaintenanceTokenRequired)
	require.Less(t, time.Since(started), waitMaintenancePort.interval,
		"a machine that is already installed must end the wait, not start the next attempt")
}

// The stand this check was written for: two disks of one size, and a selector
// that names the size.
const twoDisksOfOneSize = `{"disks":[{"name":"sda","size":32212254720},{"name":"sdb","size":32212254720}],"interfaces":[{"name":"enp3s0","mac":"f2:4e:c6:60:03:72"}]}`

// What a plain bare-metal master looks like: one system disk, and a NIC named
// the way systemd names it — never eth0, which is what the document always says.
const oneDiskMachineWithoutEth0 = `{"disks":[{"name":"sda","size":68719476736}],"interfaces":[{"name":"enp3s0","mac":"f2:4e:c6:60:03:72"}]}`

// ambiguousDiskDocument is the document that installed the OS on the wrong one
// of those two disks, which surfaced a reboot later as an unexplained NotReady.
const ambiguousDiskDocument = "apiVersion: internal.deckhouse.io/v1alpha1\nkind: NodeConfig\nspec:\n  storage:\n    diskSelector:\n      size: \"=30Gi\"\n"

// An image too old to answer is a check nobody can run, not a bootstrap to fail:
// refusing to install against one is worse than installing without the check.
func TestCheckMachineAgainstDocument(t *testing.T) {
	document := []byte(ambiguousDiskDocument)

	machine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, twoDisksOfOneSize)
	}))
	t.Cleanup(machine.Close)

	err := checkMachineAgainstDocument(t.Context(), strings.TrimPrefix(machine.URL, "http://"), document)
	require.ErrorContains(t, err, "matches 2 disks",
		"a selector matching both disks must be refused before the push")

	old := httptest.NewServer(http.HandlerFunc(http.NotFound))
	t.Cleanup(old.Close)
	require.NoError(t, checkMachineAgainstDocument(t.Context(), strings.TrimPrefix(old.URL, "http://"), document),
		"an image without the endpoint must not fail the bootstrap")

	// The other ways an inventory can be unreadable, none of which the operator
	// can do anything about while the machine is still uninstalled.
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "{not json")
	}))
	t.Cleanup(broken.Close)
	require.NoError(t, checkMachineAgainstDocument(t.Context(), strings.TrimPrefix(broken.URL, "http://"), document),
		"an inventory that cannot be parsed must not fail the bootstrap")

	require.NoError(t, checkMachineAgainstDocument(t.Context(), "127.0.0.1:1", document),
		"a machine that answers nothing at all must not fail the bootstrap")
}

// A machine still powering on is the normal case inside the push loop, and that
// attempt's own push failure already reports the dead port. A machine that answered
// unusably is worth one line: the same attempt then pushes and the loop ends.
func TestCheckMachineAgainstDocumentWarnsOnlyWhenTheMachineAnswered(t *testing.T) {
	document := []byte(ambiguousDiskDocument)

	logged := func(address string) string {
		var log bytes.Buffer
		ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&log, nil)))
		require.NoError(t, checkMachineAgainstDocument(ctx, address, document))
		return log.String()
	}

	require.Empty(t, logged("127.0.0.1:1"),
		"a machine that answers nothing is still booting, and the push failure of this very attempt says so")

	old := httptest.NewServer(http.HandlerFunc(http.NotFound))
	t.Cleanup(old.Close)
	require.Equal(t, 1, strings.Count(logged(strings.TrimPrefix(old.URL, "http://")), "level=WARN"),
		"an older image answered, and that is worth one line")

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "{not json")
	}))
	t.Cleanup(broken.Close)
	require.Equal(t, 1, strings.Count(logged(strings.TrimPrefix(broken.URL, "http://")), "level=WARN"),
		"an answer that cannot be parsed is worth one line too")
}

// The refusal has to reach the operator instead of the machine: a document that
// contradicts the hardware costs a minute here and a re-imaged machine one PUT
// later. And no waiting changes the hardware, so the wait ends on it. Every
// entry point that pushes is covered, and the two that render drive the real
// builders: the machine is checked against the NodeConfig the payload carries,
// not the #cloud-config wrapper, which unmarshals empty and fits every machine.
func TestEveryPushPathRefusesADocumentTheMachineCannotSatisfy(t *testing.T) {
	cases := []struct {
		name     string
		unpushed string
		push     func(ctx context.Context, t *testing.T, machine *testMachine) error
	}{
		{
			name:     "pushImmutablePayload",
			unpushed: "the machine must not be handed a document it cannot satisfy",
			push: func(ctx context.Context, _ *testing.T, machine *testMachine) error {
				b := &ClusterBootstrapper{Params: &Params{Options: options.New()}}
				bctx := &bootstrapContext{immutable: &immutableBootstrap{maintenancePort: machine.port}}
				// The two documents differ on purpose: the machine takes the cloud-init,
				// the check reads the NodeConfig inside it.
				return b.pushImmutablePayload(ctx, bctx, "master-0", machine.host, []byte("#cloud-config\n"), []byte(ambiguousDiskDocument))
			},
		},
		{
			name:     "bootstrapImmutableFirstMaster",
			unpushed: "the first master must not be installed onto a disk nobody chose",
			push: func(ctx context.Context, t *testing.T, machine *testMachine) error {
				b, bctx := immutableTestBootstrapper(t)
				bctx.immutable.masterNodeName = "example-master-0"
				bctx.immutable.hosts = map[string]string{"example-master-0": machine.host}
				bctx.immutable.maintenancePort = machine.port

				return b.bootstrapImmutableFirstMaster(ctx, bctx)
			},
		},
		{
			name:     "handImmutableJoinPayload",
			unpushed: "a joining master must not be installed onto a disk nobody chose",
			push: func(ctx context.Context, t *testing.T, machine *testMachine) error {
				b, bctx := immutableTestBootstrapper(t)
				bctx.metaConfig.ClusterType = config.StaticClusterType
				bctx.immutable.hosts = map[string]string{"master-1": machine.host}
				bctx.immutable.maintenancePort = machine.port

				kubeCl := client.NewFakeKubernetesClient()
				immutabletest.CreateJoinInputsWithoutToken(t, kubeCl)
				immutabletest.CreateBootstrapToken(t, kubeCl)

				return b.handImmutableJoinPayload(ctx, bctx, kubeCl, "master-1")
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			immutabletest.NoRetryCollapse(t)

			// Two disks the rendered document's own ">=20Gi" system selector matches:
			// nothing in the operator's input is needed to make this machine ambiguous.
			machine := newTestMachine(t, twoDisksOfOneSize)

			// Bounded so that a loop which keeps retrying fails the assertions below
			// instead of sitting out the whole budget.
			ctx, cancel := context.WithTimeout(t.Context(), 2*waitMaintenancePort.interval)
			defer cancel()

			started := time.Now()
			err := c.push(ctx, t, machine)

			require.ErrorContains(t, err, "matches 2 disks")
			require.False(t, machine.pushed.Load(), c.unpushed)
			require.Equal(t, int64(1), machine.reads.Load(),
				"the check costs one round trip on the attempt that reaches the machine")
			require.Less(t, time.Since(started), waitMaintenancePort.interval,
				"the hardware will not change while the loop retries: the wait must end on the refusal")
		})
	}
}

// The wait for the maintenance port is the push loop itself: a check made before it
// would run once, against a machine still powering on, and never run again. It has
// to run on the attempt that reaches the machine.
func TestPushImmutablePayloadChecksTheMachineThatAnswersLate(t *testing.T) {
	immutabletest.NoRetryCollapse(t)

	budget := waitMaintenancePort
	waitMaintenancePort = waitBudget{attempts: 4, interval: time.Millisecond}
	t.Cleanup(func() { waitMaintenancePort = budget })

	var pushes, reads atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			pushes.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		// Still powering on when the bootstrap started: the first attempt gets
		// nothing it could check the document against.
		if reads.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, twoDisksOfOneSize)
	}))
	t.Cleanup(server.Close)

	host, port := splitTestServerAddress(t, server)
	b := &ClusterBootstrapper{Params: &Params{Options: options.New()}}
	bctx := &bootstrapContext{immutable: &immutableBootstrap{maintenancePort: port}}

	err := b.pushImmutablePayload(t.Context(), bctx, "master-0", host, []byte("#cloud-config\n"), []byte(ambiguousDiskDocument))

	require.ErrorContains(t, err, "matches 2 disks", "the machine that answers late must be checked too")
	require.Equal(t, int64(1), pushes.Load(),
		"an attempt that cannot read the inventory still pushes, and the one that reads it refuses")
}

// Most hardware names its NIC enp3s0, eno1 or ens18, and the rendered document
// always says eth0 on DHCP. That guess must not refuse the machine: the node
// brings DHCP up on whatever NIC it finds, and no operator wrote the name.
func TestBootstrapImmutableFirstMasterInstallsOnAMachineWithoutEth0(t *testing.T) {
	immutabletest.NoRetryCollapse(t)

	b, bctx := immutableTestBootstrapper(t)
	bctx.immutable.masterNodeName = "example-master-0"

	machine := newTestMachine(t, oneDiskMachineWithoutEth0)
	bctx.immutable.hosts = map[string]string{"example-master-0": machine.host}
	bctx.immutable.maintenancePort = machine.port

	ctx, cancel := context.WithTimeout(t.Context(), 2*waitMaintenancePort.interval)
	defer cancel()

	require.NoError(t, b.bootstrapImmutableFirstMaster(ctx, bctx),
		"a machine with no eth0 is ordinary hardware, not a misconfiguration")
	require.True(t, machine.pushed.Load(), "the machine must be handed its configuration")
}

// In a cloud the machines are described by the master NodeGroup's instanceClass
// and --master-host is refused outright, so advising one is impossible advice.
func TestImmutableInputRefusesACloudCustomization(t *testing.T) {
	metaConfig := &config.MetaConfig{ClusterType: config.CloudClusterType}
	customizations := []immutable.Customization{{NodeName: "example-master-0"}}

	_, err := matchCustomizationsToHosts(metaConfig, nil, customizations)

	require.ErrorContains(t, err, "example-master-0")
	require.NotContains(t, err.Error(), "--master-host", "the flag it would name is refused on a cloud")
}

// The address the payload went to is what the rest of the bootstrap talks to.
// The push itself is not idempotent and every phase replays on a rerun, so the
// second attempt must skip it: by then the machine answers as an installed node.
func TestBootstrapImmutableFirstMasterReportsTheAddressAndPushesOnce(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)
	bctx.immutable.masterNodeName = "example-master-0"

	var pushes atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The push is preceded by the inventory read, which this machine answers
		// with nothing usable: an unreadable inventory must not stop the push.
		if r.Method != http.MethodPut {
			return
		}
		// What the agent of an installed node answers, and what PushNodeConfig
		// reports as terminal.
		if pushes.Add(1) > 1 {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	t.Cleanup(server.Close)

	host, port := splitTestServerAddress(t, server)
	bctx.immutable.hosts = map[string]string{"example-master-0": host}
	bctx.immutable.maintenancePort = port

	require.NoError(t, b.bootstrapImmutableFirstMaster(t.Context(), bctx))
	require.Equal(t, host, bctx.immutable.masterIP, "the handoff path reads the first master's address from here")

	bctx.immutable.masterIP = ""
	require.NoError(t, b.bootstrapImmutableFirstMaster(t.Context(), bctx),
		"a rerun must not push at a machine that already has its configuration")
	require.Equal(t, int64(1), pushes.Load())
	require.Equal(t, host, bctx.immutable.masterIP, "a rerun still has to report the address")
}

// The push goes to the address the machine boots with; from then on the rest of
// the bootstrap talks to the static one its document assigns. A rerun skips the
// push and must land on the same address, not go back to waiting at the old one.
func TestBootstrapImmutableFirstMasterFollowsTheStaticAddress(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)
	bctx.immutable.masterNodeName = "example-master-0"
	bctx.immutable.customizations = map[string]immutable.Customization{
		"example-master-0": *parseOneCustomization(t, `
    interfaces:
    - name: eth0
      dhcp: false
      addresses: ["192.168.0.101/24"]
      gateway: 192.168.0.1`),
	}

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	t.Cleanup(server.Close)

	host, port := splitTestServerAddress(t, server)
	bctx.immutable.hosts = map[string]string{"example-master-0": host}
	bctx.immutable.maintenancePort = port

	require.NoError(t, b.bootstrapImmutableFirstMaster(t.Context(), bctx))
	require.Equal(t, "192.168.0.101", bctx.immutable.masterIP,
		"the handoff and the apiserver are reached at the address the document assigns")

	bctx.immutable.masterIP = ""
	require.NoError(t, b.bootstrapImmutableFirstMaster(t.Context(), bctx))
	require.Equal(t, "192.168.0.101", bctx.immutable.masterIP,
		"a rerun must not go back to the address the machine was pushed at")
}

// parseOneCustomization builds a customization the way a run does, from a
// document: the fields it is read through are private to the package.
func parseOneCustomization(t *testing.T, networkBlock string) *immutable.Customization {
	t.Helper()

	parsed, err := immutable.ParseCustomizations(t.Context(), []string{`
apiVersion: internal.deckhouse.io/v1alpha1
kind: NodeConfig
metadata:
  name: example-master-0
spec:
  network:
` + networkBlock})
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	return &parsed[0]
}

// The machines that join a running cluster go one at a time and in a stable
// order — etcd takes members one at a time — and the first master is not one of
// them. A rerun must push at none of them: their agents answer the terminal 401.
// What ends the turn is the control plane of that machine, not its Node: master-2
// registers below and never gets a ControlPlaneNode, and the run must not end
// there believing three masters are up.
func TestBootstrapImmutableAdditionalMastersPushOnceInOrder(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)
	bctx.metaConfig.ClusterType = config.StaticClusterType
	bctx.immutable.masterNodeName = "master-0"

	kubeCl := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
		controlPlaneNodeGVR: "ControlPlaneNodeList",
	})
	immutabletest.CreateJoinInputsWithoutToken(t, kubeCl)
	immutabletest.CreateBootstrapToken(t, kubeCl)
	// Only this one is a control-plane member up front. master-2 registers when
	// the handler below serves master-1, so its wait is the thing that fails if
	// the machines are handed their payloads in the wrong order.
	_, err := kubeCl.CoreV1().Nodes().Create(t.Context(),
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "master-1"}}, metav1.CreateOptions{})
	require.NoError(t, err)
	createReadyControlPlaneNode(t, kubeCl, "master-1")

	// A bare-metal machine learns its own address from nowhere else, so the
	// operator's document has to reach a joining master too.
	customizations, err := immutable.ParseCustomizations(t.Context(), []string{`
apiVersion: internal.deckhouse.io/v1alpha1
kind: NodeConfig
metadata:
  name: master-1
spec:
  kubelet:
    nodeIP: 10.99.0.12
`})
	require.NoError(t, err)
	bctx.immutable.customizations = map[string]immutable.Customization{"master-1": customizations[0]}

	var mu sync.Mutex
	var pushes, documents []string
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// The inventory read that precedes every push; this machine answers it
		// with nothing usable, which must not stop the push.
		if r.Method != http.MethodPut {
			return
		}

		// t.Errorf, not require: a failure here would call FailNow off the test
		// goroutine, which the testing package leaves undefined.
		document, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read the pushed document: %v", err)
			return
		}

		mu.Lock()
		defer mu.Unlock()
		documents = append(documents, string(document))
		// One server stands in for all three machines, so the payload itself says
		// which node it was rendered for.
		for _, nodeName := range []string{"master-0", "master-1", "master-2"} {
			if strings.Contains(string(document), nodeName) {
				pushes = append(pushes, nodeName)
			}
		}

		// The machine that was just handed its payload registers as a node when
		// its kubelet starts — long before control-plane-manager has added its
		// etcd member, which is what the wait before the next machine is for.
		if strings.Contains(string(document), "master-1") {
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "master-2"}}
			if _, err := kubeCl.CoreV1().Nodes().Create(r.Context(), node, metav1.CreateOptions{}); err != nil {
				t.Errorf("register node master-2: %v", err)
			}
		}
	}))
	t.Cleanup(server.Close)

	host, port := splitTestServerAddress(t, server)
	bctx.immutable.hosts = map[string]string{"master-0": host, "master-1": host, "master-2": host}
	bctx.immutable.maintenancePort = port

	// master-1 is a control-plane member, so master-2 is handed its payload; the
	// run then waits on master-2, whose control plane never comes up.
	require.ErrorContains(t, b.bootstrapImmutableAdditionalMasters(t.Context(), bctx, kubeCl), "master-2",
		"a registered Node is not a control-plane member: the wait must not end on it")
	require.Equal(t, []string{"master-1", "master-2"}, pushes,
		"the first master has its configuration already, and etcd takes the rest one at a time")
	require.Contains(t, documents[0], "10.99.0.12", "the operator's document must reach a joining master")

	require.Error(t, b.bootstrapImmutableAdditionalMasters(t.Context(), bctx, kubeCl))
	require.Equal(t, []string{"master-1", "master-2"}, pushes,
		"a rerun must not push at a master that already joined")

	// And once master-2 is a member too, the same rerun goes through.
	createReadyControlPlaneNode(t, kubeCl, "master-2")
	require.NoError(t, b.bootstrapImmutableAdditionalMasters(t.Context(), bctx, kubeCl))
	require.Equal(t, []string{"master-1", "master-2"}, pushes)
}

// controlPlaneNodeGVR is what control-plane-manager reports a master's own etcd
// member and static pods through, and what the join wait gates on
// (pkg/operations/converge/infrastructure/hook/controlplane/cp-manager.go).
var controlPlaneNodeGVR = schema.GroupVersionResource{
	Group:    "control-plane.deckhouse.io",
	Version:  "v1alpha1",
	Resource: "controlplanenodes",
}

// createReadyControlPlaneNode writes the object of a master whose control plane
// is up: every condition the readiness checker requires, all True.
func createReadyControlPlaneNode(t *testing.T, kubeCl *client.KubernetesClient, nodeName string) {
	t.Helper()

	conditionTypes := []string{"EtcdReady", "APIServerReady", "ControllerManagerReady", "SchedulerReady", "CertificatesHealthy"}
	conditions := make([]any, 0, len(conditionTypes))
	for _, conditionType := range conditionTypes {
		conditions = append(conditions, map[string]any{
			"type":               conditionType,
			"status":             "True",
			"reason":             "Ready",
			"lastTransitionTime": time.Now().UTC().Format(time.RFC3339),
		})
	}

	controlPlaneNode := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "control-plane.deckhouse.io/v1alpha1",
		"kind":       "ControlPlaneNode",
		"metadata":   map[string]any{"name": nodeName, "namespace": "kube-system"},
		"status":     map[string]any{"conditions": conditions},
	}}

	_, err := kubeCl.Dynamic().Resource(controlPlaneNodeGVR).Namespace("kube-system").
		Create(t.Context(), controlPlaneNode, metav1.CreateOptions{})
	require.NoError(t, err)
}

// One record per node: under a single key the last machine's record answers for
// every other, and a rerun then pushes the first master's payload again — into
// the terminal refusal of an installed node the record exists to prevent.
func TestPushedPayloadIsRecordedPerNode(t *testing.T) {
	stateCache, err := cache.NewStateCache(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, savePushedPayload(t.Context(), stateCache, "master-0", "10.0.0.11"))
	require.NoError(t, savePushedPayload(t.Context(), stateCache, "master-1", "10.0.0.12"))

	pushed, err := payloadAlreadyPushed(t.Context(), stateCache, "master-0", "10.0.0.11")
	require.NoError(t, err)
	require.True(t, pushed, "the record of the first machine must survive the next machine's")

	pushed, err = payloadAlreadyPushed(t.Context(), stateCache, "master-0", "10.0.0.99")
	require.NoError(t, err)
	require.False(t, pushed, "a corrected --master-host names a machine that was handed nothing")
}

func TestRemainingMasterNamesSkipTheFirst(t *testing.T) {
	bctx := &bootstrapContext{immutable: &immutableBootstrap{
		masterNodeName: "master-0",
		hosts: map[string]string{
			"master-0": "10.0.0.11",
			"master-2": "10.0.0.13",
			"master-1": "10.0.0.12",
		},
	}}

	require.Equal(t, []string{"master-1", "master-2"}, remainingMasterNames(bctx))
}

// The cloud API check tunnels through the master host, and an immutable master answers no sshd.
// It is disabled by name because the cloud suites carrying it are shared with every other cloud
// bootstrap; the static path has a suite of its own instead (TestImmutableStaticPreflightsNeedNoSSHHost).
func TestImmutablePreflightsDropTheCheckThatWouldTunnelThroughTheMaster(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)
	runner := preflight.New()

	b.applyImmutablePreflights(runner, bctx)

	require.True(t, runner.IsDisabled(checks.CloudAPICheckName.String()))
}

// An immutable machine runs no sshd, so the question a missing --ssh-host
// triggers — "bootstrap the cluster on the current host?" — is a lie, and the
// --ssh-host it asks for sends later phases at a machine that answers no SSH.
func TestStaticBootstrapNeedsSSHHost(t *testing.T) {
	static := &config.MetaConfig{ClusterType: config.StaticClusterType}
	isImmutable, err := immutable.IsImmutableMaster(t.Context(), static)
	require.NoError(t, err)
	require.True(t, staticBootstrapNeedsSSHHost(static, isImmutable))

	immutableStatic := &config.MetaConfig{ClusterType: config.StaticClusterType, ResourcesYAML: `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
  systemType: Immutable
`}
	isImmutable, err = immutable.IsImmutableMaster(t.Context(), immutableStatic)
	require.NoError(t, err)
	require.False(t, staticBootstrapNeedsSSHHost(immutableStatic, isImmutable))
}

// An --ssh-host given alongside --master-host makes the static preflight suite
// build a real SSH connection to a machine that answers no sshd — and it runs
// after the first master already took its configuration, which is irreversible.
// The refusal has to come from the detection, before anything is pushed.
func TestDetectImmutableMasterRefusesAnSSHHost(t *testing.T) {
	newBootstrapper := func(hosts []sshconfig.Host) (*ClusterBootstrapper, *bootstrapContext) {
		b, bctx := immutableTestBootstrapper(t)
		b.Options.Bootstrap.MasterHostsRaw = []string{"master-0=10.0.0.11"}
		b.SSHProviderInitializer = providerinitializer.NewSSHProviderInitializer(nil,
			&sshconfig.ConnectionConfig{Config: &sshconfig.Config{}, Hosts: hosts})
		bctx.metaConfig.ClusterType = config.StaticClusterType
		bctx.metaConfig.ResourcesYAML = `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
  systemType: Immutable
`
		// Detection is what fills this in; the helper hands out a ready one.
		bctx.immutable = nil
		return b, bctx
	}

	b, bctx := newBootstrapper([]sshconfig.Host{{Host: "10.0.0.11"}})
	err := b.detectImmutableMaster(t.Context(), bctx)
	require.ErrorContains(t, err, "--master-host")
	require.ErrorContains(t, err, "sshd")
	require.Nil(t, bctx.immutable, "the bootstrap must not reach the phase that pushes a payload")

	b, bctx = newBootstrapper(nil)
	require.NoError(t, b.detectImmutableMaster(t.Context(), bctx))
	require.NotNil(t, bctx.immutable)
}

// systemType is the one line an operator who never read the documentation leaves
// out, and --master-host is read nowhere else: without this the machines they
// named are ignored without a word, and the bootstrap opens SSH to a machine
// that runs no sshd — ten minutes of retries before it says anything.
func TestDetectImmutableMasterRefusesHostsWithoutSystemType(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)
	b.Options.Bootstrap.MasterHostsRaw = []string{"master-0=10.0.0.11"}
	bctx.metaConfig.ClusterType = config.StaticClusterType
	// A static cluster has no cloud filler, so the group is read from the
	// documents — which is where the missing line would have been.
	bctx.metaConfig.CloudProviderVars = nil
	bctx.metaConfig.ResourcesYAML = `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
`
	bctx.immutable = nil

	err := b.detectImmutableMaster(t.Context(), bctx)
	require.ErrorContains(t, err, "systemType: Immutable")
	require.ErrorContains(t, err, "--master-host")
	require.Nil(t, bctx.immutable)
}

// The preflight is where an operator learns they named a machine that is not
// there, or a machine whose disks the document cannot pick between. Before it
// existed both cost the whole push budget — ten minutes — and only then said so.
func TestMachinesArePreflightedAgainstTheirDocuments(t *testing.T) {
	immutabletest.NoRetryCollapse(t)

	t.Run("a machine nobody answers for", func(t *testing.T) {
		b, bctx := immutableTestBootstrapper(t)
		bctx.metaConfig.ClusterType = config.StaticClusterType
		// Port 1 on the loopback: nothing listens there, and the dial fails at once
		// rather than hanging, so the test spends no part of the budget.
		bctx.immutable.hosts = map[string]string{"master-0": "127.0.0.1"}
		bctx.immutable.maintenancePort = 1

		started := time.Now()
		err := b.checkMachinesAreWaiting(t.Context(), bctx)
		require.ErrorContains(t, err, "master-0")
		require.ErrorContains(t, err, "not waiting for a configuration")
		// An address nobody answers for is a typo, and a typo must not be waited
		// out: on a live run the untimed version sat for minutes, because one try
		// ran to the HTTP client's own 30s.
		require.Less(t, time.Since(started), 15*time.Second,
			"the preflight must give up in seconds, not run the budget of the push")
	})

	t.Run("a machine the document cannot describe", func(t *testing.T) {
		machine := newTestMachine(t, twoDisksOfOneSize)

		b, bctx := immutableTestBootstrapper(t)
		bctx.metaConfig.ClusterType = config.StaticClusterType
		bctx.immutable.hosts = map[string]string{"master-0": machine.host}
		bctx.immutable.maintenancePort = machine.port

		err := b.checkMachinesAreWaiting(t.Context(), bctx)
		require.ErrorContains(t, err, "does not match the machine")
		require.ErrorContains(t, err, "matches 2 disks")
		require.False(t, machine.pushed.Load(), "a preflight must not hand the machine anything")
	})

	t.Run("a cloud names no machines", func(t *testing.T) {
		b, bctx := immutableTestBootstrapper(t)
		bctx.immutable.hosts = nil

		require.NoError(t, b.checkMachinesAreWaiting(t.Context(), bctx))
	})
}

// splitTestServerAddress returns the host and port a test server landed on.
func splitTestServerAddress(t *testing.T, server *httptest.Server) (string, int) {
	t.Helper()

	address, ok := server.Listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	return address.IP.String(), address.Port
}

// TestAdminKubeconfigFromCache covers the rerun: it re-enters this step with the
// node's channel closing or shut, and must read the credentials the first attempt
// saved instead of waiting half an hour on a listener that is gone.
func TestAdminKubeconfigFromCache(t *testing.T) {
	const collected = "apiVersion: v1\nkind: Config\n"

	t.Run("nothing has been collected yet", func(t *testing.T) {
		stateCache, err := cache.NewStateCache(t.TempDir())
		require.NoError(t, err)

		content, path, err := adminKubeconfigFromCache(t.Context(), stateCache)
		require.NoError(t, err)
		require.Nil(t, content, "with no record there is nothing to reuse; the handoff channel is the only source")
		require.Empty(t, path)
	})

	t.Run("an earlier attempt collected them", func(t *testing.T) {
		stateCache, err := cache.NewStateCache(t.TempDir())
		require.NoError(t, err)

		saved := filepath.Join(t.TempDir(), "example-admin.kubeconfig")
		require.NoError(t, os.WriteFile(saved, []byte(collected), 0o600))
		require.NoError(t, immutable.SaveCollectedKubeconfig(t.Context(), stateCache, saved))

		content, path, err := adminKubeconfigFromCache(t.Context(), stateCache)
		require.NoError(t, err)
		require.Equal(t, collected, string(content))
		require.Equal(t, saved, path)
	})

	// The record does not prove the channel is closed, and the operator may have
	// moved the file; refusing here would make the bootstrap unfinishable, with
	// no flag to clear the record.
	t.Run("the recorded file has been moved away", func(t *testing.T) {
		stateCache, err := cache.NewStateCache(t.TempDir())
		require.NoError(t, err)

		missing := filepath.Join(t.TempDir(), "example-admin.kubeconfig")
		require.NoError(t, immutable.SaveCollectedKubeconfig(t.Context(), stateCache, missing))

		content, path, err := adminKubeconfigFromCache(t.Context(), stateCache)
		require.NoError(t, err, "an unreadable record must fall through to the node, not end the bootstrap")
		require.Nil(t, content)
		require.Empty(t, path)
	})
}

// Once the handover is confirmed the node closes its channel for good and the
// installer's client key is dropped, so a kubeconfig that has gone missing since
// cannot be collected again. Saying so beats spending the half-hour collection
// budget on a refused port and then failing on the missing key.
func TestAdminKubeconfigFromCacheStopsWhenTheHandoverIsOver(t *testing.T) {
	stateCache, err := cache.NewStateCache(t.TempDir())
	require.NoError(t, err)

	_, err = immutable.HandoffMaterialFor(t.Context(), stateCache, "example-master-0")
	require.NoError(t, err)
	require.NoError(t, immutable.ForgetHandoffClientKey(t.Context(), stateCache))

	missing := filepath.Join(t.TempDir(), "example-admin.kubeconfig")
	require.NoError(t, immutable.SaveCollectedKubeconfig(t.Context(), stateCache, missing))

	_, _, err = adminKubeconfigFromCache(t.Context(), stateCache)
	require.Error(t, err, "there is no second collection to fall through to")
	require.Contains(t, err.Error(), "cannot be collected a second time")
}

// A rerun re-enters this step with the credentials already in hand, which skips
// the branch that writes them — so a --kubeconfig-out named for the first time
// on that rerun would be silently ignored.
func TestReuseCollectedKubeconfigHonoursKubeconfigOut(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)
	b.TmpDir = t.TempDir()

	content := []byte("apiVersion: v1\nkind: Config\n")
	collected := filepath.Join(t.TempDir(), "example-admin.kubeconfig")
	// 0644 rather than 0600: saveAdminKubeconfig writes at 0600, so the mode says
	// whether the second call below rewrote the file.
	require.NoError(t, os.WriteFile(collected, content, 0o644))
	require.NoError(t, immutable.SaveCollectedKubeconfig(t.Context(), bctx.stateCache, collected))

	out := filepath.Join(t.TempDir(), "prod.kubeconfig")
	b.Options.Bootstrap.KubeconfigOut = out

	reused, err := b.reuseCollectedKubeconfig(t.Context(), bctx)
	require.NoError(t, err)
	require.Equal(t, content, reused)

	written, err := os.ReadFile(out)
	require.NoError(t, err)
	require.Equal(t, content, written)
	require.Equal(t, out, bctx.immutable.kubeconfigPath)

	// The record follows the file, or the next rerun reads the path this one left.
	recorded, err := immutable.LoadCollectedKubeconfig(t.Context(), bctx.stateCache)
	require.NoError(t, err)
	require.Equal(t, out, recorded)

	// Nothing to do when the file is already where the flag names it: the write
	// clears the path first, and that file is the only copy of the credentials.
	b.Options.Bootstrap.KubeconfigOut = collected
	require.NoError(t, immutable.SaveCollectedKubeconfig(t.Context(), bctx.stateCache, collected))

	reused, err = b.reuseCollectedKubeconfig(t.Context(), bctx)
	require.NoError(t, err)
	require.Equal(t, content, reused)

	info, err := os.Stat(collected)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), info.Mode().Perm(),
		"the file that is already in place must not be rewritten")
}

// The record has to be written before ConfirmCollected shuts the node's channel
// for good; saveAdminKubeconfig is the last point where that ordering holds, so a
// rerun that died anywhere after it finds the file rather than a dead channel.
func TestSaveAdminKubeconfigRecordsThePathBeforeTheChannelCloses(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)
	b.TmpDir = t.TempDir()

	require.NoError(t, b.saveAdminKubeconfig(t.Context(), []byte("apiVersion: v1\nkind: Config\n"), bctx))

	recorded, err := immutable.LoadCollectedKubeconfig(t.Context(), bctx.stateCache)
	require.NoError(t, err)
	require.Equal(t, bctx.immutable.kubeconfigPath, recorded,
		"the rerun path must be usable from the moment the file exists, not from the confirmation")
}

// TestSaveAdminKubeconfigNamesTheFileAfterTheCluster: TmpDir is one directory per
// machine and the write clears the path first, so a shared name would have a second
// cluster's bootstrap delete the first cluster's only credentials.
func TestSaveAdminKubeconfigNamesTheFileAfterTheCluster(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)
	b.TmpDir = t.TempDir()

	require.NoError(t, b.saveAdminKubeconfig(t.Context(), []byte("apiVersion: v1\nkind: Config\n"), bctx))

	require.Equal(t, filepath.Join(b.TmpDir, "example-admin.kubeconfig"), bctx.immutable.kubeconfigPath)
	require.FileExists(t, bctx.immutable.kubeconfigPath)

	// The tmp cleaner spares this file by suffix, so the per-cluster name has to
	// keep the suffix or the credentials are swept away with the rest of TmpDir.
	require.True(t, strings.HasSuffix(bctx.immutable.kubeconfigPath, cache.AdminKubeconfigName))
}

// The file holds cluster-admin credentials: writing into whatever is already at the
// path would inherit a foreign mode and follow a planted symlink. Both are asserted
// because os.WriteFile passes the name check above and fails both of these.
func TestSaveAdminKubeconfigWritesAFreshPrivateFile(t *testing.T) {
	t.Run("a wider mode left by an earlier run is not inherited", func(t *testing.T) {
		b, bctx := immutableTestBootstrapper(t)
		b.TmpDir = t.TempDir()

		path := filepath.Join(b.TmpDir, "example-admin.kubeconfig")
		require.NoError(t, os.WriteFile(path, []byte("stale"), 0o644))

		require.NoError(t, b.saveAdminKubeconfig(t.Context(), []byte("apiVersion: v1\nkind: Config\n"), bctx))

		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
			"cluster-admin credentials must not be left at the mode of whatever was there before")
	})

	t.Run("a symlink at the path is replaced, not followed", func(t *testing.T) {
		b, bctx := immutableTestBootstrapper(t)
		b.TmpDir = t.TempDir()

		target := filepath.Join(t.TempDir(), "somebody-elses-file")
		require.NoError(t, os.WriteFile(target, []byte("untouched"), 0o600))

		path := filepath.Join(b.TmpDir, "example-admin.kubeconfig")
		require.NoError(t, os.Symlink(target, path))

		require.NoError(t, b.saveAdminKubeconfig(t.Context(), []byte("apiVersion: v1\nkind: Config\n"), bctx))

		pointedAt, err := os.ReadFile(target)
		require.NoError(t, err)
		require.Equal(t, "untouched", string(pointedAt),
			"the credentials must not be written through a symlink somebody else planted")

		info, err := os.Lstat(path)
		require.NoError(t, err)
		require.Zero(t, info.Mode()&os.ModeSymlink, "the symlink must have been replaced by a regular file")
	})
}

// The InstallKubernetes phase must hand an immutable master to the API-server
// path instead of the bashible one: without the guard the bundle is installed
// into the installer container itself and the bootstrap reports success.
func TestInstallKubernetesTakesTheImmutablePath(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)

	err := b.bootstrapKubernetes(t.Context(), bctx)

	require.ErrorContains(t, err, "the first master address is unknown",
		"the phase must go to connectToImmutableMaster, not to the bashible bundle")
}

// immutableTestBootstrapper builds the smallest bootstrapper that can render
// the master payload.
func immutableTestBootstrapper(t *testing.T) (*ClusterBootstrapper, *bootstrapContext) {
	t.Helper()

	stateCache, err := cache.NewStateCache(t.TempDir())
	require.NoError(t, err)

	opts := options.New()
	// The default CandiDir points into a directory only the installer image
	// populates; left as is, the test would depend on leftovers of a previous
	// dhctl run.
	opts.Global.CandiDir = immutabletest.CandiDir(t)

	b := &ClusterBootstrapper{Params: &Params{Options: opts}}

	metaConfig := immutabletest.MetaConfig(t)

	return b, &bootstrapContext{
		metaConfig: metaConfig,
		stateCache: stateCache,
		immutable:  &immutableBootstrap{masterNodeName: firstMasterNodeName(metaConfig)},
	}
}

// noRetryCollapse restores real retry behaviour: init_test.go collapses every
// loop to one attempt, which would pass with no break predicate at all. Safe to
// swap globally — nothing in this file runs in parallel.

// testMachine stands in for a machine waiting in maintenance: it answers the
// inventory read that precedes every push, and records what it was asked.
type testMachine struct {
	host   string
	port   int
	pushed atomic.Bool
	reads  atomic.Int64
}

func newTestMachine(t *testing.T, inventory string) *testMachine {
	t.Helper()

	machine := &testMachine{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			machine.pushed.Store(true)
			return
		}
		machine.reads.Add(1)
		_, _ = io.WriteString(w, inventory)
	}))
	t.Cleanup(server.Close)

	machine.host, machine.port = splitTestServerAddress(t, server)
	return machine
}

// immutableWaitingBootstrapper is the smallest bootstrapper collectImmutableKubeconfig
// runs against: no SSH provider, and real retry behaviour.
func immutableWaitingBootstrapper(t *testing.T) (*ClusterBootstrapper, *bootstrapContext, *immutable.HandoffMaterial) {
	t.Helper()

	immutabletest.NoRetryCollapse(t)

	b, bctx := immutableTestBootstrapper(t)
	bctx.immutable.masterIP = "127.0.0.1"
	bctx.immutable.masterNodeName = "example-master-0"

	material, err := immutable.HandoffMaterialFor(t.Context(), bctx.stateCache, bctx.immutable.masterNodeName)
	require.NoError(t, err)

	return b, bctx, material
}

// A node that reports Failed has stopped working towards a control plane, so the
// wait must end with its message instead of polling for the rest of the half-hour
// budget. The test would take that half hour if BreakIf stopped matching.
func TestCollectImmutableKubeconfigStopsOnAFailedNode(t *testing.T) {
	b, bctx, material := immutableWaitingBootstrapper(t)

	// The fixture binds :0 rather than the protocol's fixed port, so a port that
	// is busy cannot silently skip this test.
	server := immutabletest.HandoffServer(t, material.ServerCertPEM, material.ServerKeyPEM, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"phase":"Failed","message":"pull the kubelet system extension: 404"}`))
	})
	_, port := splitTestServerAddress(t, server)

	// Bounded so that a loop which stops treating Failed as terminal fails this
	// assertion instead of running until the test binary is killed, which takes
	// every other test in the package down with it as a panic.
	ctx, cancel := context.WithTimeout(t.Context(), 2*waitAPIServerUp.interval)
	defer cancel()

	started := time.Now()
	_, err := b.collectImmutableKubeconfig(ctx, bctx, port)
	require.Error(t, err)
	require.Contains(t, err.Error(), "pull the kubelet system extension: 404",
		"the node's own message is the only thing that says what went wrong")
	require.Less(t, time.Since(started), waitAPIServerUp.interval,
		"a node that reported Failed must end the wait, not start the next attempt")
}

// A machine that was configured at one address and expected at another leaves
// the operator with two suspects: a static address it never took, or a machine
// that died. The failure has to name both addresses to tell them apart.
func TestCollectImmutableKubeconfigNamesBothAddresses(t *testing.T) {
	b, bctx, material := immutableWaitingBootstrapper(t)
	bctx.immutable.hosts = map[string]string{bctx.immutable.masterNodeName: "192.168.0.43"}

	server := immutabletest.HandoffServer(t, material.ServerCertPEM, material.ServerKeyPEM, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"phase":"Failed","message":"pull the kubelet system extension: 404"}`))
	})
	_, port := splitTestServerAddress(t, server)

	ctx, cancel := context.WithTimeout(t.Context(), 2*waitAPIServerUp.interval)
	defer cancel()

	_, err := b.collectImmutableKubeconfig(ctx, bctx, port)
	require.Error(t, err)
	require.Contains(t, err.Error(), "192.168.0.43", "the address the machine was configured through")
	require.Contains(t, err.Error(), bctx.immutable.masterIP, "the address it was expected to answer on")
}

// The wait is not the first thing a wrong installed address breaks: the channel
// to the master is opened before it, and through a bastion that open can fail.
// It has to name both addresses too, or the earlier failure says less.
func TestOpenImmutableChannelNamesBothAddresses(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)
	bctx.immutable.masterIP = "192.168.0.101"
	bctx.immutable.hosts = map[string]string{bctx.immutable.masterNodeName: "192.168.0.43"}
	// A bastion with no usable credentials: the open fails, which is the point,
	// and it fails without waiting on the network.
	b.SSHProviderInitializer = providerinitializer.NewSSHProviderInitializer(nil,
		&sshconfig.ConnectionConfig{Config: &sshconfig.Config{
			BastionHost: "127.0.0.1",
			BastionPort: new(1),
			BastionUser: "nobody",
		}})

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	_, _, err := b.openImmutableChannel(ctx, bctx, immutable.HandoffPort, "credentials handoff")
	require.Error(t, err)
	require.Contains(t, err.Error(), "192.168.0.43", "the address the machine was configured through")
	require.Contains(t, err.Error(), "192.168.0.101", "the address it was expected to answer on")
}

// staticInstanceResources is what an operator adds to a static cluster to have node-manager adopt
// a worker over SSH. The masters of an immutable cluster are not among them and answer no sshd.
const staticInstanceResources = `
apiVersion: deckhouse.io/v1alpha1
kind: SSHCredentials
metadata:
  name: worker-creds
spec:
  user: caps
  privateSSHKey: ZmFrZQ==
---
apiVersion: deckhouse.io/v1alpha1
kind: StaticInstance
metadata:
  name: worker-0
spec:
  address: 10.0.0.31
  credentialsRef:
    kind: SSHCredentials
    name: worker-creds
`

// The preflights of an immutable static cluster have to hold together as a suite of their own.
// Built as "the static suite minus a list of names", every check added to the static suite starts
// running against the installer container, and the one that walks the StaticInstances asks for an
// SSH provider this path guarantees does not exist — after the first master took its configuration.
func TestImmutableStaticPreflightsNeedNoSSHHost(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)
	bctx.metaConfig.ClusterType = config.StaticClusterType
	bctx.metaConfig.ResourcesYAML = staticInstanceResources
	b.SSHProviderInitializer = providerinitializer.NewSSHProviderInitializer(nil,
		&sshconfig.ConnectionConfig{Config: &sshconfig.Config{}})

	built, err := b.preflightSuites(t.Context(), bctx)
	require.NoError(t, err)
	require.Len(t, built, 2, "the global suite, and the arm this path is checked by")

	// Running it is the assertion: a check that reaches for the SSH provider ends here with
	// "hosts from cache not found", and one built over the node interface probes the installer
	// container instead of a machine.
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	runner := preflight.New(built[1])
	require.NoError(t, runner.Run(ctx, preflight.PhasePreInfra))
	require.NoError(t, runner.Run(ctx, preflight.PhasePostInfra))
}

// olcedar-init closes the maintenance port the moment it accepts the document (constants.go), so
// the machine can be correctly configured while the reply is lost on the way back. Recorded only
// after a reply, the rerun found no record, pushed again, and the agent of the now-installed node
// refused the second document terminally: a machine nothing was wrong with, and no way forward.
func TestBootstrapImmutableFirstMasterSurvivesALostReply(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)
	bctx.immutable.masterNodeName = "example-master-0"

	var pushes atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			return
		}
		if pushes.Add(1) == 1 {
			// The document is taken and the port closes before the reply is flushed.
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack the accepted push: %v", err)
				return
			}
			conn.Close()
			return
		}
		// What the agent of an installed node answers a second document.
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	host, port := splitTestServerAddress(t, server)
	bctx.immutable.hosts = map[string]string{"example-master-0": host}
	bctx.immutable.maintenancePort = port

	require.Error(t, b.bootstrapImmutableFirstMaster(t.Context(), bctx),
		"the run that lost the reply cannot know the machine took the document")

	require.NoError(t, b.bootstrapImmutableFirstMaster(t.Context(), bctx),
		"a rerun must not dead-end on the machine dhctl configured itself")
	require.Equal(t, host, bctx.immutable.masterIP, "the rerun still has to report the address")
}

// A document the machine cannot satisfy never leaves dhctl, so the record written before the push
// has to be retracted: kept, it would make the rerun with a corrected document skip the machine
// and wait for a master that was handed nothing.
func TestADocumentTheMachineRefusesLeavesNoPushRecord(t *testing.T) {
	// Two disks the rendered document's own ">=20Gi" system selector matches, so nothing in the
	// operator's input is needed to make this machine ambiguous.
	machine := newTestMachine(t, twoDisksOfOneSize)

	b, bctx := immutableTestBootstrapper(t)
	bctx.immutable.masterNodeName = "example-master-0"
	bctx.immutable.hosts = map[string]string{"example-master-0": machine.host}
	bctx.immutable.maintenancePort = machine.port

	require.ErrorIs(t, b.bootstrapImmutableFirstMaster(t.Context(), bctx), errDocumentUnfitForMachine)
	require.False(t, machine.pushed.Load(), "the machine must not be handed a document it cannot satisfy")

	pushed, err := payloadAlreadyPushed(t.Context(), bctx.stateCache, "example-master-0", machine.host)
	require.NoError(t, err)
	require.False(t, pushed, "a corrected rerun has to push, not skip")
}

// ClusterPrefix is assigned only on the cloud branch of config.Prepare, so keying the per-cluster
// name off it collapsed every static cluster onto one <TmpDir>/admin.kubeconfig. The write clears
// the path first, and the node stops offering the credentials once the handoff is confirmed: the
// second static cluster bootstrapped from an installer container deleted the first one's only way in.
func TestSaveAdminKubeconfigNamesTheFileAfterAStaticClusterToo(t *testing.T) {
	tmpDir := t.TempDir()

	save := func(clusterUUID string) string {
		b, bctx := immutableTestBootstrapper(t)
		b.TmpDir = tmpDir
		bctx.metaConfig.ClusterType = config.StaticClusterType
		bctx.metaConfig.ClusterPrefix = ""
		bctx.metaConfig.UUID = clusterUUID

		content := "apiVersion: v1\nkind: Config\n# " + clusterUUID + "\n"
		require.NoError(t, b.saveAdminKubeconfig(t.Context(), []byte(content), bctx))

		return bctx.immutable.kubeconfigPath
	}

	first := save("2c4b7bd4-0f5e-4d1a-9d13-6a0f4f2b1a01")
	second := save("9f1a0c2e-77c8-4f66-b0a2-1d2e3f4a5b02")

	require.NotEqual(t, first, second, "two clusters must not share one file")
	require.FileExists(t, first, "the second bootstrap must not delete the first cluster's credentials")

	kept, err := os.ReadFile(first)
	require.NoError(t, err)
	require.Contains(t, string(kept), "2c4b7bd4-0f5e-4d1a-9d13-6a0f4f2b1a01")

	// The tmp cleaner spares this file by suffix, as for a cloud cluster.
	require.True(t, strings.HasSuffix(first, cache.AdminKubeconfigName))
}
