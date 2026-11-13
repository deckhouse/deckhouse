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

package s3cache

import (
	"bytes"
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeS3 is the part of S3 and STS the cache uses: objects in memory, keys
// handed out for one token.
type fakeS3 struct {
	t *testing.T

	mu      sync.Mutex
	objects map[string][]byte
	tokens  []string // tokens presented at STS
	stsPath string

	stsDown  bool   // STS answers 503
	noExpiry bool   // STS gives keys without an expiry
	revoked  string // requests signed with this key are refused

	s3Down         bool // S3 answers 503
	missingRefused bool // a missing object is refused, as without list rights
	ignoresPrefix  bool // List answers every object, whatever the prefix
	bare404        bool // every request answers 404 without an error code
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if strings.HasPrefix(r.URL.Path, "/sts/") {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.stsPath = r.URL.Path
		f.tokens = append(f.tokens, r.Form.Get("WebIdentityToken"))
		if f.stsDown {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		expiration := "<Expiration>" + time.Now().Add(30*time.Minute).UTC().Format(time.RFC3339) + "</Expiration>"
		if f.noExpiry {
			expiration = ""
		}
		fmt.Fprintf(w, `<AssumeRoleWithWebIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><AssumeRoleWithWebIdentityResult><Credentials>`+
			`<AccessKeyId>AKID%d</AccessKeyId><SecretAccessKey>SECRET</SecretAccessKey>%s`+
			`</Credentials></AssumeRoleWithWebIdentityResult></AssumeRoleWithWebIdentityResponse>`,
			len(f.tokens), expiration)
		return
	}

	auth := r.Header.Get("Authorization")
	if !strings.Contains(auth, "AKID") {
		http.Error(w, "unsigned", http.StatusForbidden)
		return
	}
	if f.revoked != "" && strings.Contains(auth, "Credential="+f.revoked+"/") {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `<Error><Code>AccessDenied</Code><Message>no such key</Message></Error>`)
		return
	}

	if f.s3Down {
		http.Error(w, "down", http.StatusServiceUnavailable)
		return
	}

	bucket, key, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if _, ok := f.objects[key]; f.missingRefused && !ok {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `<Error><Code>AccessDenied</Code><Message>access denied</Message></Error>`)
		return
	}
	if bucket != "d8-system-documentation" {
		http.Error(w, "bucket "+bucket, http.StatusNotFound)
		return
	}

	if f.bare404 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch {
	case r.Method == http.MethodGet && key == "" && r.URL.Query().Get("list-type") == "2":
		prefix := r.URL.Query().Get("prefix")
		if f.ignoresPrefix {
			prefix = ""
		}
		keys := make([]string, 0, len(f.objects))
		for k := range f.objects {
			if strings.HasPrefix(k, prefix) && k > r.URL.Query().Get("continuation-token") {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		// Two objects per page, to walk the pages.
		truncated := len(keys) > 2
		if truncated {
			keys = keys[:2]
		}
		var b strings.Builder
		fmt.Fprintf(&b, `<ListBucketResult><Name>d8-system-documentation</Name><IsTruncated>%t</IsTruncated>`, truncated)
		if truncated {
			fmt.Fprintf(&b, `<NextContinuationToken>%s</NextContinuationToken>`, keys[1])
		}
		for _, k := range keys {
			fmt.Fprintf(&b, `<Contents><Key>%s</Key><Size>%d</Size><LastModified>2026-11-02T12:00:00.000Z</LastModified></Contents>`, k, len(f.objects[k]))
		}
		b.WriteString(`</ListBucketResult>`)
		io.WriteString(w, b.String())
	case r.Method == http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		f.objects[key] = body
	case r.Method == http.MethodGet || r.Method == http.MethodHead:
		body, ok := f.objects[key]
		if !ok {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `<Error><Code>NoSuchKey</Code><Message>no such key</Message></Error>`)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.Write(body)
	case r.Method == http.MethodDelete:
		delete(f.objects, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "unexpected "+r.Method, http.StatusMethodNotAllowed)
	}
}

// startFakeS3 starts the fake over TLS and returns its URL and CA.
func startFakeS3(t *testing.T) (*fakeS3, string, string) {
	t.Helper()

	f := &fakeS3{t: t, objects: make(map[string][]byte)}
	srv := httptest.NewTLSServer(f)
	t.Cleanup(srv.Close)

	return f, srv.URL, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}))
}

