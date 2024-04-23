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
	"io"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/pkg/errors"

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

// Serve
// logAsDebugServeAddress - for dhctl we should hide "tarting packages proxy listener" message
// because it can break logboek output because Serve run in standalone goroutine
// we cannot use chan here because we do not want to complicate logic
func (p *Proxy) Serve(logAsDebugServeAddress bool) {
	http.HandleFunc("/package", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" && r.Method != "GET" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		digest := r.URL.Query().Get("digest")

		if digest == "" {
			http.Error(w, "missing digest", http.StatusBadRequest)
			return
		}

		repository := r.URL.Query().Get("repository")
		if repository == registry.DefaultRepository {
			p.logger.Infof("%s digest from main repository request received\n", digest)
		} else {
			p.logger.Infof("%s digest from repository %s request received\n", digest, repository)
		}

		size, packageReader, err := p.getPackage(r.Context(), digest, repository)
		if packageReader != nil {
			defer packageReader.Close()
		}
		if err != nil {
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
	})

	logServe := p.logger.Infof
	if logAsDebugServeAddress {
		logServe = p.logger.Debugf
	}
	logServe("Starting packages proxy listener: %s\n\n", p.listener.Addr())

	if err := p.server.Serve(p.listener); err != nil && err != http.ErrServerClosed {
		p.logger.Error(err)
	}
}

func (p *Proxy) Stop() {
	p.logger.Infof("graceful shutdown listener: %s", p.listener.Addr())
	err := p.server.Shutdown(context.Background())
	if err != nil && err != http.ErrServerClosed {
		p.logger.Error(err)
		os.Exit(1)
	}
}

func (p *Proxy) getPackage(ctx context.Context, digest string, repository string) (int64, io.ReadCloser, error) {
	// if cache is nil, return digest directly from registry
	if p.cache == nil {
		return p.getPackageFromRegistry(ctx, digest, repository)
	}

	// otherwise try to find digest in the cache
	size, cacheReader, err := p.cache.Get(digest)
	if err == nil {
		return size, cacheReader, nil
	}
	// if any error other than item in the cache not found, get digest directly from the registry
	if !errors.Is(err, cache.ErrEntryNotFound) {
		p.logger.Errorf("Get package from cache: %v", err)
		return p.getPackageFromRegistry(ctx, digest, repository)
	}

	// if digest is not found in the cache, get digest from registry and add digest to the cache
	size, registryReader, err := p.getPackageFromRegistry(ctx, digest, repository)
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

		err := p.cache.Set(digest, size, teeReader)
		if err == nil {
			return
		}
		// if cache set returns error, log it and directly copy content from registryReader to pipeWriter
		p.logger.Error(err)
		// Copy remaining data to pipe
		_, err = io.Copy(pipeWriter, registryReader)
		if err != nil {
			p.logger.Error(err)
		}
	}()

	return size, pipeReader, nil
}

func (p *Proxy) getPackageFromRegistry(ctx context.Context, digest string, repository string) (int64, io.ReadCloser, error) {
	registryConfig, err := p.getter.Get(repository)
	if err != nil {
		return 0, nil, err
	}

	size, registryReader, err := p.registryClient.GetPackage(ctx, registryConfig, digest)
	if err != nil {
		return 0, nil, err
	}
	return size, registryReader, nil
}

type ProxyOption func(*Proxy)

func WithCache(cache cache.Cache) ProxyOption {
	return func(p *Proxy) {
		p.cache = cache
	}
}
