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
	"archive/tar"
	"cmp"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/docs-builder/internal/metrics"
)

// Client reads module images from the registry of one source.
type Client struct {
	repo     string
	nameOpts []name.Option
	opts     []remote.Option
	// fallback, when set, is asked where this registry fails (failed).
	fallback *Client
}

func NewClient(src Source) (*Client, error) {
	return newClient(src, nil, responseHeaderTimeout)
}

// newClient makes the client of the source; with ms its requests to the
// registry are counted. A registry that does not start answering a request
// within headerTimeout fails it (responseHeaderTimeout).
func newClient(src Source, ms *metricsstorage.MetricStorage, headerTimeout time.Duration) (*Client, error) {
	var nameOpts []name.Option
	if strings.EqualFold(src.Scheme, "http") {
		nameOpts = append(nameOpts, name.Insecure)
	}

	repo := strings.TrimSuffix(src.Repo, "/")

	auth, err := authenticator(repo, src)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	transport, err := transportWithCA(src.CA, headerTimeout)
	if err != nil {
		return nil, fmt.Errorf("ca: %w", err)
	}
	if ms != nil {
		transport = countingTransport{next: transport, ms: ms}
	}

	// Without the puller every call checks /v2/ and gets a token anew: three
	// requests for one. The puller keeps the token of every repository and
	// gets a new one when the registry refuses it (registry.deckhouse.io
	// issues them for 15 minutes).
	puller, err := remote.NewPuller(remote.WithAuth(auth), remote.WithTransport(transport))
	if err != nil {
		return nil, fmt.Errorf("puller: %w", err)
	}

	client := &Client{
		repo:     repo,
		nameOpts: nameOpts,
		opts:     []remote.Option{remote.Reuse(puller)},
	}
	if src.Fallback == nil {
		return client, nil
	}

	client.fallback, err = newClient(*src.Fallback, ms, headerTimeout)
	if err != nil {
		return nil, fmt.Errorf("fallback %s: %w", src.Fallback.Name, err)
	}

	return client, nil
}

// ListTags returns the tags of the module release repository: the published
// versions and the release channels.
func (c *Client) ListTags(ctx context.Context, module string) ([]string, error) {
	return ask(ctx, c, func(r *Client) ([]string, error) {
		return r.list(ctx, r.repo+"/"+module+"/release")
	})
}

// ListModules returns the tags of the repository itself: in a module
// registry they are the names of the modules.
func (c *Client) ListModules(ctx context.Context) ([]string, error) {
	return ask(ctx, c, func(r *Client) ([]string, error) {
		return r.list(ctx, r.repo)
	})
}

func (c *Client) list(ctx context.Context, repository string) ([]string, error) {
	repo, err := name.NewRepository(repository, c.nameOpts...)
	if err != nil {
		return nil, fmt.Errorf("parse repository: %w", err)
	}

	tags, err := remote.List(repo, c.withContext(ctx)...)
	if err != nil {
		return nil, fmt.Errorf("list tags of %s: %w", repo, err)
	}

	return tags, nil
}

// Image returns the full module image of the given version.
func (c *Client) Image(ctx context.Context, module, tag string) (v1.Image, error) {
	return ask(ctx, c, func(r *Client) (v1.Image, error) {
		return r.image(ctx, r.repo+"/"+module+":"+tag)
	})
}

// ReleaseImage returns the release image of the given version: the module
// metadata and, in recent module builds, its documentation, without the
// templates and charts of the full image.
func (c *Client) ReleaseImage(ctx context.Context, module, tag string) (v1.Image, error) {
	return ask(ctx, c, func(r *Client) (v1.Image, error) {
		return r.image(ctx, r.repo+"/"+module+"/release:"+tag)
	})
}

// ReleaseDigest returns the digest of the release image a tag points to, e.g.
// a release channel, with one HEAD request: the manifest is not downloaded.
// A registry that does not answer HEAD as the API asks fails with
// ErrHeadUnsupported.
func (c *Client) ReleaseDigest(ctx context.Context, module, tag string) (string, error) {
	digest, err := ask(ctx, c, func(r *Client) (string, error) {
		ref, err := name.ParseReference(r.repo+"/"+module+"/release:"+tag, r.nameOpts...)
		if err != nil {
			return "", fmt.Errorf("parse reference: %w", err)
		}

		desc, err := remote.Head(ref, r.withContext(ctx)...)
		if err != nil {
			return "", fmt.Errorf("head image %s: %w", ref, err)
		}

		return desc.Digest.String(), nil
	})
	if !errors.Is(err, ErrNotFound) && headUnsupported(ctx, err) {
		return "", fmt.Errorf("%w: %w", ErrHeadUnsupported, err)
	}

	return digest, err
}

