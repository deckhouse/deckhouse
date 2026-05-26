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
	//   /v1/images/<image>/tags                 -> list tags
	//   /v1/images/<image>/tags/<tag>           -> download last layer as tar.gz
	//
	// where <image> must match the allowlist (deckhouse-cli or deckhouse-cli/plugins/<plugin>).
	// kube-rbac-proxy (the standard sidecar listening on :4219) gates /v1/images/* with its
	// own SubjectAccessReview-based authorization, so this handler intentionally does no
	// authentication of its own.
	cliImagesPathPrefix = "/v1/images/"

	// packagesPathPrefix is the URL prefix served on the standard proxy mux for
	// deckhouse-cli (and plugin) downloads. Paths under it look like:
	//
	//   /v1/packages/<package-name>/metadata/icon/                 -> get icon of package latest version
	//   /v1/packages/<package-name>/metadata/icon/<version>        -> get icon of package specific version
	//
	// where <package-name> is the name of the package.
	// kube-rbac-proxy (the standard sidecar listening on :4219) gates /v1/packages/* with its
	// own SubjectAccessReview-based authorization, so this handler intentionally does no
	// authentication of its own.
	packagesPathPrefix = "/v1/packages/"
)

var errEmptyRegistryConfig = errors.New("empty registry config")

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
// Two URL shapes are supported under /v1/images/<image>/:
//
//	GET /v1/images/<image>/tags                 -> JSON list of available tags
//	GET /v1/images/<image>/tags/<tag>           -> stream the last layer of the image tag
//	                                                as application/x-gzip
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
	return GetPackageCached(ctx, p.logger, p.getter, p.registryClient, p.cache, digest, repository, path, p.config.SignCheck)
}

// GetPackageCached fetches an image package by manifest digest, first consulting the optional
// on-disk cache and falling back to the registry. On a cache miss the registry stream is teed
// into the cache asynchronously so the caller still gets a streaming reader.
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
) (int64, io.ReadCloser, error) {
	if pkgCache == nil {
		logger.Infof("Digest %q not found in local cache, trying to fetch package from registry", digest)
		size, _, reader, err := getPackageFromRegistry(ctx, logger, getter, registryClient, digest, repository, path, signCheck)
		return size, reader, err
	}

	size, cacheReader, err := pkgCache.Get(digest)
	if err == nil {
		return size, cacheReader, nil
	}
	if !errors.Is(err, cache.ErrEntryNotFound) {
		logger.Errorf("Get package from cache: %v", err)
		size, _, reader, err := getPackageFromRegistry(ctx, logger, getter, registryClient, digest, repository, path, signCheck)
		return size, reader, err
	}

	size, layerDigest, registryReader, err := getPackageFromRegistry(ctx, logger, getter, registryClient, digest, repository, path, signCheck)
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
) (int64, string, io.ReadCloser, error) {
	registryConfig, err := getter.Get(repository)
	if err != nil {
		return 0, "", nil, err
	}
	registryConfig.SignCheck = signCheck

	return registryClient.GetPackage(ctx, logger, registryConfig, digest, path)
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
	registryConfig.SignCheck = h.signCheck

	_, _, packageReader, err := h.registryClient.GetPackage(ctx, h.logger, registryConfig, digest, "")
	if err != nil {
		return nil, err
	}
	defer packageReader.Close()

	binary, err := extractTarGzFile(packageReader, h.binaryName)
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
	cliActionPullTag
)

type cliTagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func (h *cliHandler) serveHTTP(w http.ResponseWriter, r *http.Request) {
	imagePath, action, tag, err := parseCLIPath(r.URL.Path)
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
	case cliActionPullTag:
		h.handlePullTag(w, r, imagePath, tag)
	default:
		http.NotFound(w, r)
	}
}

