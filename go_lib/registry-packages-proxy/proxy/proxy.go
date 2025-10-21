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
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"

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

func NewProxy(server *http.Server,
	listener net.Listener,
	clientConfigGetter registry.ClientConfigGetter,
	logger log.Logger,
	registryClient registry.Client, opts ...ProxyOption) *Proxy {

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
        if r.Method != "HEAD" && r.Method != "GET" {
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

		if r.Method == "HEAD" {
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
