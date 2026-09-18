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

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/adapters/memberlist"
	"fencing-agent/internal/config"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/usecase/failedstate"
)

const wiringMoved = "the wiring this test reads has moved"

func testAgentWithTimings() *Agent {
	return New(
		&config.Config{NodeGroup: "worker", MemberlistPort: 8500},
		Deps{},
		domain.NodeIdentity{Name: "worker-1", UID: "uid-1", IP: "10.0.0.1"},
		v1alpha1.FencingSLAProfileSpec{
			Memberlist: v1alpha1.FencingSLAProfileMemberlist{
				ProbeInterval:           metav1.Duration{Duration: 300 * time.Millisecond},
				ProbeTimeout:            metav1.Duration{Duration: 120 * time.Millisecond},
				SuspicionMult:           4,
				SuspicionMaxTimeoutMult: 6,
				IndirectChecks:          2,
				AwarenessMaxMultiplier:  8,
				GossipInterval:          metav1.Duration{Duration: 150 * time.Millisecond},
				RetransmitMult:          3,
				GossipToTheDeadTime:     metav1.Duration{Duration: 7 * time.Second},
			},
			Fallback: v1alpha1.FencingSLAProfileFallback{
				Heartbeat:            metav1.Duration{Duration: 1 * time.Second},
				TTL:                  metav1.Duration{Duration: 4 * time.Second},
				KubernetesAPITimeout: metav1.Duration{Duration: 2 * time.Second},
			},
			Rejoin: v1alpha1.FencingSLAProfileRejoin{
				Interval:    metav1.Duration{Duration: 5 * time.Second},
				MaxInterval: metav1.Duration{Duration: 30 * time.Second},
			},
			Evacuation: v1alpha1.FencingSLAProfileEvacuation{
				Delay: metav1.Duration{Duration: 12 * time.Second},
			},
			Watchdog: v1alpha1.FencingSLAProfileWatchdog{
				FeedInterval: metav1.Duration{Duration: 6 * time.Second},
				Timeout:      metav1.Duration{Duration: 60 * time.Second},
			},
		},
		log.NewNop(),
	)
}

func parseAgentRun(t *testing.T) *ast.FuncDecl {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "agent.go", nil, 0)
	if err != nil {
		t.Fatalf("parse agent.go: %v", err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "Run" || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}

		star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			continue
		}

		if recv, ok := star.X.(*ast.Ident); ok && recv.Name == "Agent" && fn.Body != nil {
			return fn
		}
	}

	t.Fatalf("(*Agent).Run not found in agent.go: %s", wiringMoved)

	return nil
}

func selectorCalls(node ast.Node, receiver, name string) []*ast.CallExpr {
	var calls []*ast.CallExpr

	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if isSelector(call.Fun, receiver, name) {
			calls = append(calls, call)
		}

		return true
	})

	return calls
}

func oneSelectorCall(t *testing.T, node ast.Node, receiver, name string) *ast.CallExpr {
	t.Helper()

	calls := selectorCalls(node, receiver, name)
	if len(calls) != 1 {
		t.Fatalf("found %d calls of %s.%s, want exactly one: %s", len(calls), receiver, name, wiringMoved)
	}

	return calls[0]
}

func compositeLits(node ast.Node, pkg, typeName string) []*ast.CompositeLit {
	var lits []*ast.CompositeLit

	ast.Inspect(node, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}

		if isSelector(lit.Type, pkg, typeName) {
			lits = append(lits, lit)
		}

		return true
	})

	return lits
}

func isSelector(expr ast.Expr, receiver, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)

	return ok && ident.Name == receiver
}

func TestMemberlistConfigCarriesTheAPITimeout(t *testing.T) {
	cfg := testAgentWithTimings().memberlistConfig()

	switch cfg.APITimeout {
	case 2 * time.Second:
	case 1 * time.Second:
		t.Error("APITimeout is fallback.heartbeat (1s), want fallback.kubernetesAPITimeout (2s)")
	case 5 * time.Second:
		t.Error("APITimeout is rejoin.interval (5s), want fallback.kubernetesAPITimeout (2s)")
	default:
		t.Errorf("APITimeout is %s, want fallback.kubernetesAPITimeout (2s)", cfg.APITimeout)
	}
}

