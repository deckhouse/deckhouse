// Copyright 2024 Flant JSC
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

package proxy

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/cache"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/log"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

type Proxy struct {
	server         *http.Server
	listener       net.Listener
	getter         registry.ClientConfigGetter
	registryClient registry.Client
	cache          cache.Cache
	logger         log.Logger
	config         Config
}

type RPPClientBinaryServer struct {
	server   *http.Server
	listener net.Listener
	logger   log.Logger
}

type RPPClientBinaryServerOptions struct {
	Listener           net.Listener
	Logger             log.Logger
	ClientConfigGetter registry.ClientConfigGetter
	RegistryClient     registry.Client
	SignCheck          bool
	ClusterUUID        string
}

type rppBinaryHandler struct {
	logger         log.Logger
	configGetter   registry.ClientConfigGetter
	registryClient registry.Client
	signCheck      bool
	binaryName     string
	expectedPath   string
}

const (
	rppBinaryName = "rpp-get"

	// cliImagesPathPrefix is the URL prefix served on the standard proxy mux for
	// deckhouse-cli (and plugin) downloads. Paths under it look like:
	//
	//   /v1/images/<image>/tags               -> list tags
	//   /v1/images/<image>/images/<version>   -> download OCI image tar.gz
	//   /v1/images/<image>/manifests/<ref>    -> raw image manifest
	//
	// where <image> must match the allowlist (deckhouse-cli or deckhouse-cli/plugins/<plugin>).
	// kube-rbac-proxy (the standard sidecar listening on :4219) gates /v1/images/* with its
	// own SubjectAccessReview-based authorization, so this handler intentionally does no
	// authentication of its own.
	cliImagesPathPrefix = "/v1/images/"

	// packagesPathPrefix is the URL prefix served on the standard proxy mux for
	// in-cluster package metadata (currently: icons). Paths under it look like:
	//
	//   /v1/packages/<packages-repo>/<package-name>/metadata/icon/          -> get icon of package latest version
	//   /v1/packages/<packages-repo>/<package-name>/metadata/icon/<version> -> get icon of package specific version
	//
	// where <packages-repo> is the PackageRepository CR name and <package-name>
	// is the OCI image name under that repository's spec.registry.repo.
	// kube-rbac-proxy (the standard sidecar listening on :4219) serves icon URLs
	// without authentication (see excludePaths in the module deployment). This
	// handler intentionally does no authentication of its own.
	//
	// /v1/packages/* is deliberately NOT routed through the public Ingress (see
	// templates/ingress.yaml), so anonymous access is bounded to the cluster:
	// callers reach it via the in-cluster Service (or hostPort 4219 on master
	// nodes during bootstrap), never via the public domain.
	packagesPathPrefix = "/v1/packages/"

	// maxIconBytes caps how much we are willing to read out of an OCI image
	// for an icon entry so that a hostile or accidentally-huge file cannot
	// blow up the proxy. 1 MiB comfortably accommodates raster icons; real
	// package icons are typically well under 200 KiB.
	maxIconBytes = 1 << 20
)

// iconCandidate describes one accepted package icon file: where to look for
// it inside the OCI image and how to serve it back.
type iconCandidate struct {
	// path is the in-archive path that must match (after normalization of
	// leading "./" / "/" by normalizeTarName).
	path string
	// contentType is the response Content-Type for this format.
	contentType string
	// ext is the filename extension stamped into Content-Disposition.
	ext string
}

// iconCandidates is the ordered list of icon files the proxy accepts, in
// priority order: SVG wins over raster formats because it's resolution
// independent. JPG is preferred over JPEG only so that the more common
// extension is picked when both happen to exist; both share the same MIME.
//
// Adding a new format is just an entry here (and a test in extract_tar_test.go).
var iconCandidates = []iconCandidate{
	{path: "docs/icon.svg", contentType: "image/svg+xml", ext: "svg"},
	{path: "docs/icon.png", contentType: "image/png", ext: "png"},
	{path: "docs/icon.jpg", contentType: "image/jpeg", ext: "jpg"},
	{path: "docs/icon.jpeg", contentType: "image/jpeg", ext: "jpeg"},
}

var (
	errEmptyRegistryConfig   = errors.New("empty registry config")
	errFileNotFoundInArchive = errors.New("file not found in archive")
)

func NewProxy(server *http.Server,
	listener net.Listener,
	clientConfigGetter registry.ClientConfigGetter,
	logger log.Logger,
	registryClient registry.Client, opts ...ProxyOption,
) *Proxy {
	p := &Proxy{
		server:         server,
		listener:       listener,
		getter:         clientConfigGetter,
		registryClient: registryClient,
		// by default, we set cache to nil, to use proxy without cache
		// to set up cache use WithCache option
		// using this option allows as to avoid interface conversion and
		// usage of reflect to determine whether we want to use cache or not
		cache:  nil,
		logger: logger,
	}

	for _, opt := range opts {
		opt(p)
	}
	return p
}

func NewRPPClientBinaryServerFromRegistry(opts RPPClientBinaryServerOptions) *RPPClientBinaryServer {
	handler := &rppBinaryHandler{
		logger:         opts.Logger,
		configGetter:   opts.ClientConfigGetter,
		registryClient: opts.RegistryClient,
		signCheck:      opts.SignCheck,
		binaryName:     rppBinaryName,
		expectedPath:   path.Join("/", normalizeBootstrapClusterUUID(opts.ClusterUUID), rppBinaryName),
	}

	return &RPPClientBinaryServer{
		server: &http.Server{
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      300 * time.Second,
			IdleTimeout:       30 * time.Second,
			MaxHeaderBytes:    4 << 10,
		},
		listener: opts.Listener,
		logger:   opts.Logger,
	}
}

