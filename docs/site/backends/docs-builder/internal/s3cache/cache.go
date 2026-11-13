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

// Package s3cache keeps archives of rendered module versions in an S3 bucket:
// the system S3 storage of the system-s3 module (a SystemBucket) or a bucket
// of the user.
//
// The connection comes from files mounted into the pod: the ConfigMap
// system-s3 writes for the SystemBucket, or the Secret of the user. Both
// appear later than the pod may start, so until then every call answers
// ErrNotReady. Once complete, the connection and its HTTP client are kept:
// a changed ConfigMap or Secret restarts the pod through a checksum in its
// template. A Secret holds static keys; without them short-lived keys are
// got through STS for the ServiceAccount token, as system-s3 issues them.
//
// Every object the cache reads, writes, lists or deletes has a key under its
// prefix, so a bucket shared with other writers keeps their objects.
//
// The client is written here instead of using minio-go or the AWS SDK: it
// needs four requests (get, put, list and delete an object), AWS Signature
// Version 4 and one STS exchange, which take a few hundred lines of the
// standard library. minio-go added about ten modules to go.mod (each to be
// scanned for CVEs) for 0.3 MB of the binary and a lot of API this cache
// never calls; the AWS SDK adds more.
package s3cache

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

var (
	// ErrNotReady means the connection of the bucket is not there yet.
	ErrNotReady = errors.New("the S3 bucket is not ready")
	// ErrNotFound means there is no object with the key.
	ErrNotFound = errors.New("no such object in S3")
)

// requestTimeout bounds one S3 request: a replica of the storage that went
// away is noticed only after about 75 seconds (system-s3 documentation).
const requestTimeout = 30 * time.Second

// Object is an object of the bucket.
type Object struct {
	Key          string
	Size         int64
	LastModified time.Time
}

// Cache is a bucket reached through the files in dir: dir/connection/<keys of
// the ConfigMap or the Secret> and, for STS, dir/token/token; namespace/name
// is the SystemBucket STS issues keys for.
type Cache struct {
	dir             string
	namespace, name string
	prefix          string
	now             func() time.Time
	onReady         func(ready bool)

	mu     sync.Mutex
	conn   connection
	client *http.Client
	creds  credentials

	// readyMu orders the changes of ready with their onReady calls.
	readyMu sync.Mutex
	ready   *bool
}

// New returns the cache of the bucket; prefix is put before every key and
// must keep the objects of the cache apart (see validPrefix). onReady, when
// set, is called each time the bucket becomes usable (STS gave a key, S3
// answered a request) or stops being usable (no connection, STS or S3
// failed), and on the first call that finds out which.
func New(dir, namespace, name, prefix string, onReady func(ready bool)) (*Cache, error) {
	if err := validPrefix(prefix); err != nil {
		return nil, err
	}
	return &Cache{dir: dir, namespace: namespace, name: name, prefix: prefix, now: time.Now, onReady: onReady}, nil
}

// validPrefix tells whether prefix can keep the objects of the cache apart:
// "" (a bucket of its own) or folders ending with "/", none of them empty,
// "." or "..", which a storage or a proxy might resolve.
func validPrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	folders, ok := strings.CutSuffix(prefix, "/")
	if !ok {
		return fmt.Errorf("S3 prefix %q: a prefix ending with \"/\" is expected", prefix)
	}
	for _, folder := range strings.Split(folders, "/") {
		if folder == "" || folder == "." || folder == ".." {
			return fmt.Errorf("S3 prefix %q: an empty, \".\" or \"..\" folder", prefix)
		}
	}
	return nil
}

// checkKey refuses a key that could reach outside of the prefix: one with a
// ".." segment a storage or a proxy might resolve. Empty and "." segments
// resolve within the prefix.
func checkKey(objectKey string) error {
	if objectKey == "" {
		return errors.New("an empty S3 key")
	}
	if slices.Contains(strings.Split(objectKey, "/"), "..") {
		return fmt.Errorf("S3 key %q may reach outside of the prefix", objectKey)
	}
	return nil
}

// object returns the key of the object in the bucket, under the prefix.
func (c *Cache) object(objectKey string) (string, error) {
	if err := checkKey(objectKey); err != nil {
		return "", err
	}
	return c.prefix + objectKey, nil
}

