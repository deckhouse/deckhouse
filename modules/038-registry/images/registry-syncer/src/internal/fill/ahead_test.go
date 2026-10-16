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
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setAndAhead discovers one reference as the set and another as ahead of it.
type setAndAhead struct{ set, ahead name.Reference }

func (d setAndAhead) Discover(context.Context, Registry, *remote.Puller) ([]name.Reference, error) {
	return []name.Reference{d.set}, nil
}

func (d setAndAhead) Optional(context.Context, Registry, *remote.Puller) ([]name.Reference, error) {
	return []name.Reference{d.ahead}, nil
}

// recordingTransport notes every path read through it.
type recordingTransport struct {
	mu    sync.Mutex
	paths []string
}

func (r *recordingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.paths = append(r.paths, request.URL.Path)
	r.mu.Unlock()
	return http.DefaultTransport.RoundTrip(request)
}

func (r *recordingTransport) read(repository string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, path := range r.paths {
		if strings.HasPrefix(path, "/v2/"+repository+"/") {
			return true
		}
	}
	return false
}

// TestTheSetIsNotReadThroughTheAheadSource is the harbor unit's cluster: everything the nodes pull
// goes to the upstream until the store holds the set, and a cold proxy cache upstream fails those
// pulls. Paced, the set took the pacer's time to arrive and the platform stood still meanwhile; only
// what is ahead of the set is background work.
func TestTheSetIsNotReadThroughTheAheadSource(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	pushImage(t, source, "platform:v1")
	pushImage(t, source, "pending:v2")

	set, err := name.ParseReference(source.Address+"/platform:v1", name.Insecure)
	require.NoError(t, err)
	ahead, err := name.ParseReference(source.Address+"/pending:v2", name.Insecure)
	require.NoError(t, err)

	paced := &recordingTransport{}
	aheadSource := source
	aheadSource.Options = []remote.Option{remote.WithTransport(paced)}

	report, err := (&Copier{
		Source:      source,
		Ahead:       &aheadSource,
		Destination: destination,
		Discover:    setAndAhead{set: set, ahead: ahead},
	}).Run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int32(1), report.Written, "the set was not copied")
	assert.Equal(t, int32(1), report.AheadCopied, "what is ahead of the set was not copied")
	assert.False(t, paced.read("platform"), "the set was read through the source meant for the copy ahead")
	assert.True(t, paced.read("pending"), "the copy ahead did not go through its own source")
}

// TestTheCopyAheadWaitsForItsTurn is the pass's half of keeping the copy ahead out of completeness: Run
// copies the set and stops, so that the caller can say the store is complete before a whole pending
// release is copied at the background pace; RunAhead then copies the rest.
func TestTheCopyAheadWaitsForItsTurn(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	pushImage(t, source, "platform:v1")
	pushImage(t, source, "pending:v2")

	set, err := name.ParseReference(source.Address+"/platform:v1", name.Insecure)
	require.NoError(t, err)
	ahead, err := name.ParseReference(source.Address+"/pending:v2", name.Insecure)
	require.NoError(t, err)

	copier := &Copier{
		Source:        source,
		Destination:   destination,
		Discover:      setAndAhead{set: set, ahead: ahead},
		SeparateAhead: true,
	}
	report, err := copier.Run(context.Background())
	require.NoError(t, err)
	assert.True(t, report.Complete(), "the set is all there is to completeness")
	assert.Zero(t, report.AheadCopied, "Run copied what is ahead of the set")
	_, err = remote.Head(mustTag(t, destination, "pending:v2"))
	assert.Error(t, err, "the pending image arrived before RunAhead")

	aheadReport := copier.RunAhead(context.Background())
	assert.Equal(t, int32(1), aheadReport.AheadCopied)
	_, err = remote.Head(mustTag(t, destination, "pending:v2"))
	assert.NoError(t, err, "RunAhead did not copy the pending image")
}

func TestRunAheadWithoutARunDoesNothing(t *testing.T) {
	report := (&Copier{SeparateAhead: true}).RunAhead(context.Background())
	assert.Zero(t, report.AheadCopied)
	assert.Empty(t, report.AheadMissed)
}

func mustTag(t *testing.T, target Registry, reference string) name.Tag {
	t.Helper()
	tag, err := name.NewTag(target.Address+"/"+reference, name.Insecure)
	require.NoError(t, err)
	return tag
}