// writeFiles writes the files of a mounted volume under dir; an empty value
// leaves the file out.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	for name, value := range files {
		if value == "" {
			continue
		}
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func mustNew(t *testing.T, dir, prefix string, onReady func(bool)) *Cache {
	t.Helper()

	c, err := New(dir, "d8-system", "documentation", prefix, onReady)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// newTestCache writes the files system-s3 would mount: the connection
// ConfigMap and the ServiceAccount token.
func newTestCache(t *testing.T) (*Cache, *fakeS3, string) {
	t.Helper()

	f, endpoint, ca := startFakeS3(t)
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"connection/S3_ENDPOINT":     endpoint,
		"connection/S3_BUCKET":       "d8-system-documentation",
		"connection/S3_REGION":       "garage",
		"connection/S3_CA_CERT":      ca,
		"connection/S3_STS_ENDPOINT": endpoint + "/sts",
		"connection/S3_STS_CA_CERT":  ca,
		"token/token":                "token-1",
	})

	return mustNew(t, dir, "", nil), f, dir
}

func TestCache(t *testing.T) {
	c, f, dir := newTestCache(t)
	ctx := context.Background()

	archive := filepath.Join(t.TempDir(), "v1.0.0.tar.gz")
	if err := os.WriteFile(archive, []byte("archive of echo v1.0.0"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.Put(ctx, "rev1/echo/v1.0.0.tar.gz", archive); err != nil {
		t.Fatal(err)
	}

	var got bytes.Buffer
	if err := c.Get(ctx, "rev1/echo/v1.0.0.tar.gz", &got); err != nil {
		t.Fatal(err)
	}
	if got.String() != "archive of echo v1.0.0" {
		t.Fatalf("got %q", got.String())
	}

	if err := c.Get(ctx, "rev1/echo/v9.9.9.tar.gz", io.Discard); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing object: err = %v, want ErrNotFound", err)
	}

	objects, err := c.List(ctx, "rev1/")
	if err != nil || len(objects) != 1 || objects[0].Key != "rev1/echo/v1.0.0.tar.gz" || objects[0].Size != 22 {
		t.Fatalf("list: %+v, %v", objects, err)
	}

	if err := c.Delete(ctx, "rev1/echo/v1.0.0.tar.gz"); err != nil {
		t.Fatal(err)
	}
	if objects, _ := c.List(ctx, "rev1/"); len(objects) != 0 {
		t.Fatalf("after delete: %+v", objects)
	}

	// The key came from STS for the bucket, in exchange for the mounted token.
	if f.stsPath != "/sts/d8-system/documentation" || len(f.tokens) != 1 || f.tokens[0] != "token-1" {
		t.Fatalf("STS: path %q, tokens %v", f.stsPath, f.tokens)
	}

	// The connection is read once: a changed one (system-s3 replaced its CA,
	// say) reaches the pod the checksum of the ConfigMap restarts, while this
	// one keeps its client and key.
	if err := os.WriteFile(filepath.Join(dir, "token", "token"), []byte("token-2"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "connection", "S3_REGION"), []byte("garage2"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.List(ctx, "rev1/"); err != nil || len(f.tokens) != 1 {
		t.Fatalf("List = %v, tokens %v: want the kept client and key", err, f.tokens)
	}
	if _, err := mustNew(t, dir, "", nil).List(ctx, "rev1/"); err != nil {
		t.Fatal(err)
	}
	if len(f.tokens) != 2 || f.tokens[1] != "token-2" {
		t.Fatalf("a restarted pod reuses the old connection: tokens %v", f.tokens)
	}
}

func TestCacheNotReady(t *testing.T) {
	var states []bool
	c := mustNew(t, t.TempDir(), "", func(ready bool) { states = append(states, ready) })

	if err := c.Get(context.Background(), "rev1/echo/v1.0.0.tar.gz", io.Discard); !errors.Is(err, ErrNotReady) {
		t.Fatalf("err = %v, want ErrNotReady before the ConfigMap is mounted", err)
	}
	c.Get(context.Background(), "rev1/echo/v1.0.0.tar.gz", io.Discard)
	if len(states) != 1 || states[0] {
		t.Fatalf("ready states %v, want one false", states)
	}
}

func TestCacheListPagesAndKeyRefresh(t *testing.T) {
	c, f, _ := newTestCache(t)
	ctx := context.Background()
	now := time.Now()
	c.now = func() time.Time { return now }

	archive := filepath.Join(t.TempDir(), "a.tar.gz")
	if err := os.WriteFile(archive, []byte("a"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"rev1/a/v1.tar.gz", "rev1/b/v1.tar.gz", "rev1/c/v1.tar.gz", "rev1/d/v1.tar.gz", "rev1/e/v1.tar.gz"} {
		if err := c.Put(ctx, k, archive); err != nil {
			t.Fatal(err)
		}
	}

	objects, err := c.List(ctx, "rev1/")
	if err != nil || len(objects) != 5 {
		t.Fatalf("list over pages: %d objects, %v", len(objects), err)
	}
	if len(f.tokens) != 1 {
		t.Fatalf("STS asked %d times, want 1: the key is reused", len(f.tokens))
	}

	// A minute before the key expires a new one is asked for (one request:
	// the fake STS counts the expiry from the real clock).
	now = now.Add(29*time.Minute + time.Second)
	if err := c.Get(ctx, "rev1/a/v1.tar.gz", io.Discard); err != nil {
		t.Fatal(err)
	}
	if len(f.tokens) != 2 {
		t.Fatalf("STS asked %d times, want 2 after the key nearly expired", len(f.tokens))
	}
}

func TestCacheReplacesRevokedKey(t *testing.T) {
	c, f, _ := newTestCache(t)
	ctx := context.Background()
	now := time.Now()
	c.now = func() time.Time { return now }

	if err := c.Get(ctx, "rev1/a/v1.tar.gz", io.Discard); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}

	// system-s3 revoked a key in use for a while: a new one is asked for
	// and the request goes through.
	now = now.Add(2 * time.Minute)
	f.mu.Lock()
	f.revoked = "AKID1"
	f.mu.Unlock()
	if err := c.Get(ctx, "rev1/a/v1.tar.gz", io.Discard); !errors.Is(err, ErrNotFound) {
		t.Fatalf("with a revoked key: %v", err)
	}

	// A fresh key that is refused means no access: STS is not asked again.
	f.mu.Lock()
	f.revoked = "AKID2"
	f.mu.Unlock()
	if err := c.Get(ctx, "rev1/a/v1.tar.gz", io.Discard); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("with a refused fresh key: %v", err)
	}
	if len(f.tokens) != 2 {
		t.Fatalf("STS asked %d times, want 2", len(f.tokens))
	}
}

func TestCacheSTSDown(t *testing.T) {
	c, f, dir := newTestCache(t)
	var states []bool
	c.onReady = func(ready bool) { states = append(states, ready) }
	ctx := context.Background()
	now := time.Now()
	c.now = func() time.Time { return now }

	f.stsDown = true
	for range 2 {
		if err := c.Get(ctx, "rev1/a/v1.tar.gz", io.Discard); err == nil || errors.Is(err, ErrNotFound) {
			t.Fatalf("with STS down: %v", err)
		}
	}
	if len(f.tokens) != 1 {
		t.Fatalf("STS asked %d times, want 1: a failure is answered for a while", len(f.tokens))
	}

	now = now.Add(stsBackoff + time.Second)
	f.mu.Lock()
	f.stsDown = false
	f.mu.Unlock()
	if err := c.Get(ctx, "rev1/a/v1.tar.gz", io.Discard); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after STS is back: %v", err)
	}
	if len(states) != 2 || states[0] || !states[1] {
		t.Fatalf("ready states %v, want not ready, then ready", states)
	}

	// The ServiceAccount token is never sent over plain HTTP.
	if err := os.WriteFile(filepath.Join(dir, "connection", "S3_STS_ENDPOINT"), []byte("http://sts.example"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := mustNew(t, dir, "", nil).Get(ctx, "rev1/a/v1.tar.gz", io.Discard); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("plain STS endpoint: %v", err)
	}
}

func TestCacheKeyWithoutExpiry(t *testing.T) {
	c, f, _ := newTestCache(t)
	f.noExpiry = true

	for range 3 {
		if err := c.Get(context.Background(), "rev1/a/v1.tar.gz", io.Discard); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
	}
	if len(f.tokens) != 1 {
		t.Fatalf("STS asked %d times, want 1", len(f.tokens))
	}
}

func TestCacheKeepsKeyWhenRefusalsAreNotAboutIt(t *testing.T) {
	c, f, _ := newTestCache(t)
	ctx := context.Background()
	now := time.Now()
	c.now = func() time.Time { return now }
	f.missingRefused = true

	// The fresh key is refused at once, so the refusals are about the
	// object, not the key: it is not replaced on the next misses.
	for range 5 {
		now = now.Add(2 * time.Minute)
		if err := c.Get(ctx, "rev1/a/v1.tar.gz", io.Discard); err == nil {
			t.Fatal("a refused request succeeded")
		}
	}
	if len(f.tokens) != 1 {
		t.Fatalf("STS asked %d times, want 1: a key per refused miss would exhaust STS", len(f.tokens))
	}
}

func TestCacheNotReadyWhenS3Fails(t *testing.T) {
	c, f, _ := newTestCache(t)
	var states []bool
	c.onReady = func(ready bool) { states = append(states, ready) }
	ctx := context.Background()

	if err := c.Get(ctx, "rev1/a/v1.tar.gz", io.Discard); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.s3Down = true
	f.mu.Unlock()
	if err := c.Get(ctx, "rev1/a/v1.tar.gz", io.Discard); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("with S3 down: %v", err)
	}
	f.mu.Lock()
	f.s3Down = false
	f.mu.Unlock()
	if err := c.Get(ctx, "rev1/a/v1.tar.gz", io.Discard); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}

	// The key is cached, so only S3 itself tells it went away and came back.
	if want := []bool{true, false, true}; !slices.Equal(states, want) {
		t.Fatalf("ready states %v, want %v", states, want)
	}
}

