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

package registry

import (
	"context"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	crregistry "github.com/google/go-containerregistry/pkg/registry"
	v1remote "github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testLogger struct{}

func (testLogger) Errorf(string, ...interface{}) {}
func (testLogger) Infof(string, ...interface{})  {}
func (testLogger) Warnf(string, ...interface{})  {}
func (testLogger) Debugf(string, ...interface{}) {}
func (testLogger) Error(string, ...interface{})  {}

func newTestRegistry(t *testing.T) (host string) {
	t.Helper()
	srv := httptest.NewServer(crregistry.New())
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

func pushRandomImage(t *testing.T, host, repo, tag string) string {
	t.Helper()
	img, err := random.Image(256, 1)
	require.NoError(t, err)
	ref, err := name.NewTag(host+"/"+repo+":"+tag, name.WeakValidation)
	require.NoError(t, err)
	require.NoError(t, v1remote.Write(ref, img))
	d, err := img.Digest()
	require.NoError(t, err)
	return d.String()
}

func TestDefaultClient_ResolveTag(t *testing.T) {
	host := newTestRegistry(t)
	wantDigest := pushRandomImage(t, host, "deckhouse/deckhouse-cli", "v1.0.1")

	c := &DefaultClient{}
	cfg := &ClientConfig{
		Repository: host + "/deckhouse",
		Scheme:     "http",
	}

	got, err := c.ResolveTag(context.Background(), testLogger{}, cfg, "deckhouse-cli", "v1.0.1")
	require.NoError(t, err)
	assert.Equal(t, wantDigest, got)

	_, err = c.ResolveTag(context.Background(), testLogger{}, cfg, "deckhouse-cli", "missing-tag")
	require.ErrorIs(t, err, ErrPackageNotFound)
}

func TestDefaultClient_ListTags(t *testing.T) {
	host := newTestRegistry(t)
	pushRandomImage(t, host, "deckhouse/deckhouse-cli", "v1.0.0")
	pushRandomImage(t, host, "deckhouse/deckhouse-cli", "v1.0.1")
	pushRandomImage(t, host, "deckhouse/deckhouse-cli", "v1.1.0")

	c := &DefaultClient{}
	cfg := &ClientConfig{
		Repository: host + "/deckhouse",
		Scheme:     "http",
	}

	tags, err := c.ListTags(context.Background(), testLogger{}, cfg, "deckhouse-cli")
	require.NoError(t, err)
	sort.Strings(tags)
	assert.Equal(t, []string{"v1.0.0", "v1.0.1", "v1.1.0"}, tags)

	_, err = c.ListTags(context.Background(), testLogger{}, cfg, "unknown-image")
	require.ErrorIs(t, err, ErrPackageNotFound)
}
