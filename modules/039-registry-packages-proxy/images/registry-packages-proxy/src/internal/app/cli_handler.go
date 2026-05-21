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

package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	pkgCache "github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/cache"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/proxy"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// cliImagesPathPrefix is the URL prefix mounted on the internal proxy mux for CLI/plugin downloads.
// Paths under it look like:
//
//	/v1/images/<image>/tags                 -> list tags
//	/v1/images/<image>/tags/<tag>           -> download last layer as tar.gz
//
// where <image> must match the allowlist (deckhouse-cli or deckhouse-cli/plugins/<plugin>).
const cliImagesPathPrefix = "/v1/images/"

// CLIHandler exposes a small subset of registry operations for the deckhouse-cli use case:
// listing image tags and pulling the last layer of an image tag as a gzip stream.
type CLIHandler struct {
	logger         *log.Logger
	getter         registry.ClientConfigGetter
	registryClient registry.Client
	cache          pkgCache.Cache
	signCheck      bool
}

// CLIHandlerOptions configures a CLIHandler. Cache may be nil to disable caching.
type CLIHandlerOptions struct {
	Logger             *log.Logger
	ClientConfigGetter registry.ClientConfigGetter
	RegistryClient     registry.Client
	Cache              pkgCache.Cache
	SignCheck          bool
}

func NewCLIHandler(opts CLIHandlerOptions) *CLIHandler {
	return &CLIHandler{
		logger:         opts.Logger,
		getter:         opts.ClientConfigGetter,
		registryClient: opts.RegistryClient,
		cache:          opts.Cache,
		signCheck:      opts.SignCheck,
	}
}

// Register mounts CLI download endpoints on the given mux. Pass http.DefaultServeMux to register
// on the same mux used by the rest of the server.
func (h *CLIHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc(cliImagesPathPrefix, h.serveHTTP)
}

func (h *CLIHandler) serveHTTP(w http.ResponseWriter, r *http.Request) {
	imagePath, action, tag, err := parseCLIPath(r.URL.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if !isAllowedCLIImagePath(imagePath) {
		h.logger.Warnf("CLI request for disallowed image path %q from %s", imagePath, requestIP(r))
		http.NotFound(w, r)
		return
	}

	switch action {
	case actionListTags:
		h.handleListTags(w, r, imagePath)
	case actionPullTag:
		h.handlePullTag(w, r, imagePath, tag)
	default:
		http.NotFound(w, r)
	}
}

type cliAction int

const (
	actionUnknown cliAction = iota
	actionListTags
	actionPullTag
)

// parseCLIPath splits an HTTP path of the form
//
//	/v1/images/<image-path>/tags
//	/v1/images/<image-path>/tags/<tag>
//
// into its components. <image-path> may contain slashes; we anchor on the final /tags segment.
func parseCLIPath(urlPath string) (imagePath string, action cliAction, tag string, err error) {
	if !strings.HasPrefix(urlPath, cliImagesPathPrefix) {
		return "", actionUnknown, "", errors.New("not a CLI path")
	}
	rest := strings.TrimPrefix(urlPath, cliImagesPathPrefix)
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return "", actionUnknown, "", errors.New("missing image path")
	}

	const sep = "/tags"
	idx := strings.LastIndex(rest, sep)
	if idx < 0 {
		return "", actionUnknown, "", errors.New("missing tags segment")
	}

	imagePath = rest[:idx]
	if imagePath == "" {
		return "", actionUnknown, "", errors.New("empty image path")
	}

	suffix := rest[idx+len(sep):]
	switch {
	case suffix == "" || suffix == "/":
		return imagePath, actionListTags, "", nil
	case strings.HasPrefix(suffix, "/"):
		tag = strings.Trim(suffix[1:], "/")
		if tag == "" || strings.Contains(tag, "/") {
			return "", actionUnknown, "", errors.New("invalid tag")
		}
		return imagePath, actionPullTag, tag, nil
	default:
		return "", actionUnknown, "", errors.New("unexpected path suffix")
	}
}

// isAllowedCLIImagePath enforces the allowlist:
//   - deckhouse-cli
//   - deckhouse-cli/plugins/<single-segment>
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

func (h *CLIHandler) handleListTags(w http.ResponseWriter, r *http.Request, imagePath string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientIP := requestIP(r)
	h.logger.Infof("CLI list-tags for image %q from client %s", imagePath, clientIP)

	cfg, err := h.getter.Get(registry.DefaultRepository)
	if err != nil {
		h.logger.Errorf("get registry config: %v", err)
		http.Error(w, "registry config unavailable", http.StatusInternalServerError)
		return
	}
	if cfg == nil {
		http.Error(w, "registry config unavailable", http.StatusInternalServerError)
		return
	}
	cfg.SignCheck = h.signCheck

	tags, err := h.registryClient.ListTags(r.Context(), h.logger, cfg, imagePath)
	if err != nil {
		if errors.Is(err, registry.ErrPackageNotFound) {
			http.Error(w, "image not found", http.StatusNotFound)
			return
		}
		h.logger.Errorf("list tags for %q: %v", imagePath, err)
		http.Error(w, "failed to list tags", http.StatusBadGateway)
		return
	}

	body, err := json.Marshal(cliTagsResponse{Name: imagePath, Tags: tags})
	if err != nil {
		h.logger.Errorf("marshal tags: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = w.Write(body)
}

func (h *CLIHandler) handlePullTag(w http.ResponseWriter, r *http.Request, imagePath, tag string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientIP := requestIP(r)
	h.logger.Infof("CLI pull image %q tag %q from client %s", imagePath, tag, clientIP)

	cfg, err := h.getter.Get(registry.DefaultRepository)
	if err != nil {
		h.logger.Errorf("get registry config: %v", err)
		http.Error(w, "registry config unavailable", http.StatusInternalServerError)
		return
	}
	if cfg == nil {
		http.Error(w, "registry config unavailable", http.StatusInternalServerError)
		return
	}
	cfg.SignCheck = h.signCheck

	manifestDigest, err := h.registryClient.ResolveTag(r.Context(), h.logger, cfg, imagePath, tag)
	if err != nil {
		if errors.Is(err, registry.ErrPackageNotFound) {
			http.Error(w, "tag not found", http.StatusNotFound)
			return
		}
		h.logger.Errorf("resolve tag %q for %q: %v", tag, imagePath, err)
		http.Error(w, "failed to resolve tag", http.StatusBadGateway)
		return
	}

	size, reader, err := proxy.GetPackageCached(
		r.Context(),
		h.logger,
		h.getter,
		h.registryClient,
		h.cache,
		manifestDigest,
		registry.DefaultRepository,
		imagePath,
		h.signCheck,
	)
	if reader != nil {
		defer reader.Close()
	}
	if err != nil {
		if errors.Is(err, registry.ErrPackageNotFound) {
			http.Error(w, "package not found", http.StatusNotFound)
			return
		}
		h.logger.Errorf("get package for %q@%s: %v", imagePath, manifestDigest, err)
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
		h.logger.Errorf("stream package for %q@%s: %v", imagePath, manifestDigest, err)
	}
}

type cliTagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func requestIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-Ip"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}