// newStaticCache writes the files of a Secret of the user: the connection and
// static keys, no STS and no token. files override them; an empty value
// leaves a key out.
func newStaticCache(t *testing.T, prefix string, files map[string]string) (*Cache, *fakeS3) {
	t.Helper()

	f, endpoint, ca := startFakeS3(t)
	all := map[string]string{
		"S3_ENDPOINT":           endpoint,
		"S3_BUCKET":             "d8-system-documentation",
		"S3_REGION":             "us-east-1",
		"S3_CA_CERT":            ca,
		"AWS_ACCESS_KEY_ID":     "AKIDSTATIC",
		"AWS_SECRET_ACCESS_KEY": "SECRET",
	}
	maps.Copy(all, files)
	connection := make(map[string]string, len(all))
	for name, value := range all {
		connection[filepath.Join("connection", name)] = value
	}
	dir := t.TempDir()
	writeFiles(t, dir, connection)
	// The volume of a missing Secret is an empty directory.
	if err := os.MkdirAll(filepath.Join(dir, "connection"), 0700); err != nil {
		t.Fatal(err)
	}

	return mustNew(t, dir, prefix, nil), f
}

func TestCacheStaticKeys(t *testing.T) {
	c, f := newStaticCache(t, "", nil)
	ctx := context.Background()

	archive := filepath.Join(t.TempDir(), "v1.0.0.tar.gz")
	if err := os.WriteFile(archive, []byte("archive"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.Put(ctx, "rev1/echo/v1.0.0.tar.gz", archive); err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := c.Get(ctx, "rev1/echo/v1.0.0.tar.gz", &got); err != nil || got.String() != "archive" {
		t.Fatalf("Get = %q, %v", got.String(), err)
	}
	if len(f.tokens) != 0 {
		t.Fatalf("STS was asked %d times with static keys", len(f.tokens))
	}

	// A refused static key is not asked for again: there is no other one.
	f.revoked = "AKIDSTATIC"
	if err := c.Get(ctx, "rev1/echo/v1.0.0.tar.gz", io.Discard); err == nil {
		t.Fatal("a refused key gave the object")
	}

	// A pair rotated in the Secret restarts the pod, which signs with it.
	writeFiles(t, c.dir, map[string]string{"connection/AWS_ACCESS_KEY_ID": "AKIDROTATED", "connection/AWS_SECRET_ACCESS_KEY": "SECRET2"})
	if err := mustNew(t, c.dir, "", nil).Get(ctx, "rev1/echo/v1.0.0.tar.gz", io.Discard); err != nil {
		t.Fatalf("Get with the rotated pair: %v", err)
	}

	// A deleted Secret leaves an empty volume: a restarted pod is not ready,
	// as before it was made.
	var states []bool
	if err := os.RemoveAll(filepath.Join(c.dir, "connection")); err != nil {
		t.Fatal(err)
	}
	restarted := mustNew(t, c.dir, "", func(ready bool) { states = append(states, ready) })
	if err := restarted.Get(ctx, "rev1/echo/v1.0.0.tar.gz", io.Discard); !errors.Is(err, ErrNotReady) {
		t.Fatalf("err = %v, want ErrNotReady without the Secret", err)
	}
	if !slices.Equal(states, []bool{false}) {
		t.Fatalf("ready states %v, want false", states)
	}
}

func TestCacheReadsConnectionOnce(t *testing.T) {
	c, _ := newStaticCache(t, "", nil)
	ctx := context.Background()
	if err := c.Get(ctx, "echo/v1.0.0.tar.gz", io.Discard); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound from the storage", err)
	}
	client := c.client

	// Files are not read again: the same client reaches the storage.
	if err := os.RemoveAll(filepath.Join(c.dir, "connection")); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(ctx, "echo/v1.0.0.tar.gz", io.Discard); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound with the kept connection", err)
	}
	if c.client != client {
		t.Fatal("the client is made again")
	}
}

