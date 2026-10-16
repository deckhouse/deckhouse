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

package fill

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readThrough fetches a body of size bytes through transport and says how long reading it to the
// end took.
func readThrough(t *testing.T, transport http.RoundTripper, size int) time.Duration {
	t.Helper()

	payload := bytes.Repeat([]byte{'x'}, size)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)

	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	started := time.Now()
	response, err := transport.RoundTrip(request)
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Len(t, body, size)
	return time.Since(started)
}

func TestThePacerBoundsTheCopy(t *testing.T) {
	const size = 2 << 20 // 2 MiB

	// The first burst is free, the rest arrives at 4 MiB/s: about half a second for 2 MiB.
	took := readThrough(t, NewPacer(4<<20).wrap(http.DefaultTransport), size)
	assert.GreaterOrEqual(t, took, 400*time.Millisecond, "a paced copy reads no faster than its rate")

	var none *Pacer
	assert.Less(t, readThrough(t, none.wrap(http.DefaultTransport), size), 300*time.Millisecond,
		"no pacer, no pacing")
}

func TestNoRateIsNoPacer(t *testing.T) {
	assert.Nil(t, NewPacer(0))
	assert.Nil(t, NewPacer(-1))
}