func (c *Cache) setReady(ready bool) {
	c.readyMu.Lock()
	defer c.readyMu.Unlock()

	if c.ready != nil && *c.ready == ready {
		return
	}
	c.ready = &ready
	if c.onReady != nil {
		c.onReady(ready)
	}
}

type connection struct {
	Endpoint, Bucket, Region, CA, STSEndpoint, STSCA string
	AccessKey, SecretKey                             string
}

// readConnection reads the files of the connection. An empty directory is a
// ConfigMap or a Secret not there yet (ErrNotReady). Any other mistake is
// named in the error: no known key at all, a missing or empty required one,
// half of a key pair, neither keys nor STS.
func (c *Cache) readConnection() (connection, error) {
	var conn connection
	files := []struct {
		key      string
		to       *string
		required bool
	}{
		{"S3_ENDPOINT", &conn.Endpoint, true},
		{"S3_BUCKET", &conn.Bucket, true},
		{"S3_REGION", &conn.Region, true},
		{"S3_CA_CERT", &conn.CA, false},
		{"S3_STS_ENDPOINT", &conn.STSEndpoint, false},
		{"S3_STS_CA_CERT", &conn.STSCA, false},
		{"AWS_ACCESS_KEY_ID", &conn.AccessKey, false},
		{"AWS_SECRET_ACCESS_KEY", &conn.SecretKey, false},
	}

	var missing []string
	found := 0
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(c.dir, "connection", f.key))
		if errors.Is(err, os.ErrNotExist) {
			if f.required {
				missing = append(missing, f.key)
			}
			continue
		}
		if err != nil {
			return connection{}, err
		}
		found++
		*f.to = strings.TrimSpace(string(b))
	}

	if found == 0 {
		return connection{}, c.noKnownKeys()
	}

	switch {
	case len(missing) > 0:
		return connection{}, fmt.Errorf("the S3 connection has no %s", strings.Join(missing, ", "))
	case conn.Endpoint == "" || conn.Bucket == "":
		return connection{}, errors.New("the S3 connection has an empty S3_ENDPOINT or S3_BUCKET")
	// SigV4 needs the region; the ConfigMap of system-s3 is left as it is.
	case conn.AccessKey != "" && conn.Region == "":
		return connection{}, errors.New("the S3 connection has an empty S3_REGION")
	case (conn.AccessKey == "") != (conn.SecretKey == ""):
		return connection{}, errors.New("the S3 connection has only one of AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY")
	case conn.AccessKey == "" && conn.STSEndpoint == "":
		return connection{}, errors.New("the S3 connection has neither AWS_ACCESS_KEY_ID nor S3_STS_ENDPOINT")
	}

	return conn, nil
}