func TestCacheConnectsOnce(t *testing.T) {
	c, f, _ := newTestCache(t)
	ctx := context.Background()

	// The first requests come together: one client, one key from STS.
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Get(ctx, "rev1/a/v1.tar.gz", io.Discard)
		}()
	}
	wg.Wait()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.tokens) != 1 {
		t.Fatalf("STS asked %d times, want 1", len(f.tokens))
	}
}

func TestCacheConnectionMistakes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"no bucket", map[string]string{"S3_BUCKET": ""}, "has no S3_BUCKET"},
		{"empty values", map[string]string{"S3_ENDPOINT": " ", "S3_BUCKET": " ", "S3_REGION": " ", "S3_CA_CERT": " ", "AWS_ACCESS_KEY_ID": " ", "AWS_SECRET_ACCESS_KEY": " "}, "empty S3_ENDPOINT"},
		{"half a key pair", map[string]string{"AWS_SECRET_ACCESS_KEY": ""}, "only one of"},
		{"neither keys nor STS", map[string]string{"AWS_ACCESS_KEY_ID": "", "AWS_SECRET_ACCESS_KEY": ""}, "neither"},
		{"plain http", map[string]string{"S3_ENDPOINT": "http://s3.example.com"}, "https URL is expected"},
		{"empty region", map[string]string{"S3_REGION": " "}, "empty S3_REGION"},
		{"keys named otherwise", map[string]string{"S3_ENDPOINT": "", "S3_BUCKET": "", "S3_REGION": "", "S3_CA_CERT": "", "AWS_ACCESS_KEY_ID": "", "AWS_SECRET_ACCESS_KEY": "", "endpoint": "https://s3.example.com", "..data": "kubelet"}, "only endpoint"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newStaticCache(t, "", tc.files)
			var states []bool
			c.onReady = func(ready bool) { states = append(states, ready) }
			err := c.Get(context.Background(), "rev1/echo/v1.0.0.tar.gz", io.Discard)
			if err == nil || errors.Is(err, ErrNotReady) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want one with %q, not ErrNotReady", err, tc.want)
			}
			// The docs promise the metric of readiness at 0.
			if !slices.Equal(states, []bool{false}) {
				t.Fatalf("ready states %v, want false", states)
			}
		})
	}
}