func TestJoinerReadsNodesFromTheAPIClient(t *testing.T) {
	call := oneSelectorCall(t, parseAgentRun(t), "join", "New")
	if len(call.Args) < 2 {
		t.Fatalf("join.New(%s) has %d arguments, want the node reader and the expected source first: %s",
			exprList(call.Args), len(call.Args), wiringMoved)
	}

	reader, ok := call.Args[0].(*ast.CallExpr)
	if !ok || !isSelector(reader.Fun, "kubeclient", "NewNodes") {
		t.Errorf("join.New reads Nodes through %s, want kubeclient.NewNodes(a.deps.K8sClient)",
			types.ExprString(call.Args[0]))
	} else if len(reader.Args) != 1 || !isDepsField(reader.Args[0], "K8sClient") {
		t.Errorf("join.New reads Nodes through kubeclient.NewNodes(%s), want kubeclient.NewNodes(a.deps.K8sClient)",
			exprList(reader.Args))
	}

	if ident, ok := call.Args[1].(*ast.Ident); !ok || ident.Name != "members" {
		t.Errorf("join.New takes the expected Nodes from %s, want the informer cache view members",
			types.ExprString(call.Args[1]))
	}
}

func isDepsField(expr ast.Expr, field string) bool {
	sel, ok := expr.(*ast.SelectorExpr)

	return ok && sel.Sel.Name == field && isSelector(sel.X, "a", "deps")
}

func exprList(exprs []ast.Expr) string {
	parts := make([]string, 0, len(exprs))
	for _, expr := range exprs {
		parts = append(parts, types.ExprString(expr))
	}

	return strings.Join(parts, ", ")
}

func TestRejoinWiringReadsNeitherTheMonitorNorItsVerdict(t *testing.T) {
	run := parseAgentRun(t)

	lits := compositeLits(run, "rejoin", "Deps")
	if len(lits) != 1 {
		t.Fatalf("rejoin wiring: found %d rejoin.Deps literals in Run, want exactly one: %s", len(lits), wiringMoved)
	}

	rejoinNews := selectorCalls(run, "rejoin", "New")
	if len(rejoinNews) != 1 {
		t.Fatalf("rejoin wiring: found %d rejoin.New calls in Run, want exactly one: %s", len(rejoinNews), wiringMoved)
	}

	rejoinNew := rejoinNews[0]

	reported := map[*ast.Ident]bool{}
	rejectMonitor := func(node ast.Node, where string) {
		ast.Inspect(node, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if ok && ident.Name == "monitor" && !reported[ident] {
				reported[ident] = true
				t.Errorf("rejoin wiring: %s reads the fallback monitor, want no monitor in a rejoin.Deps field or a rejoin.New argument, monitor.ShouldFeed included",
					where)
			}

			return true
		})
	}

	for i, elt := range lits[0].Elts {
		field := fmt.Sprintf("element %d", i)
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			field = types.ExprString(kv.Key)

			if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "APIReachable" {
				t.Errorf("rejoin wiring: rejoin.Deps sets APIReachable to %s, want no APIReachable field: it gates the rejoin attempts on the API",
					types.ExprString(kv.Value))
			}
		}

		rejectMonitor(elt, "rejoin.Deps "+field)
	}

	for i, arg := range rejoinNew.Args {
		rejectMonitor(arg, fmt.Sprintf("rejoin.New argument %d", i))
	}

	definitions := 0
	defining := map[*ast.Ident]bool{}
	allowed := map[*ast.Ident]bool{}
	selectors := map[*ast.Ident]string{}
	callees := map[*ast.Ident]string{}

	ast.Inspect(run.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			if node.Tok != token.DEFINE {
				return true
			}

			for _, lhs := range node.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok && ident.Name == "monitor" {
					definitions++
					defining[ident] = true
				}
			}
		case *ast.SelectorExpr:
			ident, ok := node.X.(*ast.Ident)
			if ok && ident.Name == "monitor" {
				selectors[ident] = types.ExprString(node)
				allowed[ident] = node.Sel.Name == "Run" || node.Sel.Name == "ShouldFeed"
			}
		case *ast.CallExpr:
			for _, arg := range node.Args {
				if ident, ok := arg.(*ast.Ident); ok && ident.Name == "monitor" {
					callees[ident] = types.ExprString(node.Fun)
				}
			}
		case *ast.Ident:
			const rule = "want monitor used in Run only as monitor.Run and monitor.ShouldFeed, " +
				"since this test does not follow any other use to see whether it reaches rejoin"

			switch {
			case node.Name != "monitor", defining[node], allowed[node]:
			case callees[node] != "":
				t.Errorf("rejoin wiring: Run passes the fallback monitor to %s, %s", callees[node], rule)
			case selectors[node] != "":
				t.Errorf("rejoin wiring: Run uses %s, %s", selectors[node], rule)
			default:
				t.Errorf("rejoin wiring: Run uses the fallback monitor as a value, %s", rule)
			}
		}

		return true
	})

	if definitions != 1 {
		t.Fatalf("rejoin wiring: found %d definitions of monitor in Run, want exactly one: %s", definitions, wiringMoved)
	}
}

