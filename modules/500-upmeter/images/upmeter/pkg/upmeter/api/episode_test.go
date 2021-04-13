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

	"upmeter/pkg/check"
	"upmeter/pkg/upmeter/db"
)

func Test_AddEpisodesHandler(t *testing.T) {
	g := NewWithT(t)

	// setup database
	dbCtx, connErr := db.Connect("test-downtime-handler.db.sqlite")
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