func TestCacheStaticKeysWin(t *testing.T) {
	// A Secret copied from the ConfigMap of system-s3, with keys added.
	c, f := newStaticCache(t, "", map[string]string{"S3_STS_ENDPOINT": "https://sts.example.com"})
	if err := c.Get(context.Background(), "rev1/echo/v1.0.0.tar.gz", io.Discard); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if len(f.tokens) != 0 {
		t.Fatalf("STS was asked %d times with static keys", len(f.tokens))
	}
}

func TestCacheWithoutCA(t *testing.T) {
	// The storage is checked against the system CAs, which do not know the
	// fake: a TLS error, not a refused CA.
	c, _ := newStaticCache(t, "", map[string]string{"S3_CA_CERT": ""})
	err := c.Get(context.Background(), "rev1/echo/v1.0.0.tar.gz", io.Discard)
	if err == nil || strings.Contains(err.Error(), "not PEM") || errors.Is(err, ErrNotReady) {
		t.Fatalf("err = %v, want a TLS error", err)
	}
}

func TestCacheBare404(t *testing.T) {
	c, f := newStaticCache(t, "", nil)
	f.bare404 = true
	archive := filepath.Join(t.TempDir(), "v1.0.0.tar.gz")
	if err := os.WriteFile(archive, []byte("archive"), 0600); err != nil {
		t.Fatal(err)
	}

	// A wrong endpoint behind a proxy answers 404 to everything: an upload
	// that went nowhere is an error, not a missing object.
	var states []bool
	c.onReady = func(ready bool) { states = append(states, ready) }
	// The miss of a reader before the upload proves nothing either way.
	if err := c.Get(context.Background(), "rev1/echo/v1.0.0.tar.gz", io.Discard); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get = %v, want ErrNotFound", err)
	}
	if err := c.Put(context.Background(), "rev1/echo/v1.0.0.tar.gz", archive); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("Put = %v, want an error other than ErrNotFound", err)
	}
	if !slices.Equal(states, []bool{false}) {
		t.Fatalf("ready states %v, want false", states)
	}
}