type Config struct {
	SignCheck bool
}

func (p *Proxy) Serve(cfg *Config) {
	// Initialize runtime config (use zero values if nil)
	if cfg != nil {
		p.config = *cfg
	} else {
		p.config = Config{}
	}

	http.HandleFunc("/package", func(w http.ResponseWriter, r *http.Request) {
		requestIP := getRequestIP(r)

		if r.Method != http.MethodHead && r.Method != http.MethodGet {
			p.logger.Errorf("method %s from client %s is not allowed", r.Method, requestIP)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		digest := r.URL.Query().Get("digest")
		repository := r.URL.Query().Get("repository")
		additionalPath := r.URL.Query().Get("path")

		if repository == "" {
			repository = registry.DefaultRepository
		}

		logEntry := fmt.Sprintf("Received request with digest %q for repo %s from client %s", digest, repository, requestIP)

		if additionalPath != "" {
			logEntry = fmt.Sprintf("%s and additional path = %s", logEntry, additionalPath)
		}

		p.logger.Infof("%s", logEntry)

		if digest == "" {
			p.logger.Errorf("request from client %s: query %q is missing required parameter \"digest\"", requestIP, r.URL.RawQuery)
			http.Error(w, "missing required query parameter \"digest\"", http.StatusBadRequest)
			return
		}

		size, packageReader, err := p.getPackage(r.Context(), digest, repository, additionalPath)
		if packageReader != nil {
			defer packageReader.Close()
		}
		if err != nil {
			p.logger.Errorf("get package %q for client %s: %v", digest, requestIP, err)
			if errors.Is(err, registry.ErrPackageNotFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/x-gzip")
		w.Header().Set("Content-Disposition", "attachment; filename="+digest+".tar.gz")
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))

		// Cache for 1 year
		w.Header().Set("Cache-Control", `public, max-age=31536000`)
		w.Header().Set("ETag", "\""+digest+"\"")

		if r.Method == http.MethodHead {
			return
		}
		_, err = io.Copy(w, packageReader)
		if err != nil {
			p.logger.Errorf("send package: %v", err)
			return
		}

		p.logger.Infof("Package for digest %q sent successfully", digest)
	})

	p.ServeCLI()
	p.ServePackages()

	p.logger.Debugf("Starting packages proxy listener: %s", p.listener.Addr())

	if err := p.server.Serve(p.listener); err != nil && err != http.ErrServerClosed {
		p.logger.Error(err.Error())
	}
}

// CLIHandler returns an http.HandlerFunc that serves the /v1/images/* CLI download routes
// (image tag listing and binary pulling) for this Proxy.
//
// Three URL shapes are supported under /v1/images/<image>/:
//
//	GET /v1/images/<image>/tags               -> JSON list of available tags
//	GET /v1/images/<image>/images/<version>   -> stream OCI image tar.gz
//	                                              as application/x-gzip
//	GET /v1/images/<image>/manifests/<ref>    -> raw image manifest bytes
//
// <image> must match the deckhouse-cli allowlist (deckhouse-cli or
// deckhouse-cli/plugins/<plugin>); other paths return 404.
//
// kube-rbac-proxy (the standard sidecar listening on :4219) is responsible for authn/authz
// before requests reach this handler, so it intentionally performs no authentication itself.
func (p *Proxy) CLIHandler() http.HandlerFunc {
	handler := &cliHandler{proxy: p}
	return handler.serveHTTP
}

// ServeCLI mounts CLIHandler under /v1/images/ on http.DefaultServeMux so the routes are
// served by the standard proxy server exposed via kube-rbac-proxy on :4219. It is invoked
// automatically from Serve, and can also be called explicitly by callers that want to opt
// in without starting the full proxy listener.
func (p *Proxy) ServeCLI() {
	http.HandleFunc(cliImagesPathPrefix, p.CLIHandler())
}

func (s *RPPClientBinaryServer) Serve() {
	s.logger.Debugf("Starting rpp-get listener: %s", s.listener.Addr())

	if err := s.server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
		s.logger.Error(err.Error())
	}
}

// StopProxy stops proxy server but does not call os.Exit
func (p *Proxy) StopProxy() {
	p.logger.Infof("graceful shutdown packages proxy listener: %s", p.listener.Addr())
	err := p.server.Shutdown(context.Background())
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		p.logger.Error(err.Error())
	}
}

func (p *Proxy) Stop() {
	p.logger.Infof("graceful shutdown listener: %s", p.listener.Addr())
	err := p.server.Shutdown(context.Background())
	if err != nil && err != http.ErrServerClosed {
		p.logger.Error(err.Error())
		os.Exit(1)
	}
}

func (s *RPPClientBinaryServer) Stop() {
	s.logger.Infof("graceful shutdown rpp-get listener: %s", s.listener.Addr())

	err := s.server.Shutdown(context.Background())
	if err != nil && err != http.ErrServerClosed {
		s.logger.Error(err.Error())
	}
}

