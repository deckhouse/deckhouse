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
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/config"
	"fencing-agent/internal/domain"
)

// Every timing below is distinct so a swapped assignment cannot pass.
func testAgent() *Agent {
	return New(
		&config.Config{NodeGroup: "worker", MemberlistPort: 8500},
		Deps{},
		domain.NodeIdentity{Name: "worker-1", UID: "uid-1", IP: "10.0.0.1"},
		v1alpha1.FencingSLAProfileSpec{
			Memberlist: v1alpha1.FencingSLAProfileMemberlist{
				ProbeInterval: metav1.Duration{Duration: 300 * time.Millisecond},
				ProbeTimeout:  metav1.Duration{Duration: 120 * time.Millisecond},
				SuspicionMult: 3,
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
			Watchdog: v1alpha1.FencingSLAProfileWatchdog{
				FeedInterval: metav1.Duration{Duration: 6 * time.Second},
				Timeout:      metav1.Duration{Duration: 60 * time.Second},
			},
		},
		log.NewNop(),
	)
}

func TestMemberlistConfigCarriesIdentityAndTuning(t *testing.T) {
	cfg := testAgent().memberlistConfig()

	if cfg.NodeName != "worker-1" {
		t.Errorf("NodeName is %q, want worker-1", cfg.NodeName)
	}

	// Peers reach the agent through the hostPort on the Node address.
	if cfg.AdvertiseAddr != "10.0.0.1" {
		t.Errorf("AdvertiseAddr is %q, want the Node InternalIP", cfg.AdvertiseAddr)
	}

	if cfg.NodeGroup != "worker" || cfg.Port != 8500 {
		t.Errorf("group/port are %q/%d, want worker/8500", cfg.NodeGroup, cfg.Port)
	}

	if cfg.Tuning.ProbeInterval.Duration != 300*time.Millisecond ||
		cfg.Tuning.ProbeTimeout.Duration != 120*time.Millisecond ||
		cfg.Tuning.SuspicionMult != 3 {
		t.Errorf("memberlist tuning does not come from the profile: %+v", cfg.Tuning)
	}
}

// Three timings from three profile sections: a swap compiles and would only show
// up as a wrong retry pace.
func TestJoinParamsTakeTimingsFromTheirOwnProfileSections(t *testing.T) {
	params := testAgent().joinParams()

	if params.APITimeout != 2*time.Second {
		t.Errorf("APITimeout is %s, want fallback.kubernetesAPITimeout (2s)", params.APITimeout)
	}

	if params.RetryInterval != 5*time.Second {
		t.Errorf("RetryInterval is %s, want rejoin.interval (5s)", params.RetryInterval)
	}

	if params.MaxRetryInterval != 30*time.Second {
		t.Errorf("MaxRetryInterval is %s, want rejoin.maxInterval (30s)", params.MaxRetryInterval)
	}

	if params.NodeName != "worker-1" || params.NodeIP != "10.0.0.1" ||
		params.NodeGroup != "worker" || params.MemberlistPort != 8500 {
		t.Errorf("identity is not wired: %+v", params)
	}
}

// Two timings from one profile section: a swap compiles and would only show up as
// a wrong feed pace or kernel timeout.
func TestWatchdogParamsTakeTimingsFromTheProfileWatchdogSection(t *testing.T) {
	params := testAgent().watchdogParams()

	if params.FeedInterval != 6*time.Second {
		t.Errorf("FeedInterval is %s, want watchdog.feedInterval (6s)", params.FeedInterval)
	}

	if params.Timeout != 60*time.Second {
		t.Errorf("Timeout is %s, want watchdog.timeout (60s)", params.Timeout)
	}
}

func TestFallbackParamsTakeTimingsFromTheFallbackSection(t *testing.T) {
	params := testAgent().fallbackParams()

	if params.Heartbeat != 1*time.Second {
		t.Errorf("Heartbeat is %s, want fallback.heartbeat (1s)", params.Heartbeat)
	}

	if params.APITimeout != 2*time.Second {
		t.Errorf("APITimeout is %s, want fallback.kubernetesAPITimeout (2s)", params.APITimeout)
	}

	if params.Node.Name != "worker-1" || params.Node.UID != "uid-1" || params.Node.IP != "10.0.0.1" {
		t.Errorf("identity is not wired: %+v", params.Node)
	}
}

func TestRejoinParamsTakeTimingsFromTheRejoinSection(t *testing.T) {
	params := testAgent().rejoinParams()

	if params.Interval != 5*time.Second {
		t.Errorf("Interval is %s, want rejoin.interval (5s)", params.Interval)
	}

	if params.MaxInterval != 30*time.Second {
		t.Errorf("MaxInterval is %s, want rejoin.maxInterval (30s)", params.MaxInterval)
	}
}

func TestJoinParamsCarryTheNodeUID(t *testing.T) {
	if params := testAgent().joinParams(); params.NodeUID != "uid-1" {
		t.Errorf("NodeUID is %q, want the identity uid", params.NodeUID)
	}
}

// loopBarriers maps every background loop Run starts to the barrier channels its
// goroutine waits on. It reads the wiring instead of running it: Run arms a real
// watchdog device, which no unit test can provide.
func loopBarriers(t *testing.T) map[string][]string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "agent.go", nil, 0)
	if err != nil {
		t.Fatalf("parse agent.go: %v", err)
	}

	loops := make(map[string][]string)

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "Run" || fn.Recv == nil {
			continue
		}

		ast.Inspect(fn.Body, func(node ast.Node) bool {
			body, ok := goroutineBody(node)
			if !ok {
				return true
			}

			barriers := receivedIdents(body)

			for _, loop := range runCalls(body) {
				loops[loop] = barriers
			}

			return true
		})
	}

	if len(loops) == 0 {
		t.Fatal("no g.Go loops found in Run: the wiring this test reads has moved")
	}

	return loops
}