func TestRejoinDepsWireTheJoinerAndTheGossipNetwork(t *testing.T) {
	run := parseAgentRun(t)

	if attempt := depsField(t, run, "rejoin", "Attempt"); !isSelector(attempt, "joiner", "Attempt") {
		t.Errorf("rejoin.Deps Attempt is %s, want joiner.Attempt", types.ExprString(attempt))
	}

	if started := depsField(t, run, "rejoin", "EpisodeStarted"); !isSelector(started, "joiner", "StartEpisode") {
		t.Errorf("rejoin.Deps EpisodeStarted is %s, want joiner.StartEpisode: without it the join log dedupe of Bootstrap lasts the whole process",
			types.ExprString(started))
	}

	changed := depsField(t, run, "rejoin", "Changed")
	if call, ok := changed.(*ast.CallExpr); !ok || len(call.Args) != 0 || !isSelector(call.Fun, "cluster", "Changed") {
		t.Errorf("rejoin.Deps Changed is %s, want cluster.Changed()", types.ExprString(changed))
	}

	const wantNotMember = "func(err error) bool { return errors.Is(err, join.ErrNotMember) }"

	if !isNotMemberMatcher(depsField(t, run, "rejoin", "NotMember")) {
		t.Errorf("rejoin.Deps NotMember is not %s", wantNotMember)
	}
}

func isNotMemberMatcher(expr ast.Expr) bool {
	fn, ok := expr.(*ast.FuncLit)
	if !ok || fn.Type.Params == nil || len(fn.Type.Params.List) != 1 || len(fn.Type.Params.List[0].Names) != 1 {
		return false
	}

	if fn.Body == nil || len(fn.Body.List) != 1 {
		return false
	}

	ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}

	param := fn.Type.Params.List[0].Names[0].Name

	return types.ExprString(ret.Results[0]) == "errors.Is("+param+", join.ErrNotMember)"
}

func TestJoinAttemptsFeedNeitherTheFallbackGateNorTheWatchdog(t *testing.T) {
	run := parseAgentRun(t)

	for _, pkg := range []string{"fallback", "watchdog"} {
		lits := compositeLits(run, pkg, "Deps")
		if len(lits) != 1 {
			t.Fatalf("found %d %s.Deps literals in Run, want exactly one: %s", len(lits), pkg, wiringMoved)
		}

		for i, elt := range lits[0].Elts {
			field := fmt.Sprintf("element %d", i)
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				field = types.ExprString(kv.Key)
			}

			ast.Inspect(elt, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.Ident:
					if node.Name == "joiner" || node.Name == "rejoiner" {
						t.Errorf("%s.Deps %s reads %s, want no join attempt outcome in it", pkg, field, node.Name)
					}
				case *ast.CallExpr:
					if isSelector(node.Fun, "kubeclient", "NewNodes") {
						t.Errorf("%s.Deps %s calls kubeclient.NewNodes, want no fresh Node read in it", pkg, field)
					}
				}

				return true
			})
		}
	}
}

