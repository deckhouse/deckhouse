/*
Copyright 2021 Flant JSC

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

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/db"
)

func Test_AddEpisodesHandler(t *testing.T) {
	g := NewWithT(t)

	// setup database
	dbCtx, connErr := db.Connect("test-downtime-handler.db.sqlite", nil)
	g.Expect(connErr).ShouldNot(HaveOccurred())
	g.Expect(dbCtx).ShouldNot(BeNil())

	var err error
	var rr *httptest.ResponseRecorder

	tests := []struct {
		name   string
		data   string
		expect func(t *testing.T)
	}{
		{
			"empty array is a success",
			`{"origin":"","episodes":[]}`,
			func(t *testing.T) {
				g := NewWithT(t)
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(rr.Code).Should(Equal(http.StatusOK), "handler returned wrong status code: got %v %v", rr.Code, rr.Body.String())
				g.Expect(rr.Body.String()).Should(Equal(`{}`))
				g.Expect(rr.Header().Get("Content-Type")).Should(Equal("application/json"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
			// pass 'nil' as the third parameter.
			req, err := http.NewRequest(http.MethodPost, "/downtime", strings.NewReader(tt.data))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")

			// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
			rr = httptest.NewRecorder()

			handler := &AddEpisodesHandler{
				DbCtx:       dbCtx,
				RemoteWrite: &exporterMock{},
			}

			// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
			// directly and pass in our Request and ResponseRecorder.
			handler.ServeHTTP(rr, req)

			tt.expect(t)
		})
	}
}

type exporterMock struct{}

func (e *exporterMock) Export(string, []*check.Episode, time.Duration) error {
	return nil
}

func Test_EpisodePayload(t *testing.T) {
	tests := []struct {
		name string
		want EpisodesPayload
	}{
		{
			name: "empty",
			want: EpisodesPayload{},
		}, {
			name: "only origin",
			want: EpisodesPayload{
				Origin: "booo",
			},
		}, {
			name: "with an episode",
			want: EpisodesPayload{
				Origin: "xxxx",
				Episodes: []check.Episode{
					{
						ProbeRef: check.ProbeRef{Group: "Grrr", Probe: "Prrr"},
						TimeSlot: time.Now().Add(-time.Hour).Truncate(30 * time.Second),
						Up:       27 * time.Second,
						Down:     300 * time.Millisecond,
						Unknown:  700 * time.Millisecond,
						NoData:   2 * time.Second,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshalled, err := json.Marshal(tt.want)
			if err != nil {
				t.Errorf("cannot marshal episode payload to JSON err=%v", err)
			}

			var got EpisodesPayload
			err = json.Unmarshal(marshalled, &got)
			if err != nil {
				t.Errorf("cannot unmarshal episode payload JSON err=%v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unmarshalled payload does match: want=%v, got=%v", tt.want, got)
			}
		})
	}
}

func Test_isLastSubSlot(t *testing.T) {
	tests := []struct {
		name string
		slot time.Time
		want bool
	}{
		{
			name: "ts=300s",
			slot: time.Unix(300, 0),
			want: false,
		},
		{
			name: "ts=330s",
			slot: time.Unix(330, 0),
			want: false,
		},
		{
			name: "ts=270s",
			slot: time.Unix(270, 0),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLastSubSlot(tt.slot); got != tt.want {
				t.Errorf("isLastSubSlot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_chooseByLatestSubSlot(t *testing.T) {
	g := NewWithT(t)

	ref := &check.ProbeRef{
		Group: "a", Probe: "b",
	}

	type aarg struct {
		ref   *check.ProbeRef
		slots []time.Time
	}
	type args struct {
		saved30s aarg
		saved5m  aarg
	}
	tests := []struct {
		name        string
		args        args
		wantByIndex []int
	}{
		{
			name:        "complete mismatch gives none",
			args:        args{},
			wantByIndex: []int{},
		},
		{
			name: "only probeRef match gives none",
			args: args{
				saved30s: aarg{ref: ref},
				saved5m:  aarg{ref: ref},
			},
			wantByIndex: []int{},
		},
		{
			name: "only 5m slot match gives none",
			args: args{
				saved30s: aarg{slots: slotsUnix(570)},
				saved5m:  aarg{slots: slotsUnix(300)},
			},
			wantByIndex: []int{},
		},
		{
			name: "ref and 5m slot match gives one out of ones",
			args: args{
				saved30s: aarg{ref: ref, slots: slotsUnix(570)},
				saved5m:  aarg{ref: ref, slots: slotsUnix(300)},
			},
			wantByIndex: []int{0},
		},
		{
			name: "ref and 5m slot match gives one out of three",
			args: args{
				saved30s: aarg{ref: ref, slots: slotsUnix(270, 570, 870)},
				saved5m:  aarg{ref: ref, slots: slotsUnix(600)},
			},
			wantByIndex: []int{0},
		},
		{
			name: "ref and 5m slot match gives three out of three",
			args: args{
				saved30s: aarg{ref: ref, slots: slotsUnix(570, 870, 1170)},
				saved5m:  aarg{ref: ref, slots: slotsUnix(0, 300, 600, 900, 1200)},
			},
			wantByIndex: []int{1, 2, 3},
		},
		{
			name: "ref and 5m slot match gives one out of three, when two subslots are not the latest ones",
			args: args{
				saved30s: aarg{ref: ref, slots: slotsUnix(240, 570, 810)},
				saved5m:  aarg{ref: ref, slots: slotsUnix(300, 600, 900)},
			},
			wantByIndex: []int{0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e30s := episodes(tt.args.saved30s.ref, tt.args.saved30s.slots)
			e5m := episodes(tt.args.saved5m.ref, tt.args.saved5m.slots)

			got := chooseByLatestSubSlot(e30s, e5m)

			want := make([]*check.Episode, 0)
			for _, i := range tt.wantByIndex {
				want = append(want, e5m[i])
			}

			g.Expect(got).To(Equal(want))
		})
	}
}

func slotsUnix(tss ...int64) []time.Time {
	slots := make([]time.Time, 0, len(tss))
	for _, ts := range tss {
		slots = append(slots, time.Unix(ts, 0))
	}
	return slots
}

func episodes(ref *check.ProbeRef, slots []time.Time) []*check.Episode {
	fixtures := check.RandomEpisodes(len(slots))
	var eps []*check.Episode
	for i := range fixtures {
		ep := &fixtures[i]
		if ref != nil {
			ep.ProbeRef = *ref
		}
		ep.TimeSlot = slots[i]
		eps = append(eps, ep)
	}
	return eps
}