// goroutineBody returns the body of a `g.Go(func() error { ... })` argument.
func goroutineBody(node ast.Node) (*ast.BlockStmt, bool) {
	call, ok := node.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Go" {
		return nil, false
	}

	if group, ok := sel.X.(*ast.Ident); !ok || group.Name != "g" {
		return nil, false
	}

	lit, ok := call.Args[0].(*ast.FuncLit)
	if !ok {
		return nil, false
	}

	return lit.Body, true
}

func receivedIdents(body *ast.BlockStmt) []string {
	var names []string

	ast.Inspect(body, func(node ast.Node) bool {
		unary, ok := node.(*ast.UnaryExpr)
		if !ok || unary.Op != token.ARROW {
			return true
		}

		if ident, ok := unary.X.(*ast.Ident); ok && !slices.Contains(names, ident.Name) {
			names = append(names, ident.Name)
		}

		return true
	})

	return names
}

func runCalls(body *ast.BlockStmt) []string {
	var calls []string

	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Run" {
			return true
		}

		if receiver, ok := sel.X.(*ast.Ident); ok {
			calls = append(calls, receiver.Name+".Run")
		}

		return true
	})

	return calls
}


func TestFeedGateLoopsDoNotWaitForTheInformerCache(t *testing.T) {
	loops := loopBarriers(t)

	for _, loop := range []string{"monitor.Run", "rejoiner.Run"} {
		barriers, started := loops[loop]

		if !started {
			t.Errorf("no goroutine in Run starts %s", loop)

			continue
		}

		if slices.Contains(barriers, "synced") {
			t.Errorf("%s waits for the informer cache (barriers %v): an unreachable API would hold the watchdog feed gate open", loop, barriers)
		}

		if !slices.Contains(barriers, "joined") {
			t.Errorf("%s does not wait for the gossip join (barriers %v): before the join this agent sees no peers", loop, barriers)
		}
	}
}

func TestWatchdogIsArmedBehindTheJoinAndNothingElse(t *testing.T) {
	barriers, started := loopBarriers(t)["watchdogManager.Run"]

	if !started {
		t.Fatal("no goroutine in Run starts watchdogManager.Run")
	}

	if len(barriers) != 0 {
		t.Errorf("the watchdog waits on %v: it is armed by the goroutine that opens the join barrier", barriers)
	}
}

func TestReadinessNeedsTheInformerCache(t *testing.T) {
	yes := func() bool { return true }
	no := func() bool { return false }

	cases := map[string]struct {
		joined, watchdog, cache func() bool
		want                    bool
	}{
		"all three":            {joined: yes, watchdog: yes, cache: yes, want: true},
		"not joined":           {joined: no, watchdog: yes, cache: yes, want: false},
		"watchdog policy down": {joined: yes, watchdog: no, cache: yes, want: false},
		"cache never synced": {joined: yes, watchdog: yes, cache: no, want: false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := readiness(tc.joined, tc.watchdog, tc.cache)(); got != tc.want {
				t.Errorf("readiness = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCacheSyncDelayIsReportedOnceTheGraceExpires(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var reasons []string

	testAgent().reportCacheSyncDelay(ctx, make(chan struct{}), time.Millisecond, func(reason, message string) {
		reasons = append(reasons, reason)

		if !strings.Contains(message, "failed peer") {
			t.Errorf("message = %q, want it to name the consequence", message)
		}

		cancel()
	})

	if !slices.Equal(reasons, []string{cacheSyncWarning}) {
		t.Errorf("events = %v, want exactly one %s", reasons, cacheSyncWarning)
	}
}

func TestCacheSyncWithinTheGraceIsSilent(t *testing.T) {
	synced := make(chan struct{})
	close(synced)

	testAgent().reportCacheSyncDelay(t.Context(), synced, time.Minute, func(reason, _ string) {
		t.Errorf("unexpected %s event for a cache that synced in time", reason)
	})
}

func TestCacheSyncDelayStopsWithTheAgent(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	testAgent().reportCacheSyncDelay(ctx, make(chan struct{}), time.Minute, func(reason, _ string) {
		t.Errorf("unexpected %s event on shutdown", reason)
	})
}

func TestFallbackParamsCarryTheWatchdogTimeout(t *testing.T) {
	if params := testAgent().fallbackParams(); params.WatchdogTimeout != 60*time.Second {
		t.Errorf("WatchdogTimeout is %s, want watchdog.timeout (60s)", params.WatchdogTimeout)
	}
}
