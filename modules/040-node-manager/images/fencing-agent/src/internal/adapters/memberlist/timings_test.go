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

package memberlist

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

const (
	shippedProfilesPath = "../../../../../../templates/fencing-agent/sla-profiles.yaml"
	agentDaemonSetPath  = "../../../../../../templates/fencing-agent/daemonset.yaml"

	healthShutdownBudget = 5 * time.Second
	shutdownSlack        = time.Second
)

func TestTCPTimeoutHasAOneSecondFloor(t *testing.T) {
	tests := []struct {
		apiTimeout time.Duration
		want       time.Duration
	}{
		{0, time.Second},
		{time.Nanosecond, time.Second},
		{500 * time.Millisecond, time.Second},
		{999 * time.Millisecond, time.Second},
		{time.Second, time.Second},
		{1001 * time.Millisecond, 1001 * time.Millisecond},
		{30 * time.Second, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.apiTimeout.String(), func(t *testing.T) {
			got := DeriveTimings(v1alpha1.FencingSLAProfileMemberlist{}, tt.apiTimeout).TCPTimeout
			if got != tt.want {
				t.Errorf("TCPTimeout for API timeout %s is %s, want %s", tt.apiTimeout, got, tt.want)
			}
		})
	}
}

func TestPushPullIntervalKeepsTwoTCPTimeouts(t *testing.T) {
	tests := []struct {
		name                string
		gossipToTheDeadTime time.Duration
		apiTimeout          time.Duration
		want                time.Duration
	}{
		{"two TCP timeouts win over an equal dead time", 5 * time.Second, 5 * time.Second, 10 * time.Second},
		{"two floored TCP timeouts win over a short dead time", time.Second, 0, 2 * time.Second},
		{"an equal dead time and two TCP timeouts", 20 * time.Second, 10 * time.Second, 20 * time.Second},
		{"a longer dead time wins", 20 * time.Second, 5 * time.Second, 20 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tuning := v1alpha1.FencingSLAProfileMemberlist{
				GossipToTheDeadTime: metav1.Duration{Duration: tt.gossipToTheDeadTime},
			}

			got := DeriveTimings(tuning, tt.apiTimeout).PushPullInterval
			if got != tt.want {
				t.Errorf("PushPullInterval for gossipToTheDeadTime %s and API timeout %s is %s, want %s",
					tt.gossipToTheDeadTime, tt.apiTimeout, got, tt.want)
			}
		})
	}
}

func TestLeaveTimeoutIsClampedTo250msAnd4s(t *testing.T) {
	tests := []struct {
		name           string
		retransmitMult int32
		gossipInterval time.Duration
		want           time.Duration
	}{
		{"inside the bounds", 4, 100 * time.Millisecond, 800 * time.Millisecond},
		{"below the floor", 1, time.Millisecond, 250 * time.Millisecond},
		{"above the ceiling", 4, time.Second, 4 * time.Second},
		{"a huge product saturates", math.MaxInt32, time.Hour, 4 * time.Second},
		{"an int64 product that wraps negative", math.MaxInt32, 3 * time.Second, 4 * time.Second},
		{"a doubling that overflows int32", 1 << 30, time.Nanosecond, 1 << 31 * time.Nanosecond},
		{"zero retransmitMult", 0, 100 * time.Millisecond, 250 * time.Millisecond},
		{"zero gossipInterval", 4, 0, 250 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tuning := v1alpha1.FencingSLAProfileMemberlist{
				GossipInterval: metav1.Duration{Duration: tt.gossipInterval},
				RetransmitMult: tt.retransmitMult,
			}

			got := DeriveTimings(tuning, 0).LeaveTimeout
			if got != tt.want {
				t.Errorf("LeaveTimeout for retransmitMult %d and gossipInterval %s is %s, want %s",
					tt.retransmitMult, tt.gossipInterval, got, tt.want)
			}
		})
	}
}

func loadShippedProfiles(t *testing.T) []v1alpha1.FencingSLAProfile {
	t.Helper()

	raw, err := os.ReadFile(shippedProfilesPath)
	if err != nil {
		t.Skipf("shipped profiles are not reachable: %v", err)
	}

	var clean []string
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.Contains(line, "{{") {
			clean = append(clean, line)
		}
	}

	var profiles []v1alpha1.FencingSLAProfile
	for _, doc := range strings.Split(strings.Join(clean, "\n"), "\n---\n") {
		if !strings.Contains(doc, "kind: FencingSLAProfile") {
			continue
		}

		var p v1alpha1.FencingSLAProfile
		if err := yaml.Unmarshal([]byte(doc), &p); err != nil {
			t.Fatalf("unmarshal shipped profile: %v\n%s", err, doc)
		}

		profiles = append(profiles, p)
	}

	if len(profiles) != 4 {
		t.Fatalf("expected 4 built-in profiles in %s, parsed %d", shippedProfilesPath, len(profiles))
	}

	return profiles
}