// ReleaseImageAt returns the release image with the digest: the one a tag
// pointed to when ReleaseDigest was asked, even if the tag has moved since.
func (c *Client) ReleaseImageAt(ctx context.Context, module, digest string) (v1.Image, error) {
	return ask(ctx, c, func(r *Client) (v1.Image, error) {
		return r.image(ctx, r.repo+"/"+module+"/release@"+digest)
	})
}

// ask reads from the registry of c, or from its fallback (withFallback), and
// marks an answer that the repository, the tag or the image is not there
// with ErrNotFound.
func ask[T any](ctx context.Context, c *Client, read func(*Client) (T, error)) (T, error) {
	v, err := withFallback(ctx, c, read)

	return v, notFound(err)
}

// withFallback reads from the registry of c and, where it fails, from the
// fallback. The answer is the one of the last registry asked: the error of
// the fallback comes first, so a "not found" there is the answer.
func withFallback[T any](ctx context.Context, c *Client, read func(*Client) (T, error)) (T, error) {
	v, err := read(c)
	if c.fallback == nil || !failed(ctx, err) {
		return v, err
	}

	v, ferr := withFallback(ctx, c.fallback, read)
	if ferr != nil {
		return v, fmt.Errorf("%w; asked because %w", ferr, err)
	}

	return v, nil
}

// failed tells a registry that gave no answer, so its fallback is asked: it
// was not reached, failed itself (5xx) or refused the credentials (401, 403).
// The node agent switches on the same failures
// (modules/038-registry/images/registry-agent/src/internal/proxy/server.go).
// An answer, "not found" too, is final, and so is a request the caller gave
// up: it says nothing of the registry.
func failed(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}

	var terr *transport.Error
	if errors.As(err, &terr) {
		return terr.StatusCode >= http.StatusInternalServerError ||
			terr.StatusCode == http.StatusUnauthorized || terr.StatusCode == http.StatusForbidden
	}
	var uerr *url.Error

	return errors.As(err, &uerr)
}

// ErrNotFound means the registry answered that the repository, the tag, the
// image or a layer of it is not there, as opposed to a registry that could
// not be asked.
var ErrNotFound = errors.New("not found in the registry")

// ErrHeadUnsupported means the registry answered HEAD, but not as the API
// asks: no Docker-Content-Digest, or no HEAD at all. The image is read by
// the tag instead.
var ErrHeadUnsupported = errors.New("the registry does not answer HEAD with the digest")

// notFound marks err with ErrNotFound when it is the answer that something is
// not there (isNotFound).
func notFound(err error) error {
	if !isNotFound(err) {
		return err
	}

	return fmt.Errorf("%w: %w", ErrNotFound, err)
}

// isNotFound tells a repository, a tag or an image that is not there apart
// from a registry that could not be asked.
func isNotFound(err error) bool {
	var terr *transport.Error
	if !errors.As(err, &terr) {
		return false
	}

	if terr.StatusCode == http.StatusNotFound {
		return true
	}

	for _, d := range terr.Errors {
		if d.Code == transport.ManifestUnknownErrorCode || d.Code == transport.NameUnknownErrorCode {
			return true
		}
	}

	return false
}

// headUnsupported tells a registry that answered HEAD but not as the API
// asks (no Docker-Content-Digest, or no HEAD at all) from one that did not
// answer, or a request given up: only the first is read by the tag.
func headUnsupported(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}
	var terr *transport.Error
	if errors.As(err, &terr) {
		return terr.StatusCode == http.StatusMethodNotAllowed || terr.StatusCode == http.StatusNotImplemented
	}
	var nerr net.Error

	return !errors.As(err, &nerr)
}

// ErrNoVersion means a release image carries no version.json, or one
// without a version.
var ErrNoVersion = errors.New("release image has no version")