func TestSelfStateReachesOnlyTheOwnNodeWatcherAndTheWatchdog(t *testing.T) {
	run := parseAgentRun(t)

	const allowedUses = "kubeclient.NewSelfWatcher, watchdog.NewIdentityGuard, watchdog.Deps State " +
		"and selfState.Snapshot() in fallback.Deps InNodeGroup"

	allowed := map[*ast.Ident]bool{}

	allowArgs := func(receiver, name string) {
		for _, call := range selectorCalls(run, receiver, name) {
			for _, arg := range call.Args {
				if ident, ok := arg.(*ast.Ident); ok && ident.Name == "selfState" {
					allowed[ident] = true
				}
			}
		}
	}

	allowArgs("kubeclient", "NewSelfWatcher")
	allowArgs("watchdog", "NewIdentityGuard")

	if state, ok := depsField(t, run, "watchdog", "State").(*ast.Ident); ok && state.Name == "selfState" {
		allowed[state] = true
	}

	for _, call := range selectorCalls(depsField(t, run, "fallback", "InNodeGroup"), "selfState", "Snapshot") {
		if len(call.Args) == 0 {
			allowed[call.Fun.(*ast.SelectorExpr).X.(*ast.Ident)] = true
		}
	}

	definitions := 0

	ast.Inspect(run.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			if node.Tok != token.DEFINE {
				return true
			}

			for _, lhs := range node.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok && ident.Name == "selfState" {
					definitions++
					allowed[ident] = true
				}
			}
		case *ast.Ident:
			if node.Name == "selfState" && !allowed[node] {
				t.Errorf("Run uses selfState beyond %s, want no other use", allowedUses)
			}
		}

		return true
	})

	if definitions != 1 {
		t.Fatalf("found %d definitions of selfState in Run, want exactly one: %s", definitions, wiringMoved)
	}
}

func newJSONLogger(w io.Writer) *log.Logger {
	return log.NewLogger(
		log.WithOutput(w),
		log.WithHandlerType(log.JSONHandlerType),
		log.WithLevel(slog.LevelDebug),
	)
}

type logRecord map[string]any

func (r logRecord) msg() string {
	s, _ := r["msg"].(string)

	return s
}

func decodeLogs(t *testing.T, snapshot string) []logRecord {
	t.Helper()

	var records []logRecord

	dec := json.NewDecoder(strings.NewReader(snapshot))

	for dec.More() {
		var record logRecord
		if err := dec.Decode(&record); err != nil {
			t.Fatalf("log output is not JSON lines: %v", err)
		}

		records = append(records, record)
	}

	return records
}

var serviceLogKeys = map[string]bool{
	"level":      true,
	"logger":     true,
	"msg":        true,
	"source":     true,
	"stacktrace": true,
	"time":       true,
}

var snakeCaseKey = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func assertSnakeCaseKeys(t *testing.T, records []logRecord) {
	t.Helper()

	for _, record := range records {
		for key, value := range record {
			if serviceLogKeys[key] {
				continue
			}

			assertSnakeCaseKey(t, record.msg(), key, key, value)
		}
	}
}

func assertSnakeCaseKey(t *testing.T, msg, path, key string, value any) {
	t.Helper()

	if !snakeCaseKey.MatchString(key) {
		t.Errorf("log key %q in %q is not snake_case", path, msg)
	}

	nested, ok := value.(map[string]any)
	if !ok {
		return
	}

	for nestedKey, nestedValue := range nested {
		assertSnakeCaseKey(t, msg, path+"."+nestedKey, nestedKey, nestedValue)
	}
}