func TestCacheRefusedMissingObjectKeepsReadiness(t *testing.T) {
	c, f := newStaticCache(t, "", nil)
	f.missingRefused = true
	var states []bool
	c.onReady = func(ready bool) { states = append(states, ready) }

	if err := c.Get(context.Background(), "rev1/echo/v1.0.0.tar.gz", io.Discard); err == nil {
		t.Fatal("a refused object was given")
	}
	if len(states) != 0 {
		t.Fatalf("ready states %v, want none: the storage answered", states)
	}
}

func TestCacheKeepsToPrefix(t *testing.T) {
	c, f := newStaticCache(t, "documentation/", nil)
	ctx := context.Background()
	// Objects of other writers of a shared bucket, two of them sharing the
	// beginning of the prefix.
	foreign := []string{"deckhouse-prod/v1/docs-dkp/index.html", "documentation-old/rev0/echo/v1.tar.gz", "rev1/echo/v1.0.0.tar.gz"}
	for _, k := range foreign {
		f.objects[k] = []byte("foreign")
	}
	// Under the prefix but with an empty folder: still counted and evicted.
	f.objects["documentation/rev0//odd"] = []byte("odd")

	archive := filepath.Join(t.TempDir(), "v1.0.0.tar.gz")
	if err := os.WriteFile(archive, []byte("own"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"rev1/echo/v1.0.0.tar.gz", "rev2/targets.json"} {
		if err := c.Put(ctx, k, archive); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := f.objects["documentation/rev1/echo/v1.0.0.tar.gz"]; !ok {
		t.Fatalf("objects %v, want the archive under the prefix", slices.Sorted(maps.Keys(f.objects)))
	}

	// The eviction lists everything and may delete everything it lists.
	objects, err := c.List(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, o := range objects {
		keys = append(keys, o.Key)
		if err := c.Delete(ctx, o.Key); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(keys, []string{"rev0//odd", "rev1/echo/v1.0.0.tar.gz", "rev2/targets.json"}) {
		t.Fatalf("listed %v, want the own objects without the prefix", keys)
	}
	for _, k := range foreign {
		if string(f.objects[k]) != "foreign" {
			t.Fatalf("object %q of another writer is gone", k)
		}
	}
	if len(f.objects) != len(foreign) {
		t.Fatalf("objects %v, want only those of other writers left", slices.Sorted(maps.Keys(f.objects)))
	}

	// A key that could step out of the prefix is refused before any request.
	for _, k := range []string{"../deckhouse-prod/v1/docs-dkp/index.html", "rev1/../../deckhouse-prod/x", "..", ""} {
		if err := c.Delete(ctx, k); err == nil || errors.Is(err, ErrNotReady) {
			t.Fatalf("Delete(%q) = %v, want a refusal", k, err)
		}
		if err := c.Get(ctx, k, io.Discard); err == nil {
			t.Fatalf("Get(%q) passed", k)
		}
	}
	if _, err := c.List(ctx, "../"); err == nil {
		t.Fatal("List(../) passed")
	}
	if string(f.objects[foreign[0]]) != "foreign" {
		t.Fatal("an object of another writer is gone")
	}
}

func TestCacheListRefusesStorageIgnoringPrefix(t *testing.T) {
	c, f := newStaticCache(t, "documentation/", nil)
	f.ignoresPrefix = true
	f.objects["deckhouse-prod/v1/docs-dkp/index.html"] = []byte("foreign")
	f.objects["documentation/rev1/echo/v1.0.0.tar.gz"] = []byte("own")

	// The eviction would delete what it lists: no list at all.
	if objects, err := c.List(context.Background(), ""); err == nil {
		t.Fatalf("List = %v, want an error", objects)
	}
}

func TestCachePrefixValidation(t *testing.T) {
	for prefix, ok := range map[string]bool{
		"":                                 true,
		"documentation/":                   true,
		"deckhouse-prod/modules-ondemand/": true,
		"documentation":                    false,
		"/documentation/":                  false,
		"../documentation/":                false,
		"a//":                              false,
		"./":                               false,
	} {
		if _, err := New(t.TempDir(), "d8-system", "documentation", prefix, nil); (err == nil) != ok {
			t.Errorf("New with prefix %q = %v, want ok %t", prefix, err, ok)
		}
	}
}