// ReleaseVersion reads the module version from version.json of a release
// image, e.g. the one a release channel tag points to. The file sits in a
// small layer of its own, so the layers are read smallest first and the
// documentation layer is usually never downloaded. A layer the registry
// does not have is ErrNotFound.
func ReleaseVersion(img v1.Image) (string, error) {
	version, err := releaseVersion(img)

	return version, notFound(err)
}

func releaseVersion(img v1.Image) (string, error) {
	layers, err := img.Layers()
	if err != nil {
		return "", fmt.Errorf("layers: %w", err)
	}

	sizes := make(map[v1.Layer]int64, len(layers))
	for _, l := range layers {
		size, err := l.Size()
		if err != nil {
			return "", fmt.Errorf("layer size: %w", err)
		}
		sizes[l] = size
	}
	slices.SortStableFunc(layers, func(a, b v1.Layer) int { return cmp.Compare(sizes[a], sizes[b]) })

	for _, l := range layers {
		version, found, err := versionFromLayer(l)
		if err != nil {
			return "", err
		}
		if found {
			return version, nil
		}
	}

	return "", ErrNoVersion
}

func versionFromLayer(l v1.Layer) (string, bool, error) {
	rc, err := l.Uncompressed()
	if err != nil {
		return "", false, fmt.Errorf("read layer: %w", err)
	}
	defer rc.Close()

	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return "", false, nil
		}
		if err != nil {
			return "", false, fmt.Errorf("read layer: %w", err)
		}

		if path.Clean(strings.TrimPrefix(hdr.Name, "/")) != "version.json" {
			continue
		}

		var v struct {
			Version string `json:"version"`
		}
		if err := json.NewDecoder(io.LimitReader(tr, 1<<20)).Decode(&v); err != nil {
			return "", false, fmt.Errorf("decode version.json: %w", err)
		}
		if v.Version == "" {
			return "", false, fmt.Errorf("version.json: %w", ErrNoVersion)
		}

		return v.Version, true, nil
	}
}

func (c *Client) image(ctx context.Context, reference string) (v1.Image, error) {
	ref, err := name.ParseReference(reference, c.nameOpts...)
	if err != nil {
		return nil, fmt.Errorf("parse reference: %w", err)
	}

	img, err := remote.Image(ref, c.withContext(ctx)...)
	if err != nil {
		return nil, fmt.Errorf("get image %s: %w", ref, err)
	}

	return img, nil
}

func (c *Client) withContext(ctx context.Context) []remote.Option {
	return slices.Concat(c.opts, []remote.Option{remote.WithContext(ctx)})
}

// maskedValue is what kube-apiserver returns for a sensitive field
// (x-kubernetes-sensitive-data) to a caller without access to it.
const maskedValue = "<omitted>"

