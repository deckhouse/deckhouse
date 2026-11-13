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

package kuberbacauth

import (
	"errors"
	"fmt"
	"log/slog"
	"path"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// DefaultTokenCacheTTL is how long a successful TokenReview is reused.
	DefaultTokenCacheTTL = 2 * time.Minute
	// DefaultAllowCacheTTL is how long an allowing SubjectAccessReview is reused.
	DefaultAllowCacheTTL = 5 * time.Minute
	// DefaultDenyCacheTTL is how long a denying review is reused, kept short so that a granted role applies quickly.
	DefaultDenyCacheTTL = 30 * time.Second
	// DefaultStaleCacheTTL matches --stale-cache-interval of the kube-rbac-proxy sidecars in Deckhouse modules.
	DefaultStaleCacheTTL = 90 * time.Minute
	// DefaultReviewTimeout bounds the checks of one request with their retries, like the 5 second client
	// timeout of kube-rbac-proxy in Deckhouse, and leaves time to serve a cached decision within a 10 second scrape.
	DefaultReviewTimeout = 5 * time.Second
	// DefaultCacheSize limits the number of cached tokens and decisions each.
	DefaultCacheSize = 4096
)

// DefaultRetryBackoff retries a failed review for about 4 seconds, within DefaultReviewTimeout.
var DefaultRetryBackoff = wait.Backoff{
	Duration: 500 * time.Millisecond,
	Factor:   1.5,
	Jitter:   0.2,
	Steps:    5,
}

// ResourceAttributes is the resource a client must have access to.
// The fields and their JSON names follow resourceAttributes in the kube-rbac-proxy configuration.
type ResourceAttributes struct {
	Namespace   string `json:"namespace,omitempty"`
	APIGroup    string `json:"apiGroup,omitempty"`
	APIVersion  string `json:"apiVersion,omitempty"`
	Resource    string `json:"resource,omitempty"`
	Subresource string `json:"subresource,omitempty"`
	Name        string `json:"name,omitempty"`
}

// Rule protects requests whose path matches Path with a check of access to ResourceAttributes.
// Path follows http.ServeMux patterns without a method or a host: "/metrics" matches only itself,
// "/metrics/" matches the whole subtree, and the longest pattern wins.
type Rule struct {
	Path               string             `json:"path"`
	ResourceAttributes ResourceAttributes `json:"resourceAttributes"`
}

// Config configures the Middleware. Only Rules is required.
type Config struct {
	Rules []Rule `json:"rules"`

	// ExcludePaths are served without authentication, for example probes.
	// Each entry is a path.Match pattern matched against the whole request path.
	ExcludePaths []string `json:"excludePaths,omitempty"`

	// Cache lifetimes. Zero selects the default. A negative StaleCacheTTL disables the stale cache.
	TokenCacheTTL time.Duration `json:"-"`
	AllowCacheTTL time.Duration `json:"-"`
	DenyCacheTTL  time.Duration `json:"-"`
	// StaleCacheTTL is how long a past decision is still used while the API server cannot be reached.
	StaleCacheTTL time.Duration `json:"-"`
	CacheSize     int           `json:"-"`

	// RetryBackoff is used for failed reviews. Nil selects DefaultRetryBackoff.
	// Answers that a retry cannot change, such as Forbidden, are not retried.
	RetryBackoff *wait.Backoff `json:"-"`
	// ReviewTimeout bounds the checks of one request, both reviews with their retries. Zero selects the default.
	ReviewTimeout time.Duration `json:"-"`

	// Logger defaults to slog.Default().
	Logger *slog.Logger `json:"-"`
}

func (c *Config) validate() error {
	if len(c.Rules) == 0 {
		return errors.New("at least one rule is required")
	}
	seen := make(map[string]struct{}, len(c.Rules))
	for i, r := range c.Rules {
		if r.Path == "" || r.Path[0] != '/' {
			return fmt.Errorf("rules[%d]: path %q must start with /", i, r.Path)
		}
		if _, ok := seen[r.Path]; ok {
			return fmt.Errorf("rules[%d]: duplicate path %q", i, r.Path)
		}
		seen[r.Path] = struct{}{}
		if r.ResourceAttributes.Resource == "" {
			return fmt.Errorf("rules[%d]: resourceAttributes.resource is required", i)
		}
	}
	for i, p := range c.ExcludePaths {
		if _, err := path.Match(p, "/"); err != nil {
			return fmt.Errorf("excludePaths[%d]: %w", i, err)
		}
	}
	return nil
}

func (c *Config) withDefaults() Config {
	out := *c
	if out.TokenCacheTTL == 0 {
		out.TokenCacheTTL = DefaultTokenCacheTTL
	}
	if out.AllowCacheTTL == 0 {
		out.AllowCacheTTL = DefaultAllowCacheTTL
	}
	if out.DenyCacheTTL == 0 {
		out.DenyCacheTTL = DefaultDenyCacheTTL
	}
	if out.StaleCacheTTL == 0 {
		out.StaleCacheTTL = DefaultStaleCacheTTL
	}
	if out.StaleCacheTTL < 0 {
		out.StaleCacheTTL = 0
	}
	if out.ReviewTimeout <= 0 {
		out.ReviewTimeout = DefaultReviewTimeout
	}
	if out.CacheSize <= 0 {
		out.CacheSize = DefaultCacheSize
	}
	if out.RetryBackoff == nil {
		b := DefaultRetryBackoff
		out.RetryBackoff = &b
	}
	if out.Logger == nil {
		out.Logger = slog.Default()
	}
	return out
}
