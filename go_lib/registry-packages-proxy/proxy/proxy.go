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
	options Options) (*Proxy, error) {
	if options.RegistryClient == nil {
		options.RegistryClient = &registry.DefaultClient{}
	}

	return &Proxy{
		server:         server,
		listener:       listener,
		getter:         clientConfigGetter,
		registryClient: options.RegistryClient,
		cache:          options.Cache,
		logger:         options.Logger,
	}, nil
}

func (p *Proxy) Serve() {
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
		if repository == "" {
			p.logger.Infof("%s digest from main repository request received", digest)
		} else {
			p.logger.Infof("%s digest from repository %s request received", digest, repository)
		}

		size, packageReader, err := p.getPackage(r.Context(), digest, repository)
		if err != nil {
			if errors.Is(err, registry.ErrPackageNotFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer packageReader.Close()

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

	p.logger.Infof("starting listener: %s", p.listener.Addr())
	if err := p.server.Serve(p.listener); err != nil && err != http.ErrServerClosed {
		p.logger.Errorf("http server error: %v", err)
	}
}

func (p *Proxy) Stop() {
	p.logger.Infof("stopping listener: %s", p.listener.Addr())
	err := p.server.Shutdown(context.Background())
	if err != nil && err != http.ErrServerClosed {
		p.logger.Errorf("http server graceful shutdown error: %v", err)
		os.Exit(1)
	}
}

func (p *Proxy) getPackage(ctx context.Context, digest string, repository string) (int64, io.ReadCloser, error) {

	// if cache is nil, return digest directly from registry
	if p.cache == nil {
		registryConfig, err := p.getter.Get(repository)
		if err != nil {
			return 0, nil, err
		}

		size, packageReader, err := p.registryClient.GetPackage(ctx, registryConfig, digest)
		if err != nil {
			return 0, nil, err
		}
		return size, packageReader, nil
	}

	// otherwise try to find digest in the cache
	size, packageReader, cacheErr := p.cache.Get(digest)
	if cacheErr == nil {
		return size, packageReader, nil
	}

	registryConfig, err := p.getter.Get(repository)
	if err != nil {
		return 0, nil, err
	}

	size, packageReader, err = p.registryClient.GetPackage(ctx, registryConfig, digest)
	if err != nil {
		return 0, nil, err
	}

	// if any error other than item in the cache not found, get digest directly from the registry
	if !errors.Is(cacheErr, cache.ErrEntryNotFound) {
		p.logger.Errorf("Get package from cache: %v", cacheErr)
		return size, packageReader, nil
	}

	// Otherwise, get the digest from registry and put them to cache
	pipeReader, pipeWriter := io.Pipe()

	reader := io.TeeReader(packageReader, pipeWriter)

	go func() {
		defer packageReader.Close()

		err := p.cache.Set(digest, size, reader)
		if err != nil {
			defer pipeWriter.Close()

			p.logger.Errorf("Add package to cache: %v", err)

			// Copy remaining data to pipe
			_, err := io.Copy(pipeWriter, packageReader)
			if err != nil {
				p.logger.Errorf("Copy remaining data to pipe: %v", err)
			}
		}
	}()

	return size, pipeReader, nil

}