func TestStartupLogCarriesTheDerivedMemberlistTimings(t *testing.T) {
	var buf bytes.Buffer

	a := testAgentWithTimings()
	a.cfg.ProfileRefName = "standard"
	a.cfg.WatchdogDevice = "/dev/watchdog1"
	a.cfg.APISocketPath = "/run/fencing-agent.sock"
	a.logger = newJSONLogger(&buf)

	cfg := a.memberlistConfig()
	a.logStart(memberlist.DeriveTimings(cfg.Tuning, cfg.APITimeout))

	records := decodeLogs(t, buf.String())
	assertSnakeCaseKeys(t, records)

	if len(records) != 1 || records[0].msg() != "fencing-agent starting" {
		t.Fatalf("logStart wrote %d records %v, want one \"fencing-agent starting\"", len(records), records)
	}

	want := map[string]any{
		"node":                   "worker-1",
		"node_uid":               "uid-1",
		"node_ip":                "10.0.0.1",
		"node_group":             "worker",
		"profile":                "standard",
		"probe_interval":         "300ms",
		"memberlist_port":        float64(8500),
		"watchdog_device":        "/dev/watchdog1",
		"watchdog_feed_interval": "6s",
		"watchdog_timeout":       "1m0s",
		"api_socket_path":        "/run/fencing-agent.sock",
		"tcp_timeout":            "2s",
		"push_pull_interval":     "7s",
		"dead_node_reclaim_time": "7s",
		"leave_timeout":          "900ms",
	}

	record := records[0]

	for key, value := range want {
		got, ok := record[key]
		if !ok {
			t.Errorf("startup line has no %s, want %v", key, value)

			continue
		}

		if got != value {
			t.Errorf("startup line has %s=%v, want %v", key, got, value)
		}
	}

	for key, value := range record {
		if _, known := want[key]; !known && !serviceLogKeys[key] {
			t.Errorf("startup line has unexpected key %s=%v", key, value)
		}
	}

	buf.Reset()

	a.logStart(memberlist.Timings{
		TCPTimeout:          1100 * time.Millisecond,
		PushPullInterval:    2200 * time.Millisecond,
		DeadNodeReclaimTime: 3300 * time.Millisecond,
		LeaveTimeout:        440 * time.Millisecond,
	})

	records = decodeLogs(t, buf.String())
	if len(records) != 1 {
		t.Fatalf("logStart wrote %d records %v, want one", len(records), records)
	}

	for key, value := range map[string]string{
		"tcp_timeout":            "1.1s",
		"push_pull_interval":     "2.2s",
		"dead_node_reclaim_time": "3.3s",
		"leave_timeout":          "440ms",
	} {
		if got := records[0][key]; got != value {
			t.Errorf("startup line has %s=%v for hand-built timings, want %s", key, got, value)
		}
	}
}

func TestOwnFailedRecordIsFalseUntilTheFencingCacheSyncs(t *testing.T) {
	startedAt := failedstate.StartOfLife(time.Date(2026, time.September, 17, 10, 0, 0, 400_000_000, time.UTC))

	fresh := v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-1", CreationTimestamp: metav1.NewTime(startedAt.Add(3 * time.Second))},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Failed: &v1alpha1.FencingFailedNodeStateFailed{DetectedBy: "worker-2"},
		},
	}

	var (
		synced  bool
		calls   int
		listErr error
	)

	list := func(context.Context) ([]v1alpha1.FencingFailedNodeState, error) {
		calls++

		if !synced {
			t.Error("List ran before the fencing state cache synced, want no read until it syncs")
		}

		return []v1alpha1.FencingFailedNodeState{fresh}, listErr
	}

	trigger := ownFailedRecord(list, func() bool { return synced }, "worker-1", startedAt)

	for range 3 {
		if trigger(t.Context()) {
			t.Error("trigger is true before the fencing state cache synced, want false")
		}
	}

	if calls != 0 {
		t.Errorf("List ran %d times before the fencing state cache synced, want 0", calls)
	}

	synced = true

	if !trigger(t.Context()) {
		t.Error("trigger is false for a fresh failed record about this node after the cache synced, want true")
	}

	if calls != 1 {
		t.Errorf("List ran %d times for one synced check, want 1", calls)
	}

	listErr = errors.New("list fencingfailednodestates: context canceled")

	if trigger(t.Context()) {
		t.Error("trigger is true when List fails, want false")
	}
}

