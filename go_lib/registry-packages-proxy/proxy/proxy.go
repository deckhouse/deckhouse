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
		if r.Method != http.MethodHead && r.Method != http.MethodGet {
			p.logger.Error("method not allowed")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		requestIP := getRequestIP(r)
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
			p.logger.Error("missing digest")
			http.Error(w, "missing digest", http.StatusBadRequest)
			return
		}

		size, packageReader, err := p.getPackage(r.Context(), digest, repository, additionalPath)
		if packageReader != nil {
			defer packageReader.Close()
		}
		if err != nil {
			p.logger.Error(err.Error())
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

	p.logger.Debugf("Starting packages proxy listener: %s", p.listener.Addr())

	if err := p.server.Serve(p.listener); err != nil && err != http.ErrServerClosed {
		p.logger.Error(err.Error())
	}
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
	// if cache is nil, return digest directly from registry
	if p.cache == nil {
		p.logger.Infof("Digest %q not found in local cache, trying to fetch package from registry", digest)
		size, _, reader, err := p.getPackageFromRegistry(ctx, digest, repository, path)
		return size, reader, err
	}

	// otherwise try to find digest in the cache
	size, cacheReader, err := p.cache.Get(digest)
	if err == nil {
		return size, cacheReader, nil
	}
	// if any error other than item in the cache not found, get digest directly from the registry
	if !errors.Is(err, cache.ErrEntryNotFound) {
		p.logger.Errorf("Get package from cache: %v", err)
		size, _, reader, err := p.getPackageFromRegistry(ctx, digest, repository, path)
		return size, reader, err
	}

	// if digest is not found in the cache, get digest from registry and add digest to the cache
	size, layerDigest, registryReader, err := p.getPackageFromRegistry(ctx, digest, repository, path)
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

	// asynchronously copy registry package to the cache
	go func() {
		defer registryReader.Close()
		defer pipeWriter.Close()

		err := p.cache.Set(digest, layerDigest, teeReader)
		if err == nil {
			return
		}
		// if cache set returns error, log it and directly copy content from registryReader to pipeWriter
		p.logger.Error(err.Error())
		// Copy remaining data to pipe
		_, err = io.Copy(pipeWriter, registryReader)
		if err != nil {
			p.logger.Error(err.Error())
		}
	}()

	return size, pipeReader, nil
}

func (p *Proxy) getPackageFromRegistry(ctx context.Context, digest string, repository string, path string) (int64, string, io.ReadCloser, error) {
	registryConfig, err := p.getter.Get(repository)
	if err != nil {
		return 0, "", nil, err
	}
	registryConfig.SignCheck = p.config.SignCheck

	size, layerDigest, registryReader, err := p.registryClient.GetPackage(ctx, p.logger, registryConfig, digest, path)
	if err != nil {
		return 0, "", nil, err
	}
	return size, layerDigest, registryReader, nil
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

func normalizeBootstrapClusterUUID(clusterUUID string) string {
	return strings.Trim(strings.TrimSpace(clusterUUID), "/")
}

func extractTarGzFile(reader io.Reader, fileName string) ([]byte, error) {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
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