// noKnownKeys tells a volume not filled yet (ErrNotReady) from a Secret whose
// keys are named otherwise. The names, not the values, go to the error; the
// "..data" entries are the bookkeeping of kubelet.
func (c *Cache) noKnownKeys() error {
	entries, err := os.ReadDir(filepath.Join(c.dir, "connection"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var names []string
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "..") {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return ErrNotReady
	}
	return fmt.Errorf("the S3 connection has none of S3_ENDPOINT, S3_BUCKET, S3_REGION, only %s", strings.Join(names, ", "))
}

// session returns the HTTP client and a key for the connection.
func (c *Cache) session(ctx context.Context) (connection, *http.Client, key, error) {
	c.mu.Lock()
	connected := c.client != nil
	c.mu.Unlock()
	if !connected {
		if err := c.connect(); err != nil {
			c.setReady(false)
			return connection{}, nil, key{}, err
		}
	}

	// Read after connect: a call that raced to connect uses what the first
	// one made.
	c.mu.Lock()
	conn, client, creds := c.conn, c.client, c.creds
	c.mu.Unlock()

	k, err := creds.get(ctx)
	if err != nil {
		return connection{}, nil, key{}, err
	}

	return conn, client, k, nil
}

// connect reads the connection and makes its client and credentials, which
// are kept from then on. Until the connection is complete every call reads
// it again: the ConfigMap or the Secret may appear after the pod starts.
func (c *Cache) connect() error {
	conn, err := c.readConnection()
	if err != nil {
		return err
	}
	client, err := newHTTPClient(conn)
	if err != nil {
		return err
	}
	creds := c.credentials(conn, client)

	c.mu.Lock()
	defer c.mu.Unlock()
	// Calls that raced to connect keep the client made first.
	if c.client == nil {
		c.conn, c.client, c.creds = conn, client, creds
	} else {
		client.CloseIdleConnections()
	}

	return nil
}

// credentials are the keys of the connection: the static ones of a Secret
// or, without them, those STS issues for the ServiceAccount token, as
// system-s3 does.
func (c *Cache) credentials(conn connection, client *http.Client) credentials {
	if conn.AccessKey != "" {
		return staticKeys{key{accessKey: conn.AccessKey, secretKey: conn.SecretKey}}
	}

	return &stsKeys{
		endpoint:  strings.TrimSuffix(conn.STSEndpoint, "/") + "/" + c.namespace + "/" + c.name,
		tokenPath: filepath.Join(c.dir, "token", "token"),
		client:    client,
		now:       c.now,
		ready:     c.setReady,
	}
}

// refused forgets a key S3 refused and reports whether the request is worth
// sending again (credentials).
func (c *Cache) refused(k key) bool {
	c.mu.Lock()
	creds := c.creds
	c.mu.Unlock()

	return creds.refused(k)
}

func newHTTPClient(conn connection) (*http.Client, error) {
	roots, err := x509.SystemCertPool()
	if err != nil {
		roots = x509.NewCertPool()
	}
	// Without a CA of its own the storage is trusted by the system ones.
	for _, ca := range []string{conn.CA, conn.STSCA} {
		if ca != "" && !roots.AppendCertsFromPEM([]byte(ca)) {
			return nil, errors.New("the CA of the S3 bucket or of STS is not PEM")
		}
	}

	// The keys and the ServiceAccount token go there, so both are https.
	// readConnection leaves only the STS endpoint empty, with static keys.
	for _, endpoint := range []string{conn.Endpoint, conn.STSEndpoint} {
		if endpoint == "" {
			continue
		}
		if u, err := url.Parse(endpoint); err != nil || u.Scheme != "https" || u.Host == "" {
			return nil, fmt.Errorf("S3 endpoint %q: an https URL is expected", endpoint)
		}
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig.RootCAs = roots

	return &http.Client{Transport: transport, Timeout: requestTimeout}, nil
}

// do sends a signed request to the bucket; key is the object key ("" for the
// bucket itself). The caller closes the body of a successful response.
func (c *Cache) do(ctx context.Context, method, objectKey string, query url.Values, body io.ReadSeeker, size int64, payload string) (*http.Response, error) {
	resp, k, status, err := c.send(ctx, method, objectKey, query, body, size, payload)
	if status == http.StatusForbidden && c.refused(k) {
		if body != nil {
			if _, err := body.Seek(0, io.SeekStart); err != nil {
				return nil, err
			}
		}
		resp, k, status, err = c.send(ctx, method, objectKey, query, body, size, payload)
		if status == http.StatusForbidden {
			c.refused(k)
		}
	}

	return resp, err
}

// send is one attempt of do; it also returns the key it signed with and the
// status of a refused request.
func (c *Cache) send(ctx context.Context, method, objectKey string, query url.Values, body io.ReadSeeker, size int64, payload string) (*http.Response, key, int, error) {
	conn, client, k, err := c.session(ctx)
	if err != nil {
		return nil, key{}, 0, err
	}

	u, err := url.Parse(strings.TrimSuffix(conn.Endpoint, "/"))
	if err != nil {
		return nil, k, 0, err
	}
	u.Path = "/" + conn.Bucket
	if objectKey != "" {
		u.Path += "/" + objectKey
	}
	u.RawPath = encodePath(u.Path)
	u.RawQuery = canonicalQuery(query)

	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, k, 0, err
	}
	if body != nil {
		req.Body = io.NopCloser(body)
		req.ContentLength = size
		req.Header.Set("Content-Type", "application/gzip")
	}
	sign(req, k.accessKey, k.secretKey, k.sessionToken, conn.Region, payload, c.now())

	resp, err := client.Do(req)
	if err != nil {
		// A reader that went away says nothing about the storage.
		if ctx.Err() == nil {
			c.setReady(false)
		}
		return nil, k, 0, err
	}
	if resp.StatusCode/100 == 2 {
		c.setReady(true)
		return resp, k, resp.StatusCode, nil
	}

	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	code := errorCode(b)
	if code == "NoSuchKey" {
		c.setReady(true)
		return nil, k, resp.StatusCode, fmt.Errorf("%w: %s", ErrNotFound, objectKey)
	}
	// A bare 404 for an object comes from a storage without error codes, or
	// from a proxy before a wrong endpoint: the object is missing, but the
	// storage is not proven usable, so readiness waits for a PUT or a List.
	if resp.StatusCode == http.StatusNotFound && objectKey != "" && code == "" && method != http.MethodPut {
		return nil, k, resp.StatusCode, fmt.Errorf("%w: %s", ErrNotFound, objectKey)
	}

	// A key without the right to list gets AccessDenied for a missing object
	// instead of NoSuchKey: the storage answered, so readiness stays as it
	// is, and the PUT that follows tells whether writing is allowed. Other
	// refusals (a bad signature, an expired key) are failures.
	deniedMissing := resp.StatusCode == http.StatusForbidden && code == "AccessDenied" && method == http.MethodGet && objectKey != ""
	if !deniedMissing {
		c.setReady(false)
	}
	return nil, k, resp.StatusCode, fmt.Errorf("S3 %s %s: %s %s", method, u.Path, resp.Status, code)
}

func errorCode(body []byte) string {
	var e struct {
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	}
	if xml.Unmarshal(body, &e) != nil || e.Code == "" {
		// STS wraps the error: <ErrorResponse><Error>...
		var wrapped struct {
			Error struct {
				Code string `xml:"Code"`
			} `xml:"Error"`
		}
		if xml.Unmarshal(body, &wrapped) == nil {
			return wrapped.Error.Code
		}
		return ""
	}
	return e.Code
}

// Get writes the object to w.
func (c *Cache) Get(ctx context.Context, objectKey string, w io.Writer) error {
	object, err := c.object(objectKey)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.do(ctx, http.MethodGet, object, nil, nil, 0, emptyPayload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(w, resp.Body)

	return err
}

// Put uploads the file at path as the object. The body is signed, so the
// file is read twice: for the hash and for the upload.
func (c *Cache) Put(ctx context.Context, objectKey, path string) error {
	object, err := c.object(objectKey)
	if err != nil {
		return err
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.do(ctx, http.MethodPut, object, nil, f, size, hex.EncodeToString(h.Sum(nil)))
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

// List returns the objects with the prefix (ListObjectsV2, page by page),
// with the keys Get and Delete take: without the prefix of the cache.
func (c *Cache) List(ctx context.Context, prefix string) ([]Object, error) {
	if prefix != "" {
		if err := checkKey(strings.TrimSuffix(prefix, "/")); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	var objects []Object
	token := ""
	for {
		query := url.Values{"list-type": {"2"}, "prefix": {c.prefix + prefix}}
		if token != "" {
			query.Set("continuation-token", token)
		}

		resp, err := c.do(ctx, http.MethodGet, "", query, nil, 0, emptyPayload)
		if err != nil {
			return nil, err
		}
		var page struct {
			Contents []struct {
				Key          string    `xml:"Key"`
				Size         int64     `xml:"Size"`
				LastModified time.Time `xml:"LastModified"`
			} `xml:"Contents"`
			IsTruncated           bool   `xml:"IsTruncated"`
			NextContinuationToken string `xml:"NextContinuationToken"`
		}
		err = xml.NewDecoder(resp.Body).Decode(&page)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("list objects: %w", err)
		}

		for _, o := range page.Contents {
			// A storage that ignores the prefix of the request would hand
			// the eviction objects of other writers: no list at all then.
			k, ok := strings.CutPrefix(o.Key, c.prefix)
			if !ok || !strings.HasPrefix(k, prefix) {
				return nil, fmt.Errorf("list objects: %q is outside of the prefix %q", o.Key, c.prefix+prefix)
			}
			// Delete refuses such a key, so it is neither counted nor
			// evicted; only a writer other than this cache puts one.
			if checkKey(k) != nil {
				continue
			}
			objects = append(objects, Object{Key: k, Size: o.Size, LastModified: o.LastModified})
		}
		if !page.IsTruncated || page.NextContinuationToken == "" {
			return objects, nil
		}
		token = page.NextContinuationToken
	}
}

// Delete removes the object; a missing one is not an error.
func (c *Cache) Delete(ctx context.Context, objectKey string) error {
	object, err := c.object(objectKey)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.do(ctx, http.MethodDelete, object, nil, nil, 0, emptyPayload)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}