func (p *Proxy) getPackage(ctx context.Context, digest string, repository string, path string) (int64, io.ReadCloser, error) {
	return GetPackageCached(ctx, p.logger, p.getter, p.registryClient, p.cache, digest, repository, path, p.config.SignCheck, nil)
}

// GetPackageCached fetches an image package by manifest digest, first consulting the optional
// on-disk cache and falling back to the registry. On a cache miss the registry stream is teed
// into the cache asynchronously so the caller still gets a streaming reader.
//
// If registryConfig is nil, configuration is resolved via getter.Get(repository).
//
// The returned reader must be closed by the caller.
func GetPackageCached(
	ctx context.Context,
	logger log.Logger,
	getter registry.ClientConfigGetter,
	registryClient registry.Client,
	pkgCache cache.Cache,
	digest string,
	repository string,
	path string,
	signCheck bool,
	registryConfig *registry.ClientConfig,
) (int64, io.ReadCloser, error) {
	if pkgCache == nil {
		logger.Infof("Digest %q not found in local cache, trying to fetch package from registry", digest)
		size, _, reader, err := getPackageFromRegistry(ctx, logger, getter, registryClient, digest, repository, path, signCheck, registryConfig)
		return size, reader, err
	}

	size, cacheReader, err := pkgCache.Get(digest)
	if err == nil {
		return size, cacheReader, nil
	}
	if !errors.Is(err, cache.ErrEntryNotFound) {
		logger.Errorf("Get package from cache: %v", err)
		size, _, reader, err := getPackageFromRegistry(ctx, logger, getter, registryClient, digest, repository, path, signCheck, registryConfig)
		return size, reader, err
	}

	size, layerDigest, registryReader, err := getPackageFromRegistry(ctx, logger, getter, registryClient, digest, repository, path, signCheck, registryConfig)
	if err != nil {
		return 0, nil, err
	}

	// TeeReader returns teeReader which writes to pipeWriter what it reads from registryReader
	// pipeWriter is pipe to pipeReader
	// so when we read from teeReader it read content from registryReader and write it to the pipeWriter
	// and on the another side of the pipe we have second reader - pipeReader
	// thus, we have two readers - teeReader and pipeReader that is copy of registryReader
	pipeReader, pipeWriter := io.Pipe()
	teeReader := io.TeeReader(registryReader, pipeWriter)

	go func() {
		defer registryReader.Close()
		defer pipeWriter.Close()

		err := pkgCache.Set(digest, layerDigest, teeReader)
		if err == nil {
			return
		}
		logger.Errorf("cache set for digest %q: %v", digest, err)
		_, err = io.Copy(pipeWriter, registryReader)
		if err != nil {
			logger.Errorf("copy registry reader to pipe for digest %q: %v", digest, err)
		}
	}()

	return size, pipeReader, nil
}

func getPackageFromRegistry(
	ctx context.Context,
	logger log.Logger,
	getter registry.ClientConfigGetter,
	registryClient registry.Client,
	digest string,
	repository string,
	path string,
	signCheck bool,
	registryConfig *registry.ClientConfig,
) (int64, string, io.ReadCloser, error) {
	if registryConfig == nil {
		var err error
		registryConfig, err = getter.Get(repository)
		if err != nil {
			return 0, "", nil, err
		}
	}
	// Create a local copy so SignCheck does not mutate shared getter-backed configs.
	registryConfigWithSignCheck := *registryConfig
	registryConfigWithSignCheck.SignCheck = signCheck

	return registryClient.GetPackage(ctx, logger, &registryConfigWithSignCheck, digest, path)
}

type ProxyOption func(*Proxy)

func WithCache(cache cache.Cache) ProxyOption {
	return func(p *Proxy) {
		p.cache = cache
	}
}

func getRequestIP(r *http.Request) string {
	IPAddress := r.Header.Get("X-Real-Ip")
	if IPAddress == "" {
		IPAddress = r.Header.Get("X-Forwarded-For")
	}
	if IPAddress == "" {
		IPAddress = r.RemoteAddr
	}
	return IPAddress
}