func (h *cliHandler) handleListTags(w http.ResponseWriter, r *http.Request, imagePath string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientIP := getRequestIP(r)
	logger := h.proxy.logger
	logger.Infof("CLI list-tags for image %q from client %s", imagePath, clientIP)

	cfg, err := h.proxy.getter.Get(registry.DefaultRepository)
	if err != nil {
		logger.Errorf("get registry config: %v", err)
		http.Error(w, "registry config unavailable", http.StatusInternalServerError)
		return
	}
	if cfg == nil {
		http.Error(w, "registry config unavailable", http.StatusInternalServerError)
		return
	}
	cfg.SignCheck = h.proxy.config.SignCheck

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

func (h *cliHandler) handlePullTag(w http.ResponseWriter, r *http.Request, imagePath, tag string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientIP := getRequestIP(r)
	logger := h.proxy.logger
	logger.Infof("CLI pull image %q tag %q from client %s", imagePath, tag, clientIP)

	cfg, err := h.proxy.getter.Get(registry.DefaultRepository)
	if err != nil {
		logger.Errorf("get registry config: %v", err)
		http.Error(w, "registry config unavailable", http.StatusInternalServerError)
		return
	}
	if cfg == nil {
		http.Error(w, "registry config unavailable", http.StatusInternalServerError)
		return
	}
	cfg.SignCheck = h.proxy.config.SignCheck

	manifestDigest, err := h.proxy.registryClient.ResolveTag(r.Context(), logger, cfg, imagePath, tag)
	if err != nil {
		if errors.Is(err, registry.ErrPackageNotFound) {
			http.Error(w, "tag not found", http.StatusNotFound)
			return
		}
		logger.Errorf("resolve tag %q for %q: %v", tag, imagePath, err)
		http.Error(w, "failed to resolve tag", http.StatusBadGateway)
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
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.tar.gz"`, fileBase, tag))
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

// parseCLIPath splits an HTTP path of the form
//
//	/v1/images/<image-path>/tags
//	/v1/images/<image-path>/tags/<tag>
//
// into its components. <image-path> may contain slashes; the split anchors on the final
// /tags segment.
func parseCLIPath(urlPath string) (imagePath string, action cliAction, tag string, err error) {
	if !strings.HasPrefix(urlPath, cliImagesPathPrefix) {
		return "", cliActionUnknown, "", errors.New("not a CLI path")
	}
	// rest is the part of the path after the /v1/images/ prefix
	rest := strings.TrimPrefix(urlPath, cliImagesPathPrefix)
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return "", cliActionUnknown, "", errors.New("missing image path")
	}

	const sep = "/tags"
	idx := strings.LastIndex(rest, sep)
	if idx < 0 {
		return "", cliActionUnknown, "", errors.New("missing tags segment")
	}

	// imagePath is the part of the path before the tags segment
	imagePath = rest[:idx]
	if imagePath == "" {
		return "", cliActionUnknown, "", errors.New("empty image path")
	}

	// suffix is the part of the path after the tags segment
	suffix := rest[idx+len(sep):]
	switch {
	case suffix == "" || suffix == "/":
		return imagePath, cliActionListTags, "", nil
	case strings.HasPrefix(suffix, "/"):
		tag = strings.Trim(suffix[1:], "/")
		if tag == "" || strings.Contains(tag, "/") {
			return "", cliActionUnknown, "", errors.New("invalid tag")
		}
		return imagePath, cliActionPullTag, tag, nil
	default:
		return "", cliActionUnknown, "", errors.New("unexpected path suffix")
	}
}

// isAllowedCLIImagePath enforces the allowlist:
//   - deckhouse-cli
//   - deckhouse-cli/plugins/<single-segment>
func isAllowedCLIImagePath(imagePath string) bool {
	if imagePath == "deckhouse-cli" {
		return true
	}
	const pluginsPrefix = "deckhouse-cli/plugins"
	if !strings.HasPrefix(imagePath, pluginsPrefix) {
		return false
	}
	plugin := strings.TrimPrefix(imagePath, pluginsPrefix)
	// remove leading slash if present (/plugin -> plugin)
	plugin = strings.TrimPrefix(plugin, "/")
	if strings.Contains(plugin, "/") {
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

var (
	packagesActionToSegment = map[packagesAction]string{
		packagesMetadataActionGetIcon: "metadata/icon",
	}
)

// PackagesHandler returns an http.HandlerFunc that serves the /v1/packages/* packages routes
// (icon fetching) for this Proxy.
//
// Two URL shapes are supported under /v1/packages/<package-name>/:
//
//	GET /v1/packages/<package-name>/metadata/icon/                 -> get icon of package latest version
//	GET /v1/packages/<package-name>/metadata/icon/<version>        -> get icon of package specific version
//
// <package-name> must not contain slashes;
// <version> is a semantic version, eg. v0.0.1.
// other paths return 404.
func (p *Proxy) PackagesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		action, packageName, version, err := parsePackagesPath(r.URL.Path)
		if err != nil {
			p.logger.Errorf("parse packages path: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		clientIP := getRequestIP(r)
		logger := p.logger
		logger.Infof("Packages request from client %s", clientIP)

		switch action {
		case packagesMetadataActionGetIcon:
			p.handleGetIcon(w, r, packageName, version)
		default:
			http.NotFound(w, r)
		}
	}
}

// handleGetIcon handles the GET /v1/packages/<package-name>/metadata/icon/ or
// GET /v1/packages/<package-name>/metadata/icon/<version> request.
// It fetches the icon of the package and writes it to the response.
// If version is empty, it finds the latest version and fetches the icon of the latest version.
func (p *Proxy) handleGetIcon(w http.ResponseWriter, r *http.Request, packageName, version string) {
	cfg, err := p.getter.Get(registry.DefaultRepository)
	if err != nil {
		p.logger.Errorf("get registry config: %v", err)
		http.Error(w, "registry config unavailable", http.StatusInternalServerError)
		return
	}
	if cfg == nil {
		http.Error(w, "registry config unavailable", http.StatusInternalServerError)
		return
	}
	cfg.SignCheck = p.config.SignCheck

	imagePath := fmt.Sprintf("packages/%s", packageName)

	// if version is empty, find the latest version
	if version == "" {
		tags, err := p.registryClient.ListTags(r.Context(), p.logger, cfg, imagePath)
		if err != nil {
			if errors.Is(err, registry.ErrPackageNotFound) {
				http.Error(w, "package not found", http.StatusNotFound)
				return
			}
			p.logger.Errorf("list tags for %q: %v", imagePath, err)
			http.Error(w, "failed to list tags", http.StatusInternalServerError)
			return
		}
		if len(tags) == 0 {
			http.Error(w, "no tags found", http.StatusNotFound)
			return
		}

		var latestVersion *semver.Version
		for _, tag := range tags {
			v, err := semver.NewVersion(tag)
			if err != nil {
				continue
			}
			if latestVersion == nil || latestVersion.LessThan(v) {
				latestVersion = v
			}
		}
		if latestVersion == nil {
			http.Error(w, "no valid tags found", http.StatusNotFound)
			return
		}
		version = latestVersion.Original()
	}

	// get icon of package specific version
	manifestDigest, err := p.registryClient.ResolveTag(r.Context(), p.logger, cfg, imagePath, version)
	if err != nil {
		if errors.Is(err, registry.ErrPackageNotFound) {
			http.Error(w, "tag not found", http.StatusNotFound)
			return
		}
		p.logger.Errorf("resolve tag %q for %q: %v", version, imagePath, err)
		http.Error(w, "failed to resolve tag", http.StatusBadGateway)
		return
	}

	size, reader, err := GetPackageCached(
		r.Context(),
		p.logger,
		p.getter,
		p.registryClient,
		p.cache,
		manifestDigest,
		registry.DefaultRepository,
		imagePath,
		p.config.SignCheck,
	)
	if reader != nil {
		defer reader.Close()
	}
	if err != nil {
		if errors.Is(err, registry.ErrPackageNotFound) {
			http.Error(w, "package not found", http.StatusNotFound)
			return
		}
		p.logger.Errorf("get package for %q@%s: %v", imagePath, manifestDigest, err)
		http.Error(w, "failed to fetch package", http.StatusBadGateway)
		return
	}

	fileBase := imagePath
	if i := strings.LastIndex(imagePath, "/"); i >= 0 {
		fileBase = imagePath[i+1:]
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.png"`, fileBase))
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Header().Set("ETag", `"`+manifestDigest+`"`)
	w.Header().Set("Docker-Content-Digest", manifestDigest)

	if r.Method == http.MethodHead {
		return
	}

	// find icon in the oci image and copy it to the response
	icon, err := extractTarGzFile(reader, "docs/icon.svg")
	if err != nil {
		p.logger.Errorf("extract icon from package for %q@%s: %v", imagePath, manifestDigest, err)
		http.Error(w, "failed to extract icon", http.StatusBadGateway)
		return
	}
	_, _ = w.Write(icon)
}

// parsePackagesPath splits an HTTP path of the form:
// - /v1/packages/<package-name>/<action>/
// - /v1/packages/<package-name>/<action>/<version>
// into its components:
// - <package-name> must not contain slashes;
// - <action> must match packagesActionToSegment, eg. metadata/icon;
// - optional <version> is a semantic version, eg. v0.0.1.
// example:
// - /v1/packages/my-package/metadata/icon/ -> packagesMetadataActionGetIcon, my-package, ""
// - /v1/packages/my-package/metadata/icon/v0.0.1 -> packagesMetadataActionGetIcon, my-package, "v0.0.1"
func parsePackagesPath(urlPath string) (action packagesAction, packageName, version string, err error) {
	if !strings.HasPrefix(urlPath, packagesPathPrefix) {
		return packagesMetadataActionUnknown, "", "", errors.New("not a packages metadata path")
	}

	rest := strings.Trim(strings.TrimPrefix(urlPath, packagesPathPrefix), "/")
	packageName, afterPackage, ok := strings.Cut(rest, "/")
	if !ok || packageName == "" {
		return packagesMetadataActionUnknown, "", "", errors.New("missing package segment")
	}

	for actionType, segment := range packagesActionToSegment {
		switch {
		case afterPackage == segment:
			return actionType, packageName, "", nil
		case strings.HasPrefix(afterPackage, segment+"/"):
			tag := strings.TrimPrefix(afterPackage, segment+"/")
			if tag == "" || strings.Contains(tag, "/") {
				return packagesMetadataActionUnknown, "", "", errors.New("invalid version segment")
			}
			v, err := semver.NewVersion(tag)
			if err != nil {
				return packagesMetadataActionUnknown, "", "", fmt.Errorf("invalid semantic version: %w", err)
			}
			return actionType, packageName, v.String(), nil
		}
	}

	return packagesMetadataActionUnknown, "", "", errors.New("unknown action")
}

func normalizeBootstrapClusterUUID(clusterUUID string) string {
	return strings.Trim(strings.TrimSpace(clusterUUID), "/")
}

func extractTarGzFile(reader io.Reader, fileName string) ([]byte, error) {
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
				return nil, fmt.Errorf("file %q not found in archive", fileName)
			}

			return nil, err
		}

		if header.Typeflag != tar.TypeReg || path.Base(header.Name) != fileName {
			continue
		}

		return io.ReadAll(tarReader)
	}
}
