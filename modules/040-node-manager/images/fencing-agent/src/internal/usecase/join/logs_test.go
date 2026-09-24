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

package join

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"fencing-agent/internal/domain"
	"fencing-agent/internal/logtest"
)

const (
	completedMsg = "memberlist join completed"
	partialMsg   = "some seeds are unreachable, gossip will converge"
	droppedMsg   = "join candidate dropped"
)

func lineOf(record logtest.Record) string {
	return strings.Join([]string{record.Msg(), record.Level(), record.Str("member"), record.Str("reason")}, "|")
}

func unleveledLineOf(record logtest.Record) string {
	return strings.Join([]string{record.Msg(), record.Str("member"), record.Str("reason")}, "|")
}

func loudLines(records []logtest.Record) []string {
	var lines []string

	for _, record := range records {
		if record.Level() != "debug" {
			lines = append(lines, lineOf(record))
		}
	}

	slices.Sort(lines)

	return lines
}

func TestJoinPathLinesAreDedupedAcrossAttempts(t *testing.T) {
	const attempts = 10

	twoPeers := func() (*fakeNodes, *fakeExpected, *fakeCluster) {
		nodes, expected := mirroredGroup(
			selfPeer(),
			domain.Peer{Name: "worker-2", IP: "10.0.0.2"},
			domain.Peer{Name: "worker-3", IP: "10.0.0.3"},
		)

		return nodes, expected, &fakeCluster{}
	}

	cases := []struct {
		name    string
		group   func() (*fakeNodes, *fakeExpected, *fakeCluster)
		wantErr bool
		want    []string
	}{
		{
			name:  "every seed joined",
			group: twoPeers,
			want:  []string{completedMsg + "|info||"},
		},
		{
			name: "some seeds unreachable",
			group: func() (*fakeNodes, *fakeExpected, *fakeCluster) {
				nodes, expected, cluster := twoPeers()
				cluster.joinFn = func([]string) (int, error) { return 1, nil }

				return nodes, expected, cluster
			},
			want: []string{completedMsg + "|info||", partialMsg + "|warn||"},
		},
		{
			name: "every candidate dropped",
			group: func() (*fakeNodes, *fakeExpected, *fakeCluster) {
				nodes, expected := mirroredGroup(
					selfPeer(),
					domain.Peer{Name: "worker-2", IP: "10.0.0.2"},
					domain.Peer{Name: "worker-3", IP: "10.0.0.3"},
					domain.Peer{Name: "worker-4", IP: "10.0.0.4"},
				)
				nodes.setAnswer("worker-2", nodeAnswer{record: peerRecord("worker-2", "")})
				nodes.setAnswer("worker-3", nodeAnswer{err: notFound("worker-3")})
				nodes.setAnswer("worker-4", nodeAnswer{err: errors.New("etcd is unavailable")})

				return nodes, expected, &fakeCluster{}
			},
			wantErr: true,
			want: []string{
				droppedMsg + "|info|worker-3|not_found",
				droppedMsg + "|warn|worker-2|no_internal_ip",
				droppedMsg + "|warn|worker-4|read_failed",
			},
		},
		{
			name: "stale clone next to a peer",
			group: func() (*fakeNodes, *fakeExpected, *fakeCluster) {
				nodes, expected := mirroredGroup(
					selfPeer(),
					domain.Peer{Name: "worker-1-old", IP: testNodeIP},
					domain.Peer{Name: "worker-2", IP: "10.0.0.2"},
				)

				return nodes, expected, &fakeCluster{}
			},
			want: []string{cloneMsg + "|warn|worker-1-old|", completedMsg + "|info||"},
		},
		{
			name: "alone with a stale clone",
			group: func() (*fakeNodes, *fakeExpected, *fakeCluster) {
				nodes, expected := mirroredGroup(
					selfPeer(),
					domain.Peer{Name: "worker-1-old", IP: testNodeIP},
				)

				return nodes, expected, &fakeCluster{}
			},
			want: []string{aloneMsg + "|info||", cloneMsg + "|warn|worker-1-old|"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes, expected, cluster := tc.group()

			var logs bytes.Buffer

			joiner := New(nodes, expected, cluster, joinerParams(), logtest.NewJSONLogger(&logs))

			attempt := func() {
				if err := joiner.Attempt(t.Context()); (err != nil) != tc.wantErr {
					t.Fatalf("attempt returned %v, want an error: %t", err, tc.wantErr)
				}
			}

			attempt()

			first := logtest.Drain(t, &logs)
			if got, want := loudLines(first), slices.Sorted(slices.Values(tc.want)); !slices.Equal(got, want) {
				t.Fatalf("first attempt lines above debug are %v, want %v", got, want)
			}

			for range attempts - 1 {
				attempt()
			}

			rest := logtest.Drain(t, &logs)
			logtest.AssertSnakeCaseKeys(t, slices.Concat(first, rest))

			if got := loudLines(slices.Concat(first, rest)); len(got) != len(tc.want) {
				t.Errorf("lines above debug after %d attempts are %v, want the %d of the first attempt only", attempts, got, len(tc.want))
			}

			if len(rest) != (attempts-1)*len(first) {
				t.Errorf("attempts 2 to %d wrote %d records, want the %d lines of the first attempt each", attempts, len(rest), len(first))
			}

			firstLines := make(map[string]bool, len(first))
			for _, record := range first {
				firstLines[unleveledLineOf(record)] = true
			}

			for _, record := range rest {
				if record.Level() != "debug" || !firstLines[unleveledLineOf(record)] {
					t.Errorf("repeated record %v, want a line of the first attempt at level debug", record)
				}
			}
		})
	}

	t.Run("another member or reason gets its own line", func(t *testing.T) {
		nodes, expected := mirroredGroup(
			selfPeer(),
			domain.Peer{Name: "worker-2", IP: "10.0.0.2"},
		)
		nodes.setAnswer("worker-2", nodeAnswer{record: peerRecord("worker-2", "")})
		cluster := &fakeCluster{}

		var logs bytes.Buffer

		joiner := New(nodes, expected, cluster, joinerParams(), logtest.NewJSONLogger(&logs))

		attemptDrops := func() []string {
			if err := joiner.Attempt(t.Context()); err == nil {
				t.Fatal("attempt succeeded, want every candidate dropped")
			}

			records := logtest.Drain(t, &logs)
			logtest.AssertSnakeCaseKeys(t, records)

			var lines []string
			for _, record := range logtest.WithMsg(records, droppedMsg) {
				lines = append(lines, lineOf(record))
			}

			return lines
		}

		steps := []struct {
			name  string
			setup func()
			want  string
		}{
			{name: "first drop", setup: func() {}, want: droppedMsg + "|warn|worker-2|no_internal_ip"},
			{name: "same member and reason", setup: func() {}, want: droppedMsg + "|debug|worker-2|no_internal_ip"},
			{
				name: "same member, another reason",
				setup: func() {
					nodes.setAnswer("worker-2", nodeAnswer{err: errors.New("etcd is unavailable")})
				},
				want: droppedMsg + "|warn|worker-2|read_failed",
			},
			{
				name: "same member and reason, another error",
				setup: func() {
					nodes.setAnswer("worker-2", nodeAnswer{err: errors.New("etcd is still unavailable")})
				},
				want: droppedMsg + "|debug|worker-2|read_failed",
			},
			{
				name: "another member, same reason",
				setup: func() {
					expected.setPeers([]domain.Peer{selfPeer(), {Name: "worker-3", IP: "10.0.0.3"}})
					nodes.setAnswer("worker-3", nodeAnswer{err: errors.New("etcd is unavailable")})
				},
				want: droppedMsg + "|warn|worker-3|read_failed",
			},
		}

		for _, step := range steps {
			step.setup()

			if got := attemptDrops(); !slices.Equal(got, []string{step.want}) {
				t.Errorf("%s: drop lines are %v, want %v", step.name, got, []string{step.want})
			}
		}
	})

	t.Run("another clone gets its own line", func(t *testing.T) {
		nodes, expected := mirroredGroup(
			selfPeer(),
			domain.Peer{Name: "worker-1-old", IP: testNodeIP},
		)
		cluster := &fakeCluster{}

		var logs bytes.Buffer

		joiner := New(nodes, expected, cluster, joinerParams(), logtest.NewJSONLogger(&logs))

		attemptClones := func() []string {
			if err := joiner.Attempt(t.Context()); err != nil {
				t.Fatalf("attempt returned %v, want this node to start alone", err)
			}

			records := logtest.Drain(t, &logs)
			logtest.AssertSnakeCaseKeys(t, records)

			var lines []string
			for _, record := range logtest.WithMsg(records, cloneMsg) {
				lines = append(lines, lineOf(record))
			}

			slices.Sort(lines)

			return lines
		}

		steps := []struct {
			name  string
			setup func()
			want  []string
		}{
			{name: "first clone", setup: func() {}, want: []string{cloneMsg + "|warn|worker-1-old|"}},
			{
				name: "another clone",
				setup: func() {
					expected.setPeers([]domain.Peer{
						selfPeer(),
						{Name: "worker-1-old", IP: testNodeIP},
						{Name: "worker-1-older", IP: testNodeIP},
					})
				},
				want: []string{cloneMsg + "|debug|worker-1-old|", cloneMsg + "|warn|worker-1-older|"},
			},
			{
				name:  "same clones",
				setup: func() {},
				want:  []string{cloneMsg + "|debug|worker-1-old|", cloneMsg + "|debug|worker-1-older|"},
			},
		}

		for _, step := range steps {
			step.setup()

			if got, want := attemptClones(), slices.Sorted(slices.Values(step.want)); !slices.Equal(got, want) {
				t.Errorf("%s: clone lines are %v, want %v", step.name, got, want)
			}
		}
	})
}

func TestStartEpisodeRestartsTheDedupe(t *testing.T) {
	nodes, expected := mirroredGroup(
		selfPeer(),
		domain.Peer{Name: "worker-2", IP: "10.0.0.2"},
	)
	cluster := &fakeCluster{}

	var logs bytes.Buffer

	joiner := New(nodes, expected, cluster, joinerParams(), logtest.NewJSONLogger(&logs))

	attempts := func(n int) {
		for range n {
			if err := joiner.Attempt(t.Context()); err != nil {
				t.Fatalf("attempt returned %v, want worker-2 to be joined", err)
			}
		}
	}

	attempts(3)
	joiner.StartEpisode()
	attempts(3)

	joiner.Bootstrap(t.Context())

	records := logtest.Drain(t, &logs)
	logtest.AssertSnakeCaseKeys(t, records)

	levels := logtest.Levels(logtest.WithMsg(records, completedMsg))

	want := []string{"info", "debug", "debug", "info", "debug", "debug", "info"}
	if !slices.Equal(levels, want) {
		t.Errorf("join completed levels are %v, want %v: a new episode logs the line at info again", levels, want)
	}
}