func (h *rppBinaryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestIP := getRequestIP(r)

	if r.URL.Path != h.expectedPath {
		h.logger.Warnf("rpp-get request from client %s for unexpected path %q, expected %q", requestIP, r.URL.Path, h.expectedPath)
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		h.logger.Warnf("rpp-get request from client %s with method %s is not allowed", requestIP, r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	digest := r.URL.Query().Get("digest")
	if digest == "" {
		h.logger.Warnf("rpp-get request from client %s: query %q is missing required parameter \"digest\"", requestIP, r.URL.RawQuery)
		http.Error(w, "missing required query parameter \"digest\"", http.StatusBadRequest)
		return
	}

	h.logger.Infof("Received rpp-get request with digest %q from client %s", digest, requestIP)

	binary, err := h.fetchBinary(r.Context(), digest)
	if err != nil {
		h.writeFetchError(w, digest, requestIP, err)
		return
	}

	h.writeBinaryResponse(w, binary)
	h.logger.Infof("rpp-get binary for digest %q sent successfully to client %s, size %d", digest, requestIP, len(binary))
}

func (h *rppBinaryHandler) fetchBinary(ctx context.Context, digest string) ([]byte, error) {
	registryConfig, err := h.configGetter.Get(registry.DefaultRepository)
	if err != nil {
		return nil, fmt.Errorf("get registry config: %w", err)
	}
	if registryConfig == nil {
		return nil, errEmptyRegistryConfig
	}
	// Copy before mutating so we don't race with the watcher (which may
	// rewrite registryClientConfigs under Lock while readers still hold a
	// pointer to the previous value).
	localCfg := *registryConfig
	localCfg.SignCheck = h.signCheck

	_, _, packageReader, err := h.registryClient.GetPackage(ctx, h.logger, &localCfg, digest, "")
	if err != nil {
		return nil, err
	}
	defer packageReader.Close()

	// rpp-get binaries are small (a few MiB); cap reads at 64 MiB so a
	// malformed or hostile archive cannot exhaust the process memory.
	const maxBinaryBytes = 64 << 20
	binary, err := extractTarGzFile(packageReader, baseNameMatcher(h.binaryName), maxBinaryBytes)
	if err != nil {
		return nil, fmt.Errorf("extract %s binary: %w", h.binaryName, err)
	}

	return binary, nil
}

func (h *rppBinaryHandler) writeFetchError(w http.ResponseWriter, digest, requestIP string, err error) {
	if errors.Is(err, registry.ErrPackageNotFound) {
		h.logger.Warnf("rpp-get package %q requested by client %s was not found", digest, requestIP)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.logger.Errorf("fetch %s package %q requested by client %s: %v", h.binaryName, digest, requestIP, err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func (h *rppBinaryHandler) writeBinaryResponse(w http.ResponseWriter, binary []byte) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, h.binaryName))
	w.Header().Set("Content-Length", strconv.Itoa(len(binary)))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if _, err := w.Write(binary); err != nil {
		h.logger.Errorf("write %s response: %v", h.binaryName, err)
	}
}

// cliHandler implements the /v1/images/* HTTP routes documented on Proxy.ServeCLI.
// It is intentionally thin: all of the registry-config / cache state lives on the parent Proxy
// so a single Proxy instance backs both /package and /v1/images/* on the same standard server.
type cliHandler struct {
	proxy *Proxy
}

type cliAction int

const (
	cliActionUnknown cliAction = iota
	cliActionListTags
	cliActionPullImage
	cliActionGetManifest
)

type cliTagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func (h *cliHandler) serveHTTP(w http.ResponseWriter, r *http.Request) {
	imagePath, action, ref, err := parseCLIPath(r.URL.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if !isAllowedCLIImagePath(imagePath) {
		h.proxy.logger.Warnf("CLI request for disallowed image path %q from %s", imagePath, getRequestIP(r))
		http.NotFound(w, r)
		return
	}

	switch action {
	case cliActionListTags:
		h.handleListTags(w, r, imagePath)
	case cliActionPullImage:
		h.handlePullImage(w, r, imagePath, ref)
	case cliActionGetManifest:
		h.handleGetManifest(w, r, imagePath, ref)
	default:
		http.NotFound(w, r)
	}
}

// cliClientConfig resolves the default registry config and returns a private
// copy with SignCheck stamped in. Callers must NOT mutate the value returned
// directly by the getter: the watcher rewrites those entries under a write
// lock while readers still hold a pointer to them.
func (h *cliHandler) cliClientConfig(w http.ResponseWriter, logger log.Logger) (*registry.ClientConfig, bool) {
	cfg, err := h.proxy.getter.Get(registry.DefaultRepository)
	if err != nil {
		logger.Errorf("get registry config: %v", err)
		http.Error(w, "registry config unavailable", http.StatusInternalServerError)
		return nil, false
	}
	if cfg == nil {
		http.Error(w, "registry config unavailable", http.StatusInternalServerError)
		return nil, false
	}
	local := *cfg
	local.SignCheck = h.proxy.config.SignCheck
	return &local, true
}

func (h *cliHandler) handleListTags(w http.ResponseWriter, r *http.Request, imagePath string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientIP := getRequestIP(r)
	logger := h.proxy.logger
	logger.Infof("CLI list-tags for image %q from client %s", imagePath, clientIP)

	cfg, ok := h.cliClientConfig(w, logger)
	if !ok {
		return
	}

	tags, err := h.proxy.registryClient.ListTags(r.Context(), logger, cfg, imagePath)
	if err != nil {
		if errors.Is(err, registry.ErrPackageNotFound) {
			http.Error(w, "image not found", http.StatusNotFound)
			return
		}
		logger.Errorf("list tags for %q: %v", imagePath, err)
		http.Error(w, "failed to list tags", http.StatusBadGateway)
		return
	}

	body, err := json.Marshal(cliTagsResponse{Name: imagePath, Tags: tags})
	if err != nil {
		logger.Errorf("marshal tags: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = w.Write(body)
}

// handleGetManifest serves an image's raw manifest without pulling any layers, so
// the CLI can read the plugin contract from its annotations itself. Returns the
// manifest bytes (200) with the upstream media type, or 404 when ref does not exist.
func (h *cliHandler) handleGetManifest(w http.ResponseWriter, r *http.Request, imagePath, ref string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientIP := getRequestIP(r)
	logger := h.proxy.logger
	logger.Infof("CLI get-manifest for image %q ref %q from client %s", imagePath, ref, clientIP)

	cfg, ok := h.cliClientConfig(w, logger)
	if !ok {
		return
	}

	manifest, mediaType, err := h.proxy.registryClient.GetRawManifest(r.Context(), logger, cfg, imagePath, ref)
	if err != nil {
		if errors.Is(err, registry.ErrPackageNotFound) {
			http.Error(w, "manifest not found", http.StatusNotFound)
			return
		}
		logger.Errorf("get manifest for %q ref %q: %v", imagePath, ref, err)
		http.Error(w, "failed to read manifest", http.StatusBadGateway)
		return
	}

	if mediaType == "" {
		mediaType = "application/vnd.oci.image.manifest.v1+json"
	}

	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Content-Length", strconv.Itoa(len(manifest)))
	_, _ = w.Write(manifest)
}

func (h *cliHandler) handlePullImage(w http.ResponseWriter, r *http.Request, imagePath, version string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientIP := getRequestIP(r)
	logger := h.proxy.logger

	platform, err := parsePlatformQuery(r)
	if err != nil {
		logger.Errorf("CLI pull image %q version %q from client %s: invalid platform: %v", imagePath, version, clientIP, err)
		http.Error(w, "invalid platform", http.StatusBadRequest)
		return
	}

	logger.Infof("CLI pull image %q version %q platform %q from client %s", imagePath, version, platformString(platform), clientIP)

	cfg, ok := h.cliClientConfig(w, logger)
	if !ok {
		return
	}

	manifestDigest, err := h.proxy.registryClient.ResolveTag(r.Context(), logger, cfg, imagePath, version, platform)
	if err != nil {
		if errors.Is(err, registry.ErrPackageNotFound) {
			http.Error(w, "version not found", http.StatusNotFound)
			return
		}
		logger.Errorf("resolve version %q for %q: %v", version, imagePath, err)
		http.Error(w, "failed to resolve version", http.StatusBadGateway)
		return
	}

	size, reader, err := GetPackageCached(
		r.Context(),
		logger,
		h.proxy.getter,
		h.proxy.registryClient,
		h.proxy.cache,
		manifestDigest,
		registry.DefaultRepository,
		imagePath,
		h.proxy.config.SignCheck,
		cfg,
	)
	if reader != nil {
		defer reader.Close()
	}
	if err != nil {
		if errors.Is(err, registry.ErrPackageNotFound) {
			http.Error(w, "package not found", http.StatusNotFound)
			return
		}
		logger.Errorf("get package for %q@%s: %v", imagePath, manifestDigest, err)
		http.Error(w, "failed to fetch package", http.StatusBadGateway)
		return
	}

	fileBase := imagePath
	if i := strings.LastIndex(imagePath, "/"); i >= 0 {
		fileBase = imagePath[i+1:]
	}

	w.Header().Set("Content-Type", "application/x-gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.tar.gz"`, fileBase, version))
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Header().Set("ETag", `"`+manifestDigest+`"`)
	w.Header().Set("Docker-Content-Digest", manifestDigest)

	if r.Method == http.MethodHead {
		return
	}

	if _, err := io.Copy(w, reader); err != nil {
		logger.Errorf("stream package for %q@%s: %v", imagePath, manifestDigest, err)
	}
}

// parsePlatformQuery reads the optional ?platform=<os>-<arch> selector a CLI
// client attaches to a pull. Absent -> nil, which keeps the legacy single-manifest
// behavior (the registry default platform). The dash separator keeps the value a
// single unescaped URL token; go-containerregistry's Platform uses os/arch, so we
// build it directly. A malformed value is an error so the handler answers 400
// instead of silently serving the wrong architecture.
func parsePlatformQuery(r *http.Request) (*v1.Platform, error) {
	raw := r.URL.Query().Get("platform")
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, "-")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("platform %q is not <os>-<arch>", raw)
	}

	return &v1.Platform{OS: parts[0], Architecture: parts[1]}, nil
}

// platformString renders a platform for logs ("any" when none was requested).
func platformString(p *v1.Platform) string {
	if p == nil {
		return "any"
	}

	return p.String()
}

// parseCLIPath splits an HTTP path of the form
//
//	/v1/images/<image-path>/tags
//	/v1/images/<image-path>/images/<version>
//
// into its components. <image-path> may contain slashes; the split anchors on the
// action segment, taking its last occurrence so a plugin named after a segment word
// is not misparsed. The returned third value is the version (empty for a tags list).
func parseCLIPath(urlPath string) (string, cliAction, string, error) {
	if !strings.HasPrefix(urlPath, cliImagesPathPrefix) {
		return "", cliActionUnknown, "", errors.New("not a CLI path")
	}

	rest := strings.Trim(strings.TrimPrefix(urlPath, cliImagesPathPrefix), "/")
	if rest == "" {
		return "", cliActionUnknown, "", errors.New("missing image path")
	}

	if imagePath, ok := strings.CutSuffix(rest, "/tags"); ok {
		if imagePath == "" {
			return "", cliActionUnknown, "", errors.New("empty image path")
		}
		return imagePath, cliActionListTags, "", nil
	}

	if imagePath, version, ok := cutCLIActionSegment(rest, "images"); ok {
		return imagePath, cliActionPullImage, version, nil
	}

	if imagePath, ref, ok := cutCLIActionSegment(rest, "manifests"); ok {
		return imagePath, cliActionGetManifest, ref, nil
	}

	return "", cliActionUnknown, "", errors.New("unexpected path suffix")
}

// cutCLIActionSegment splits rest of the form "<image-path>/<segment>/<value>" into
// (imagePath, value, true). It anchors on the LAST "/<segment>/" so a plugin whose
// name equals the segment word is not confused. value must be a single non-empty,
// slash-free path component.
func cutCLIActionSegment(rest, segment string) (string, string, bool) {
	marker := "/" + segment + "/"
	idx := strings.LastIndex(rest, marker)
	if idx <= 0 {
		return "", "", false
	}

	imagePath := rest[:idx]
	value := rest[idx+len(marker):]
	if value == "" || strings.Contains(value, "/") {
		return "", "", false
	}

	return imagePath, value, true
}

// isAllowedCLIImagePath enforces the allowlist:
//   - deckhouse-cli
//   - deckhouse-cli/plugins/<single non-empty segment>
//
// The bare "deckhouse-cli/plugins" / "deckhouse-cli/plugins/" forms are NOT
// allowed: they map to an OCI repo with an empty trailing path segment which
// name.NewRepository would reject downstream anyway. Refusing them here keeps
// the error 404 (allowlist) instead of leaking a registry error.
func isAllowedCLIImagePath(imagePath string) bool {
	if imagePath == "deckhouse-cli" {
		return true
	}
	const pluginsPrefix = "deckhouse-cli/plugins/"
	if !strings.HasPrefix(imagePath, pluginsPrefix) {
		return false
	}
	plugin := strings.TrimPrefix(imagePath, pluginsPrefix)
	if plugin == "" || strings.Contains(plugin, "/") {
		return false
	}
	return true
}

func (p *Proxy) ServePackages() {
	http.HandleFunc(packagesPathPrefix, p.PackagesHandler())
}

type packagesAction int

const (
	packagesMetadataActionUnknown packagesAction = iota
	packagesMetadataActionGetIcon
)

// packagesRoutes is the ordered list of recognized action segments. Order is
// significant: parsePackagesPath walks it top-down and picks the FIRST match,
// so longer/more-specific segments must come first if any future segment is
// a prefix of another (e.g. "metadata/icon-small" before "metadata/icon").
var packagesRoutes = []struct {
	segment string
	action  packagesAction
}{
	{"metadata/icon", packagesMetadataActionGetIcon},
}

// PackagesHandler returns an http.HandlerFunc that serves the /v1/packages/* packages routes
func (p *Proxy) PackagesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		action, packageRepositoryName, packageName, version, err := parsePackagesPath(r.URL.Path)
		if err != nil {
			// Don't surface internal parser detail to anonymous clients;
			// they only need to know the URL didn't parse.
			p.logger.Warnf("parse packages path %q from %s: %v", r.URL.Path, getRequestIP(r), err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		packagesCfg, err := p.getter.GetPackagesConfig(packageRepositoryName)
		if err != nil {
			p.logger.Errorf("get packages config for %q: %v", packageRepositoryName, err)
			http.Error(w, "package repository not found", http.StatusNotFound)
			return
		}
		if packagesCfg == nil {
			p.logger.Errorf("get packages config for %q: nil config", packageRepositoryName)
			http.Error(w, "package repository not found", http.StatusNotFound)
			return
		}

		// Icon extraction needs to read docs/icon.svg regardless of which
		// layer it was added in, so the registry client must flatten all
		// layers for /v1/packages/* routes.
		cfg := packagesCfg.ToClientConfig(p.config.SignCheck, true)

		clientIP := getRequestIP(r)
		p.logger.Infof("Packages request from client %s", clientIP)

		switch action {
		case packagesMetadataActionGetIcon:
			p.handleGetIcon(w, r, cfg, packageName, version)
		default:
			http.NotFound(w, r)
		}
	}
}

// handleGetIcon serves
//
//	GET  /v1/packages/<repo>/<package>/metadata/icon[/<version>]
//	HEAD /v1/packages/<repo>/<package>/metadata/icon[/<version>]
//
// If <version> is omitted the latest semver tag is resolved on the fly. The
// SVG itself is extracted from docs/icon.svg inside the OCI image whose tag
// resolves to that version. Response headers (Content-Type, Content-Length,
// ETag, Cache-Control) are only written after the icon bytes are in hand, so
// errors don't leave the client with a misleading "attachment; filename=*.svg"
// download containing a plain-text error message.
func (p *Proxy) handleGetIcon(w http.ResponseWriter, r *http.Request, cfg *registry.ClientConfig, packageName, version string) {
	if cfg == nil {
		http.Error(w, "registry config unavailable", http.StatusInternalServerError)
		return
	}

	imagePath := packageName + "/version"
	p.logger.Debugf("handleGetIcon for %q/%q:%q", cfg.Repository, imagePath, version)

	if version == "" {
		resolved, ok := p.resolveLatestVersion(w, r, cfg, imagePath)
		if !ok {
			return
		}
		version = resolved
	}

	manifestDigest, ok := p.resolveManifestDigest(w, r, cfg, imagePath, version)
	if !ok {
		return
	}

	icon, cand, ok := p.fetchIcon(w, r, cfg, imagePath, manifestDigest)
	if !ok {
		return
	}

	// Icons are immutable for a given manifest digest, so cache aggressively
	// downstream. Content-Type and the filename extension come from which
	// file we actually found inside the OCI image (see iconCandidates).
	// ETag mirrors what cliHandler.handlePullImage does so the header surface
	// is consistent between routes.
	w.Header().Set("Content-Type", cand.contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.%s"`, packageName, cand.ext))
	w.Header().Set("Content-Length", strconv.Itoa(len(icon)))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+manifestDigest+`"`)
	w.Header().Set("Docker-Content-Digest", manifestDigest)

	if r.Method == http.MethodHead {
		return
	}

	if _, err := w.Write(icon); err != nil {
		p.logger.Errorf("write icon for %q@%s: %v", imagePath, manifestDigest, err)
	}
}

// resolveLatestVersion finds the largest semver tag for imagePath, writing an
// HTTP error response and returning ok=false on any failure.
func (p *Proxy) resolveLatestVersion(w http.ResponseWriter, r *http.Request, cfg *registry.ClientConfig, imagePath string) (string, bool) {
	tags, err := p.registryClient.ListTags(r.Context(), p.logger, cfg, imagePath)
	if err != nil {
		if errors.Is(err, registry.ErrPackageNotFound) {
			http.Error(w, "package not found", http.StatusNotFound)
			return "", false
		}
		p.logger.Errorf("list tags for %q: %v", imagePath, err)
		http.Error(w, "failed to list tags", http.StatusBadGateway)
		return "", false
	}
	if len(tags) == 0 {
		http.Error(w, "no tags found", http.StatusNotFound)
		return "", false
	}

	var latest *semver.Version
	for _, tag := range tags {
		v, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}
		if latest == nil || latest.LessThan(v) {
			latest = v
		}
	}
	if latest == nil {
		http.Error(w, "no valid tags found", http.StatusNotFound)
		return "", false
	}

	version := latest.Original()
	p.logger.Debugf("resolved latest version for %q: %q", imagePath, version)
	return version, true
}

func (p *Proxy) resolveManifestDigest(w http.ResponseWriter, r *http.Request, cfg *registry.ClientConfig, imagePath, version string) (string, bool) {
	manifestDigest, err := p.registryClient.ResolveTag(r.Context(), p.logger, cfg, imagePath, version, nil)
	if err != nil {
		if errors.Is(err, registry.ErrPackageNotFound) {
			http.Error(w, "tag not found", http.StatusNotFound)
			return "", false
		}
		p.logger.Errorf("resolve tag %q for %q: %v", version, imagePath, err)
		http.Error(w, "failed to resolve tag", http.StatusBadGateway)
		return "", false
	}
	return manifestDigest, true
}

func (p *Proxy) fetchIcon(w http.ResponseWriter, r *http.Request, cfg *registry.ClientConfig, imagePath, manifestDigest string) ([]byte, iconCandidate, bool) {
	_, reader, err := GetPackageCached(
		r.Context(),
		p.logger,
		p.getter,
		p.registryClient,
		p.cache,
		manifestDigest,
		cfg.Repository,
		imagePath,
		p.config.SignCheck,
		cfg,
	)
	if reader != nil {
		defer reader.Close()
	}
	if err != nil {
		if errors.Is(err, registry.ErrPackageNotFound) {
			http.Error(w, "package not found", http.StatusNotFound)
			return nil, iconCandidate{}, false
		}
		p.logger.Errorf("get package for %q@%s: %v", imagePath, manifestDigest, err)
		http.Error(w, "failed to fetch package", http.StatusBadGateway)
		return nil, iconCandidate{}, false
	}

	icon, cand, err := extractIcon(reader)
	if err != nil {
		// Any failure to surface a recognized icon - no candidate present,
		// a corrupted gzip stream, or an entry that exceeds maxIconBytes -
		// is surfaced as 404 to the client. Icons are public best-effort
		// metadata: the caller (browser, console) should just fall back to
		// a default icon. The underlying cause is logged for ops; the
		// client can't usefully distinguish them.
		p.logger.Warnf("extract icon from package for %q@%s: %v", imagePath, manifestDigest, err)
		http.Error(w, "icon not found", http.StatusNotFound)
		return nil, iconCandidate{}, false
	}
	return icon, cand, true
}

// parsePackagesPath splits an HTTP path of the form:
// - /v1/packages/<packages-repo>/<package-name>/<action>/
// - /v1/packages/<packages-repo>/<package-name>/<action>/<version>
// into its components:
// - <packages-repo> and <package-name> must not contain slashes;
// - <action> must match packagesActionToSegment, eg. metadata/icon;
// - optional <version> is a semantic version, eg. v0.0.1.
// example:
// - /v1/packages/deckhouse/my-package/metadata/icon/ -> packagesMetadataActionGetIcon, deckhouse, my-package, ""
// - /v1/packages/deckhouse/my-package/metadata/icon/v0.0.1 -> packagesMetadataActionGetIcon, deckhouse, my-package, "v0.0.1"
func parsePackagesPath(urlPath string) (packagesAction, string, string, string, error) {
	if !strings.HasPrefix(urlPath, packagesPathPrefix) {
		return packagesMetadataActionUnknown, "", "", "", errors.New("not a packages metadata path")
	}

	rest := strings.Trim(strings.TrimPrefix(urlPath, packagesPathPrefix), "/")
	packageRepositoryName, afterRepo, ok := strings.Cut(rest, "/")
	if !ok || packageRepositoryName == "" {
		return packagesMetadataActionUnknown, "", "", "", errors.New("missing package repository segment")
	}

	packageName, afterPackage, ok := strings.Cut(afterRepo, "/")
	if !ok || packageName == "" {
		return packagesMetadataActionUnknown, "", "", "", errors.New("missing package segment")
	}

	for _, route := range packagesRoutes {
		switch {
		case afterPackage == route.segment:
			return route.action, packageRepositoryName, packageName, "", nil
		case strings.HasPrefix(afterPackage, route.segment+"/"):
			tag := strings.TrimPrefix(afterPackage, route.segment+"/")
			if tag == "" || strings.Contains(tag, "/") {
				return packagesMetadataActionUnknown, "", "", "", errors.New("invalid version segment")
			}
			if _, err := semver.NewVersion(tag); err != nil {
				return packagesMetadataActionUnknown, "", "", "", fmt.Errorf("invalid semantic version: %w", err)
			}
			return route.action, packageRepositoryName, packageName, tag, nil
		}
	}

	return packagesMetadataActionUnknown, "", "", "", errors.New("unknown action")
}

func normalizeBootstrapClusterUUID(clusterUUID string) string {
	return strings.Trim(strings.TrimSpace(clusterUUID), "/")
}

// tarEntryMatcher reports whether a tar header is the entry the caller wants.
// Implementations should be cheap (only the tar header is available).
type tarEntryMatcher func(header *tar.Header) bool

// exactNameMatcher returns a matcher that picks a single, fully-qualified
// entry path. Leading "./" and "/" segments in the header name are stripped
// before comparison so archives produced by different tools (`tar` strips
// "./", BuildKit doesn't) match the same target.
func exactNameMatcher(filePath string) tarEntryMatcher { //nolint:unparam
	want := normalizeTarName(filePath)
	return func(header *tar.Header) bool {
		return normalizeTarName(header.Name) == want
	}
}

// baseNameMatcher returns a matcher that picks any regular entry whose
// basename equals fileName, regardless of where it lives in the archive.
func baseNameMatcher(fileName string) tarEntryMatcher {
	return func(header *tar.Header) bool {
		return path.Base(normalizeTarName(header.Name)) == fileName
	}
}

func normalizeTarName(name string) string {
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimPrefix(name, "/")
	return name
}

// extractIcon walks the gzipped tar in reader once and returns the icon entry
// that ranks highest in iconCandidates (lower index = higher priority). The
// scan stops early when the top-priority candidate is found.
//
// The OCI image stream is single-use, so this MUST be done in one pass: we
// can't rewind to look for SVG after we already saw PNG. We solve that by
// buffering whichever match we currently believe is "best" and overwriting
// it if a higher-priority one shows up later in the same archive.
//
// If no candidate is present, errFileNotFoundInArchive is returned so the
// caller can map it to a 404. Any other error (corrupt gzip, oversized
// entry, IO failure) is returned wrapped.
func extractIcon(reader io.Reader) ([]byte, iconCandidate, error) {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, iconCandidate{}, fmt.Errorf("read gzip stream: %w", err)
	}
	defer gzipReader.Close()

	// Pre-index candidates by normalized path for O(1) lookup per entry,
	// remembering the priority (slice index).
	type indexed struct {
		priority int
		cand     iconCandidate
	}
	pathToCandidate := make(map[string]indexed, len(iconCandidates))
	for i, c := range iconCandidates {
		pathToCandidate[normalizeTarName(c.path)] = indexed{priority: i, cand: c}
	}

	bestPriority := -1
	var bestData []byte
	var bestCand iconCandidate

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, iconCandidate{}, fmt.Errorf("read tar header: %w", err)
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		entry, ok := pathToCandidate[normalizeTarName(header.Name)]
		if !ok {
			continue
		}
		// Already saw an equal-or-higher priority match; ignore this one.
		if bestPriority != -1 && entry.priority >= bestPriority {
			continue
		}

		data, err := readTarEntry(tarReader, maxIconBytes)
		if err != nil {
			return nil, iconCandidate{}, err
		}

		bestPriority = entry.priority
		bestData = data
		bestCand = entry.cand

		// Top priority found - no point scanning the rest of the archive.
		if bestPriority == 0 {
			return bestData, bestCand, nil
		}
	}

	if bestPriority == -1 {
		return nil, iconCandidate{}, errFileNotFoundInArchive
	}
	return bestData, bestCand, nil
}

// readTarEntry reads at most maxBytes from the current tar entry, returning
// an error if the entry exceeds the cap. Centralized so extractIcon and
// extractTarGzFile share the same overflow semantics.
func readTarEntry(tarReader io.Reader, maxBytes int64) ([]byte, error) {
	// +1 so we can detect overflow: io.LimitReader stops at maxBytes,
	// which would silently truncate a maxBytes-sized real file.
	data, err := io.ReadAll(io.LimitReader(tarReader, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read tar entry: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("tar entry exceeds %d bytes", maxBytes)
	}
	return data, nil
}

// extractTarGzFile reads a gzipped tar from reader and returns the bytes of
// the first regular file matched by matcher, capped at maxBytes to protect
// the proxy from a hostile or accidentally-huge entry. If no entry matches,
// errFileNotFoundInArchive is returned so callers can map it to a 404.
func extractTarGzFile(reader io.Reader, matcher tarEntryMatcher, maxBytes int64) ([]byte, error) {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("read gzip stream: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, errFileNotFoundInArchive
			}
			return nil, err
		}

		if header.Typeflag != tar.TypeReg || !matcher(header) {
			continue
		}

		return readTarEntry(tarReader, maxBytes)
	}
}
