package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"upmeter/pkg/check"
	"upmeter/pkg/upmeter/db"
)

func Test_DowntimeHandler(t *testing.T) {
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

			handler := &DowntimeHandler{
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

func (e *exporterMock) Export(string, []*check.DowntimeEpisode, int64) error {
	return nil
}
