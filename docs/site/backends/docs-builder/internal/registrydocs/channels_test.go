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

package registrydocs

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/registry"
)

func TestChannelVersion(t *testing.T) {

	tests := []struct {
		name    string
		images  map[string][]entry
		want    string
		wantErr error
	}{
		{
			name:   "stable first",
			images: map[string][]entry{"src1/echo/release:stable": release("v1.2.0"), "src1/echo/release:alpha": release("v1.3.0")},
			want:   "v1.2.0",
		},
		{
			name:   "a channel pointing at a branch build is skipped",
			images: map[string][]entry{"src1/echo/release:stable": release("mr10"), "src1/echo/release:beta": release("v1.1.0")},
			want:   "v1.1.0",
		},
		{
			name:   "channels of the next source",
			images: map[string][]entry{"src2/echo/release:early-access": release("v2.0.0")},
			want:   "v2.0.0",
		},
		{
			name:   "a release image without version.json is skipped",
			images: map[string][]entry{"src1/echo/release:stable": {{name: "module.yaml", body: "name: echo"}}, "src1/echo/release:beta": release("v1.1.0")},
			want:   "v1.1.0",
		},
		{
			name:    "no channel tags",
			images:  map[string][]entry{"src1/echo/release:v1.0.0": release("v1.0.0")},
			wantErr: ondemand.ErrNoChannel,
		},
		{
			name:    "only branch builds in the channels",
			images:  map[string][]entry{"src1/echo/release:stable": release("0.0.1")},
			wantErr: ondemand.ErrNoChannel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry(t)
			for path, files := range tt.images {
				r.push(path, files...)
			}

			got, err := ChannelVersion(r.sources(), registry.NewClients())(context.Background(), "echo")
			if !errors.Is(err, tt.wantErr) || got != tt.want {
				t.Fatalf("version %q, err %v; want %q, %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestChannelVersionAsksHeadWhileTheChannelStays(t *testing.T) {
	r := newTestRegistry(t)
	r.push("src1/echo/release:stable", release("v1.0.0")...)
	r.takeRequests()
	channelVersion := ChannelVersion(r.sources(), registry.NewClients())

	pass := func(want string) []string {
		t.Helper()
		got, err := channelVersion(context.Background(), "echo")
		if err != nil || got != want {
			t.Fatalf("version %q, %v; want %q", got, err, want)
		}
		return r.takeRequests()
	}

	pass("v1.0.0")
	// The token, the /v2/ check and the manifest are kept: one HEAD.
	if got := pass("v1.0.0"); !reflect.DeepEqual(got, []string{"HEAD /v2/src1/echo/release/manifests/stable"}) {
		t.Fatalf("an unchanged channel made requests %q; want one HEAD", got)
	}

	r.push("src1/echo/release:stable", release("v1.1.0")...)
	r.takeRequests()
	moved := pass("v1.1.0")
	if !slices.ContainsFunc(moved, func(req string) bool { return strings.HasPrefix(req, "GET /v2/src1/echo/release/manifests/sha256:") }) {
		t.Fatalf("a moved channel made requests %q; want its image read by the digest", moved)
	}

	// A registry that answers HEAD without the digest is read by the tag.
	r.push("src1/echo/release:stable", release("v1.2.0")...)
	r.takeRequests()
	r.noHeadDigest.Store(true)
	if got := pass("v1.2.0"); !slices.Contains(got, "GET /v2/src1/echo/release/manifests/stable") {
		t.Fatalf("without the digest on HEAD the requests were %q; want the manifest by the tag", got)
	}
}

func TestChannelVersionOfAnIndexAsksHeadOnly(t *testing.T) {
	r := newTestRegistry(t)
	r.pushIndex("src1/echo/release:stable", release("v1.0.0")...)
	channelVersion := ChannelVersion(r.sources(), registry.NewClients())

	for range 2 {
		r.takeRequests()
		got, err := channelVersion(context.Background(), "echo")
		if err != nil || got != "v1.0.0" {
			t.Fatalf("version %q, %v; want v1.0.0", got, err)
		}
	}
	// HEAD gives the digest of the index, the image read is of the platform:
	// the index digest is what is compared on the next pass.
	if got := r.takeRequests(); !reflect.DeepEqual(got, []string{"HEAD /v2/src1/echo/release/manifests/stable"}) {
		t.Fatalf("the second pass made requests %q; want one HEAD", got)
	}
}

func TestChannelVersionImageGoneAfterHead(t *testing.T) {
	r := newTestRegistry(t)
	r.push("src1/echo/release:stable", release("v1.0.0")...)
	r.push("src1/echo/release:beta", release("v1.1.0")...)
	// The channel exists, its image does not: an error the module keeps its
	// version on, not a missing channel that moves it to the next one.
	r.goneByDigest.Store(true)
	got, err := ChannelVersion(r.sources(), registry.NewClients())(context.Background(), "echo")
	if err == nil || errors.Is(err, ondemand.ErrNoChannel) || got != "" {
		t.Fatalf("version %q, err %v; want a registry error", got, err)
	}
}

func TestChannels(t *testing.T) {
	release := func(version string) []entry {
		return []entry{{name: "version.json", body: `{"version": "` + version + `"}`}}
	}

	tests := []struct {
		name    string
		images  map[string][]entry
		want    ondemand.Channels
		wantErr error
	}{
		{
			name: "every channel of the site",
			images: map[string][]entry{
				"src1/echo/release:alpha": release("v1.3.0"), "src1/echo/release:beta": release("v1.3.0"),
				"src1/echo/release:early-access": release("v1.2.0"), "src1/echo/release:stable": release("v1.2.0"),
				"src1/echo/release:rock-solid": release("v1.1.0"),
			},
			want: ondemand.Channels{"alpha": "v1.3.0", "beta": "v1.3.0", "early-access": "v1.2.0", "stable": "v1.2.0", "rock-solid": "v1.1.0"},
		},
		{
			name:   "a channel pointing at a branch build is left out",
			images: map[string][]entry{"src1/echo/release:alpha": release("mr10"), "src1/echo/release:stable": release("v1.2.0")},
			want:   ondemand.Channels{"stable": "v1.2.0"},
		},
		{
			name: "the channels never mix two sources",
			images: map[string][]entry{
				"src1/echo/release:alpha": release("v9.0.0"),
				"src2/echo/release:alpha": release("v1.3.0"), "src2/echo/release:stable": release("v1.2.0"),
			},
			want: ondemand.Channels{"alpha": "v9.0.0"},
		},
		{
			name:   "the next source when the first has none",
			images: map[string][]entry{"src2/echo/release:stable": release("v2.0.0")},
			want:   ondemand.Channels{"stable": "v2.0.0"},
		},
		{
			name:    "no channel tags",
			images:  map[string][]entry{"src1/echo/release:v1.0.0": release("v1.0.0")},
			wantErr: ondemand.ErrNoChannel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry(t)
			for path, files := range tt.images {
				r.push(path, files...)
			}

			got, err := Channels(r.sources(), registry.NewClients())(context.Background(), "echo")
			if !errors.Is(err, tt.wantErr) || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("channels %v, err %v; want %v, %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestChannelsImageGoneAfterHead(t *testing.T) {
	r := newTestRegistry(t)
	r.push("src1/echo/release:stable", release("v1.0.0")...)
	r.goneByDigest.Store(true)

	// The channel is not dropped silently: the source fails.
	got, err := Channels(r.sources(), registry.NewClients())(context.Background(), "echo")
	if err == nil || errors.Is(err, ondemand.ErrNoChannel) || got != nil {
		t.Fatalf("channels %v, err %v; want a registry error", got, err)
	}
}

// Channel tags missing right after a lookup found them, as while the release
// repository is uploaded anew, are no module without channels yet: that
// lookup fails, the next one finds none. A failed lookup in between changes
// nothing.
func TestChannelsGoneAreNotNoneAtOnce(t *testing.T) {
	tests := []struct {
		name   string
		lookup func(r *testRegistry) func() error
	}{
		{
			name: "the version of a channel",
			lookup: func(r *testRegistry) func() error {
				return errOf(ChannelVersion(r.sources(), registry.NewClients()))
			},
		},
		{
			name: "the channels of the site",
			lookup: func(r *testRegistry) func() error {
				return errOf(Channels(r.sources(), registry.NewClients()))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry(t)
			r.push("src1/echo/release:stable", release("v1.0.0")...)
			lookup := tt.lookup(r)
			if err := lookup(); err != nil {
				t.Fatal(err)
			}

			r.failing.Store(&[]string{"/v2/src1/"})
			if err := lookup(); err == nil || errors.Is(err, ondemand.ErrNoChannel) || errors.Is(err, ondemand.ErrChannelsGone) {
				t.Fatalf("err %v; want a registry error", err)
			}
			r.failing.Store(nil)

			r.untag("src1/echo/release:stable")
			if err := lookup(); !errors.Is(err, ondemand.ErrChannelsGone) {
				t.Fatalf("err %v right after the channel was found, want %v", err, ondemand.ErrChannelsGone)
			}
			if err := lookup(); !errors.Is(err, ondemand.ErrNoChannel) {
				t.Fatalf("err %v on the next lookup, want %v", err, ondemand.ErrNoChannel)
			}
		})
	}
}

// A registry that took a request and sends nothing more, no answer to the
// HEAD of a channel or a blob stopping halfway, fails the lookup once its
// context is done: ondemand bounds a lookup so, and the module keeps its last
// target.
func TestChannelVersionGivesUpOnASilentRegistry(t *testing.T) {
	tests := []struct {
		name   string
		silent func(w http.ResponseWriter, req *http.Request) bool
	}{
		{
			name: "no answer to the HEAD of a channel",
			silent: func(_ http.ResponseWriter, req *http.Request) bool {
				return req.Method == http.MethodHead && strings.Contains(req.URL.Path, "/release/manifests/")
			},
		},
		{
			name: "a blob stops halfway",
			silent: func(w http.ResponseWriter, req *http.Request) bool {
				if req.Method != http.MethodGet || !strings.Contains(req.URL.Path, "/blobs/") {
					return false
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte{0x1f, 0x8b})
				w.(http.Flusher).Flush()
				return true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry(t)
			r.push("src1/echo/release:stable", release("v1.2.0")...)
			r.silent.Store(&tt.silent)
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()

			done := make(chan error, 1)
			go func() {
				_, err := ChannelVersion(r.sources(), registry.NewClients())(ctx, "echo")
				done <- err
			}()
			select {
			case err := <-done:
				if err == nil || errors.Is(err, ondemand.ErrNoChannel) || ondemand.Unpublished(err) {
					t.Fatalf("err %v; want the lookup failed", err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("the lookup still waits for the registry")
			}
		})
	}
}