func TestStartupLogAndMemberlistReadOneConfig(t *testing.T) {
	run := parseAgentRun(t)

	if configs := selectorCalls(run, "a", "memberlistConfig"); len(configs) != 1 {
		t.Errorf("Run calls a.memberlistConfig() %d times, want once: the startup line and memberlist.New read one mlConfig",
			len(configs))
	}

	const wantConfig = "mlConfig := a.memberlistConfig()"

	reads, rejected := 0, 0

	for _, write := range writes(run.Body, "mlConfig") {
		if stmt, ok := write.(*ast.AssignStmt); ok {
			if call := definedCall(stmt, "mlConfig"); call != nil && len(call.Args) == 0 &&
				isSelector(call.Fun, "a", "memberlistConfig") {
				reads++

				continue
			}
		}

		rejected++

		t.Errorf("Run has %s, want mlConfig to stay what a.memberlistConfig() returned", writeString(write))
	}

	switch {
	case reads == 0 && rejected == 0:
		t.Fatalf("found no %s in Run: %s", wantConfig, wiringMoved)
	case reads > 1:
		t.Errorf("found %d %s in Run, want exactly one", reads, wantConfig)
	}

	const wantTimings = "memberlist.DeriveTimings(mlConfig.Tuning, mlConfig.APITimeout)"

	logStart := oneSelectorCall(t, run, "a", "logStart")
	if len(logStart.Args) != 1 {
		t.Fatalf("a.logStart(%s) has %d arguments, want the derived timings only: %s",
			exprList(logStart.Args), len(logStart.Args), wiringMoved)
	}

	timings, ok := logStart.Args[0].(*ast.CallExpr)

	switch {
	case !ok || !isSelector(timings.Fun, "memberlist", "DeriveTimings") || len(timings.Args) != 2:
		t.Errorf("the startup line logs %s, want %s", types.ExprString(logStart.Args[0]), wantTimings)
	case !isSelector(timings.Args[0], "mlConfig", "Tuning") || !isSelector(timings.Args[1], "mlConfig", "APITimeout"):
		t.Errorf("the startup line logs memberlist.DeriveTimings(%s), want %s", exprList(timings.Args), wantTimings)
	}

	if derives := selectorCalls(run, "memberlist", "DeriveTimings"); len(derives) != 1 {
		t.Errorf("Run calls memberlist.DeriveTimings %d times, want once, for the startup line: memberlist derives its own",
			len(derives))
	}

	create := oneSelectorCall(t, run, "memberlist", "New")
	if len(create.Args) == 0 {
		t.Fatalf("memberlist.New() has no arguments, want mlConfig first: %s", wiringMoved)
	}

	if ident, ok := create.Args[0].(*ast.Ident); !ok || ident.Name != "mlConfig" {
		t.Errorf("memberlist.New takes %s, want mlConfig, the config the startup line derives its timings from",
			types.ExprString(create.Args[0]))
	}
}