func TestDeriveTimingsForEveryShippedProfile(t *testing.T) {
	want := map[string]Timings{
		"critical": {
			TCPTimeout:          time.Second,
			PushPullInterval:    2 * time.Second,
			DeadNodeReclaimTime: 2 * time.Second,
			LeaveTimeout:        800 * time.Millisecond,
		},
		"medium": {
			TCPTimeout:          2 * time.Second,
			PushPullInterval:    5 * time.Second,
			DeadNodeReclaimTime: 5 * time.Second,
			LeaveTimeout:        1600 * time.Millisecond,
		},
		"moderate": {
			TCPTimeout:          5 * time.Second,
			PushPullInterval:    10 * time.Second,
			DeadNodeReclaimTime: 10 * time.Second,
			LeaveTimeout:        2400 * time.Millisecond,
		},
		"slow": {
			TCPTimeout:          10 * time.Second,
			PushPullInterval:    20 * time.Second,
			DeadNodeReclaimTime: 20 * time.Second,
			LeaveTimeout:        4 * time.Second,
		},
	}

	seen := make(map[string]bool, len(want))
	for _, p := range loadShippedProfiles(t) {
		expected, ok := want[p.Name]
		if !ok {
			t.Errorf("shipped profile %q has no expected timings", p.Name)
			continue
		}

		if seen[p.Name] {
			t.Errorf("shipped profile %q is defined more than once", p.Name)
		}
		seen[p.Name] = true

		got := DeriveTimings(p.Spec.Memberlist, p.Spec.Fallback.KubernetesAPITimeout.Duration)
		if got != expected {
			t.Errorf("DeriveTimings for shipped profile %q = %+v, want %+v", p.Name, got, expected)
		}
	}

	for name := range want {
		if !seen[name] {
			t.Errorf("shipped profile %q is missing from %s", name, shippedProfilesPath)
		}
	}
}

func TestDerivedLeaveTimeoutFitsTheTerminationGracePeriod(t *testing.T) {
	raw, err := os.ReadFile(agentDaemonSetPath)
	if err != nil {
		t.Skipf("agent DaemonSet is not reachable: %v", err)
	}

	matches := regexp.MustCompile(`terminationGracePeriodSeconds:\s*(\d+)`).FindAllSubmatch(raw, -1)
	if len(matches) == 0 {
		t.Fatalf("no numeric terminationGracePeriodSeconds in %s", agentDaemonSetPath)
	}

	type tuningCase struct {
		name       string
		tuning     v1alpha1.FencingSLAProfileMemberlist
		apiTimeout time.Duration
	}

	shipped := loadShippedProfiles(t)
	cases := make([]tuningCase, 0, len(shipped)+1)
	for _, p := range shipped {
		cases = append(cases, tuningCase{
			name:       "shipped " + p.Name,
			tuning:     p.Spec.Memberlist,
			apiTimeout: p.Spec.Fallback.KubernetesAPITimeout.Duration,
		})
	}
	cases = append(cases, tuningCase{
		name: "custom with a huge retransmitMult and gossipInterval",
		tuning: v1alpha1.FencingSLAProfileMemberlist{
			GossipInterval: metav1.Duration{Duration: time.Hour},
			RetransmitMult: 1000,
		},
	})

	for _, m := range matches {
		seconds, err := strconv.Atoi(string(m[1]))
		if err != nil {
			t.Fatalf("parse terminationGracePeriodSeconds %q: %v", m[1], err)
		}

		grace := time.Duration(seconds) * time.Second

		for _, tc := range cases {
			leave := DeriveTimings(tc.tuning, tc.apiTimeout).LeaveTimeout
			if budget := leave + healthShutdownBudget + shutdownSlack; budget > grace {
				t.Errorf("%s: LeaveTimeout %s + health shutdown %s + slack %s = %s exceeds terminationGracePeriodSeconds %s",
					tc.name, leave, healthShutdownBudget, shutdownSlack, budget, grace)
			}
		}
	}
}

func TestShutdownLeavesWithTheDerivedTimeout(t *testing.T) {
	cfg := Config{
		NodeName:      "worker-1",
		NodeGroup:     "worker",
		AdvertiseAddr: "127.0.0.1",
		Port:          0,
		Tuning:        testTuning(),
		APITimeout:    1500 * time.Millisecond,
	}

	cluster, err := New(cfg, log.NewNop())
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	t.Cleanup(func() {
		if err := cluster.Shutdown(); err != nil {
			t.Errorf("shutdown: %v", err)
		}
	})

	if want := DeriveTimings(cfg.Tuning, cfg.APITimeout).LeaveTimeout; cluster.leaveTimeout != want {
		t.Errorf("New stored leaveTimeout %s, want the derived %s", cluster.leaveTimeout, want)
	}

	file, err := parser.ParseFile(token.NewFileSet(), "memberlist.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse memberlist.go: %v", err)
	}

	var shutdown *ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv != nil && len(fn.Recv.List) == 1 && fn.Name.Name == "Shutdown" &&
			types.ExprString(fn.Recv.List[0].Type) == "*Cluster" {
			shutdown = fn
			break
		}
	}

	if shutdown == nil || shutdown.Body == nil {
		t.Fatal("memberlist.go has no Shutdown method on *Cluster")
	}

	if len(shutdown.Recv.List[0].Names) != 1 {
		t.Fatal("Shutdown has no named receiver, so it cannot call Leave on its memberlist")
	}

	recv := shutdown.Recv.List[0].Names[0].Name

	var leaves []*ast.CallExpr
	ast.Inspect(shutdown.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && types.ExprString(call.Fun) == recv+".list.Leave" {
			leaves = append(leaves, call)
		}
		return true
	})

	if len(leaves) == 0 {
		t.Fatalf("Shutdown does not call %s.list.Leave: the derived leave timeout never reaches memberlist", recv)
	}

	for _, call := range leaves {
		args := make([]string, 0, len(call.Args))
		for _, arg := range call.Args {
			args = append(args, types.ExprString(arg))
		}

		if len(call.Args) != 1 {
			t.Errorf("Shutdown calls %s.list.Leave(%s), want exactly one argument %s.leaveTimeout",
				recv, strings.Join(args, ", "), recv)
			continue
		}

		if _, ok := call.Args[0].(*ast.SelectorExpr); !ok || args[0] != recv+".leaveTimeout" {
			t.Errorf("Shutdown calls %s.list.Leave(%s), want %s.list.Leave(%s.leaveTimeout)",
				recv, args[0], recv, recv)
		}
	}
}