// authenticator picks the credentials for the registry of repo as
// deckhouse-controller picks them (readAuthConfig in go_lib/dependency/cr):
// the login and the password first, else the entry of the docker config for
// the host of repo. Without a login and a docker config the access is
// anonymous.
func authenticator(repo string, src Source) (authn.Authenticator, error) {
	if src.Login == maskedValue || src.DockerCfg == maskedValue {
		return nil, errors.New("registry credentials are hidden by kube-apiserver: docs-builder needs get on modulesources/sensitive and packagerepositories/sensitive")
	}
	if src.Login != "" {
		return authn.FromConfig(authn.AuthConfig{Username: src.Login, Password: src.Password}), nil
	}
	if src.DockerCfg == "" {
		return authn.Anonymous, nil
	}

	// A docker config that is not base64 is taken as it is.
	raw, err := base64.StdEncoding.DecodeString(src.DockerCfg)
	if err != nil {
		raw = []byte(src.DockerCfg)
	}

	var cfg struct {
		Auths map[string]json.RawMessage `json:"auths"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse docker config: %w", err)
	}

	host, err := registryHost(repo)
	if err != nil {
		return nil, fmt.Errorf("parse repository %s: %w", repo, err)
	}
	// A key names the host as an address, a URL or a repository:
	// "registry.example.io", "https://registry.example.io" and
	// "registry.example.io/modules" are the same registry. Only the entry
	// taken is parsed, so a broken one of another registry does no harm. The
	// keys go in order, so of two keys for the host the same one is taken
	// every time.
	for _, key := range slices.Sorted(maps.Keys(cfg.Auths)) {
		if keyHost, err := registryHost(key); err != nil || keyHost != host {
			continue
		}

		var auth authn.AuthConfig
		if err := json.Unmarshal(cfg.Auths[key], &auth); err != nil {
			return nil, fmt.Errorf("parse docker config entry %s: %w", key, err)
		}

		return authn.FromConfig(auth), nil
	}

	return nil, fmt.Errorf("docker config has no credentials for %s", host)
}

// registryHost is the host, with the port, of a repository or of a key of a
// docker config, parsed as deckhouse-controller parses them.
func registryHost(ref string) (string, error) {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		u, err := url.ParseRequestURI(ref)
		if err != nil {
			return "", err
		}
		return u.Host, nil
	}

	u, err := url.Parse("//" + ref)
	if err != nil {
		return "", err
	}

	return u.Host, nil
}

// responseHeaderTimeout is how long a registry may take to start answering a
// request it took. A hung proxy, or a connection a firewall dropped without a
// word, fails the request instead of holding the caller for good; the
// connection is closed, so the next request opens a new one. The body is not
// bounded: a layer takes as long as the link needs, the callers bound it with
// their context.
const responseHeaderTimeout = time.Minute

// transportWithCA trusts the given PEM CA in addition to the system ones and
// gives up on a registry that does not start answering within headerTimeout.
func transportWithCA(ca string, headerTimeout time.Duration) (http.RoundTripper, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = headerTimeout
	if ca == "" {
		return transport, nil
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}

	if !pool.AppendCertsFromPEM([]byte(ca)) {
		return nil, errors.New("no PEM certificates found")
	}

	transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}

	return transport, nil
}

// Clients keeps one client per module source, so requests to a registry reuse
// its connections. A source whose registry settings changed gets a new client.
type Clients struct {
	// Metrics, when set before the first client is made, counts the
	// requests to the registries.
	Metrics *metricsstorage.MetricStorage

	mu      sync.Mutex
	clients map[string]cachedClient
}

type cachedClient struct {
	src    Source
	client *Client
}

func NewClients() *Clients {
	return &Clients{clients: make(map[string]cachedClient)}
}

// For returns the client of the source.
func (c *Clients) For(src Source) (*Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cached, ok := c.clients[src.Name]; ok && sameSource(cached.src, src) {
		return cached.client, nil
	}

	client, err := newClient(src, c.Metrics, responseHeaderTimeout)
	if err != nil {
		return nil, err
	}

	c.clients[src.Name] = cachedClient{src: src, client: client}

	return client, nil
}

// sameSource reports whether a and b are read from the same registries in
// the same way; their fallbacks are compared by value.
func sameSource(a, b Source) bool {
	if (a.Fallback == nil) != (b.Fallback == nil) {
		return false
	}
	if a.Fallback != nil && !sameSource(*a.Fallback, *b.Fallback) {
		return false
	}
	a.Fallback, b.Fallback = nil, nil

	return a == b
}

// countingTransport counts the requests to a registry by kind and status
// class: the registry load docs-builder makes.
type countingTransport struct {
	next http.RoundTripper
	ms   *metricsstorage.MetricStorage
}

func (t countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.next.RoundTrip(req)
	code := "error"
	if err == nil {
		code = strconv.Itoa(resp.StatusCode/100) + "xx"
	}
	t.ms.CounterAdd(metrics.DocsBuilderOnDemandRegistryRequests, 1, map[string]string{"request": requestKind(req), "code": code})

	return resp, err
}

// requestKind names a request of the distribution API. A token request goes
// to the realm of the registry, a blob may be redirected to a storage: both
// are outside /v2/.
func requestKind(req *http.Request) string {
	p := req.URL.Path
	switch {
	case p == "/v2/" || p == "/v2":
		return "ping"
	case !strings.HasPrefix(p, "/v2/"):
		if q := req.URL.Query(); q.Has("service") || q.Has("scope") {
			return "token"
		}
		return "other"
	case strings.HasSuffix(p, "/tags/list"):
		return "tags"
	case strings.Contains(p, "/manifests/") && req.Method == http.MethodHead:
		return "manifest_head"
	case strings.Contains(p, "/manifests/"):
		return "manifest"
	case strings.Contains(p, "/blobs/"):
		return "blob"
	default:
		return "other"
	}
}