func TestWriterAndRejoinUseOneProcessStart(t *testing.T) {
	run := parseAgentRun(t)

	const wantStart = "startedAt := failedstate.StartOfLife(a.deps.StartedAt)"

	if lives := selectorCalls(run, "failedstate", "StartOfLife"); len(lives) != 1 {
		t.Errorf("Run calls failedstate.StartOfLife %d times, want once: the writer and the rejoin trigger share one startedAt",
			len(lives))
	}

	ast.Inspect(run.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && isTruncateToSecond(call) {
			t.Errorf("Run calls %s, want the second truncation left to failedstate.StartOfLife", types.ExprString(call))
		}

		return true
	})

	starts, rejected := 0, 0

	for _, write := range writes(run.Body, "startedAt") {
		if stmt, ok := write.(*ast.AssignStmt); ok && isProcessStartOfLife(stmt) {
			starts++

			continue
		}

		rejected++

		if len(selectorCalls(write, "time", "Now")) > 0 {
			t.Errorf("Run has %s, want startedAt from the process start: %s", writeString(write), wantStart)
		} else {
			t.Errorf("Run has %s, want only %s", writeString(write), wantStart)
		}
	}

	switch {
	case starts == 0 && rejected == 0:
		t.Fatalf("found no %s in Run: %s", wantStart, wiringMoved)
	case starts > 1:
		t.Errorf("found %d %s in Run, want exactly one", starts, wantStart)
	}

	handOvers := assigns(run.Body, func(lhs ast.Expr) bool { return isSelector(lhs, "params", "StartedAt") })
	if len(handOvers) != 1 {
		t.Fatalf("found %d assignments to params.StartedAt in Run, want exactly one params.StartedAt = startedAt: %s",
			len(handOvers), wiringMoved)
	}

	handOver := handOvers[0]
	if !isAssignOf(handOver, "startedAt") {
		t.Errorf("Run assigns %s, want params.StartedAt = startedAt", assignString(handOver))
	}

	writer := oneSelectorCall(t, run, "failedstate", "New")
	if len(writer.Args) == 0 {
		t.Fatalf("failedstate.New() has no arguments, want params first: %s", wiringMoved)
	}

	if ident, ok := writer.Args[0].(*ast.Ident); !ok || ident.Name != "params" {
		t.Errorf("failedstate.New takes %s, want params, which carries startedAt", types.ExprString(writer.Args[0]))
	}

	if handOver.Pos() > writer.Pos() {
		t.Error("params.StartedAt = startedAt comes after failedstate.New, want it before: the writer takes params by value")
	}

	var triggers []*ast.CallExpr

	ast.Inspect(run.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if fn, ok := call.Fun.(*ast.Ident); ok && fn.Name == "ownFailedRecord" {
				triggers = append(triggers, call)
			}
		}

		return true
	})

	if len(triggers) != 1 {
		t.Fatalf("found %d ownFailedRecord calls in Run, want exactly one: %s", len(triggers), wiringMoved)
	}

	const wantTrigger = "ownFailedRecord(states.List, closed(synced), a.identity.Name, startedAt)"

	if got := types.ExprString(triggers[0]); got != wantTrigger {
		t.Errorf("Run builds the rejoin trigger as %s, want %s", got, wantTrigger)
	}

	if value := depsField(t, run, "rejoin", "OwnFailedRecord"); value != triggers[0] {
		t.Errorf("rejoin.Deps OwnFailedRecord is %s, want the ownFailedRecord call itself: %s",
			types.ExprString(value), wantTrigger)
	}

	if value, ok := depsField(t, run, "failedstate", "States").(*ast.Ident); !ok || value.Name != "states" {
		t.Errorf("failedstate.Deps States is %s, want states, the store the rejoin trigger lists",
			types.ExprString(depsField(t, run, "failedstate", "States")))
	}
}

func depsField(t *testing.T, run *ast.FuncDecl, pkg, field string) ast.Expr {
	t.Helper()

	lits := compositeLits(run, pkg, "Deps")
	if len(lits) != 1 {
		t.Fatalf("found %d %s.Deps literals in Run, want exactly one: %s", len(lits), pkg, wiringMoved)
	}

	for _, elt := range lits[0].Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		if key, ok := kv.Key.(*ast.Ident); ok && key.Name == field {
			return kv.Value
		}
	}

	t.Fatalf("%s.Deps in Run sets no %s: %s", pkg, field, wiringMoved)

	return nil
}

func TestShutdownDisarmsTheWatchdogBeforeLeavingGossip(t *testing.T) {
	run := parseAgentRun(t)
	defers := frameDefers(run)

	disarm := enclosingDefer(defers, oneSelectorCall(t, run, "watchdogManager", "Close"))

	leave := enclosingDefer(defers, oneSelectorCall(t, run, "cluster", "Shutdown"))
	if leave == nil {
		t.Fatalf("cluster.Shutdown() is not deferred by Run itself: %s", wiringMoved)
	}

	switch {
	case disarm == nil:
		t.Error("watchdogManager.Close() is not deferred by Run itself, want the backstop disarm on every return of Run")
	case disarm == leave:
		t.Fatalf("watchdogManager.Close() and cluster.Shutdown() share one defer, want a defer each: %s", wiringMoved)
	case disarm.Pos() < leave.Pos():
		t.Error("defer watchdogManager.Close() is registered before the defer calling cluster.Shutdown(), " +
			"want it after: LIFO would leave gossip with the watchdog still armed")
	}

	feed := oneSelectorCall(t, run, "watchdogManager", "Run")
	if len(feed.Args) != 1 {
		t.Fatalf("watchdogManager.Run(%s) has %d arguments, want the group context only: %s",
			exprList(feed.Args), len(feed.Args), wiringMoved)
	}

	if ident, ok := feed.Args[0].(*ast.Ident); !ok || ident.Name != "gctx" {
		t.Errorf("watchdogManager.Run takes %s, want gctx: the feed loop must stop when any loop of the group fails",
			types.ExprString(feed.Args[0]))
	}
}

func assigns(node ast.Node, match func(lhs ast.Expr) bool) []*ast.AssignStmt {
	var found []*ast.AssignStmt

	ast.Inspect(node, func(n ast.Node) bool {
		stmt, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		for _, lhs := range stmt.Lhs {
			if match(lhs) {
				found = append(found, stmt)

				break
			}
		}

		return true
	})

	return found
}

func writes(node ast.Node, name string) []ast.Node {
	var found []ast.Node

	ast.Inspect(node, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			if slices.ContainsFunc(stmt.Lhs, func(lhs ast.Expr) bool { return rootIdent(lhs) == name }) {
				found = append(found, stmt)
			}
		case *ast.IncDecStmt:
			if rootIdent(stmt.X) == name {
				found = append(found, stmt)
			}
		case *ast.ValueSpec:
			if slices.ContainsFunc(stmt.Names, func(ident *ast.Ident) bool { return ident.Name == name }) {
				found = append(found, stmt)
			}
		}

		return true
	})

	return found
}

func writeString(write ast.Node) string {
	switch stmt := write.(type) {
	case *ast.AssignStmt:
		return assignString(stmt)
	case *ast.IncDecStmt:
		return types.ExprString(stmt.X) + stmt.Tok.String()
	case *ast.ValueSpec:
		names := make([]string, 0, len(stmt.Names))
		for _, ident := range stmt.Names {
			names = append(names, ident.Name)
		}

		decl := "var " + strings.Join(names, ", ")
		if stmt.Type != nil {
			decl += " " + types.ExprString(stmt.Type)
		}

		if len(stmt.Values) > 0 {
			decl += " = " + exprList(stmt.Values)
		}

		return decl
	}

	return fmt.Sprintf("%T", write)
}

func rootIdent(expr ast.Expr) string {
	for {
		switch e := expr.(type) {
		case *ast.Ident:
			return e.Name
		case *ast.SelectorExpr:
			expr = e.X
		default:
			return ""
		}
	}
}

func definedCall(stmt *ast.AssignStmt, name string) *ast.CallExpr {
	if stmt.Tok != token.DEFINE || len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
		return nil
	}

	if ident, ok := stmt.Lhs[0].(*ast.Ident); !ok || ident.Name != name {
		return nil
	}

	call, _ := stmt.Rhs[0].(*ast.CallExpr)

	return call
}

func isProcessStartOfLife(stmt *ast.AssignStmt) bool {
	call := definedCall(stmt, "startedAt")

	return call != nil && isSelector(call.Fun, "failedstate", "StartOfLife") &&
		len(call.Args) == 1 && isDepsField(call.Args[0], "StartedAt")
}

func isAssignOf(stmt *ast.AssignStmt, name string) bool {
	if stmt.Tok != token.ASSIGN || len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
		return false
	}

	rhs, ok := stmt.Rhs[0].(*ast.Ident)

	return ok && rhs.Name == name
}

func isTruncateToSecond(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)

	return ok && sel.Sel.Name == "Truncate" && len(call.Args) == 1 && isSelector(call.Args[0], "time", "Second")
}

func assignString(stmt *ast.AssignStmt) string {
	return exprList(stmt.Lhs) + " " + stmt.Tok.String() + " " + exprList(stmt.Rhs)
}

func frameDefers(run *ast.FuncDecl) []*ast.DeferStmt {
	var defers []*ast.DeferStmt

	ast.Inspect(run.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.DeferStmt:
			defers = append(defers, node)

			return false
		case *ast.FuncLit:
			return false
		}

		return true
	})

	return defers
}

func enclosingDefer(defers []*ast.DeferStmt, call *ast.CallExpr) *ast.DeferStmt {
	for _, d := range defers {
		if d.Pos() <= call.Pos() && call.End() <= d.End() {
			return d
		}
	}

	return nil
}
