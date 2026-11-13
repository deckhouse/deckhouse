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
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"golang.org/x/sync/singleflight"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// StaticSources are the registries of the public site, where there is no
// cluster to ask: a fixed list of repositories whose tags are the modules,
// with the credentials of a docker config. A module in several repositories is taken from the first one that
// has it.
type StaticSources struct {
	sources []Source
	clients *Clients
	logger  *log.Logger

	now   func() time.Time
	group singleflight.Group

	mu sync.Mutex
	// first is the index of the first source listing the module, as of the
	// last Modules; failed are the sources that listing could not read.
	first    map[string]int
	failed   []int
	listedAt time.Time
}

// staticListTTL is how old the listing ForModule goes by may get: a replica
// that does not read the registries for the search (a follower) lists them
// on a request instead, at most this often.
const staticListTTL = 15 * time.Minute

const (
	// staticListRetry is how soon ForModule lists again when no registry
	// answered.
	staticListRetry = time.Minute
	// staticListTimeout bounds a listing ForModule starts.
	staticListTimeout = time.Minute
)

// NewStaticSources returns the sources of repos, each "<host>/<path>" or
// "http://<host>/<path>"; spaces around an entry are dropped, an empty,
// unusable or repeated repository is an error.
// dockerCfg is the docker config JSON (not encoded); empty means anonymous
// access.
func NewStaticSources(repos []string, dockerCfg []byte, clients *Clients, logger *log.Logger) (*StaticSources, error) {
	if len(repos) == 0 {
		return nil, errors.New("no registry repositories")
	}

	encoded := ""
	if len(dockerCfg) > 0 {
		encoded = base64.StdEncoding.EncodeToString(dockerCfg)
	}

	s := &StaticSources{clients: clients, logger: logger, now: time.Now, first: make(map[string]int)}
	seen := make(map[string]bool)
	for _, repo := range repos {
		repo = strings.TrimSpace(repo)
		if repo == "" {
			return nil, errors.New("an empty repository in the list")
		}
		src := Source{Name: repo, Repo: repo, DockerCfg: encoded}
		// A registry without TLS, such as a local one, is given as http://.
		var nameOpts []name.Option
		if plain, ok := strings.CutPrefix(repo, "http://"); ok {
			src.Repo, src.Scheme = plain, "HTTP"
			nameOpts = append(nameOpts, name.Insecure)
		}
		// go-containerregistry takes "https:" for the name of a registry.
		if strings.Contains(src.Repo, "://") {
			return nil, fmt.Errorf("repository %q: <host>/<path> or http://<host>/<path> is expected", repo)
		}
		// Reject a repository that can not be read at the start, not at the
		// first reader: one that never lists counts as not answering, and
		// ForModule then returns it for every unknown module instead of
		// ErrNoSource.
		parsed, err := name.NewRepository(strings.TrimSuffix(src.Repo, "/"), nameOpts...)
		if err != nil {
			return nil, fmt.Errorf("repository %q: %w", repo, err)
		}
		if seen[parsed.String()] {
			return nil, fmt.Errorf("repository %q is listed twice", repo)
		}
		seen[parsed.String()] = true
		if _, err := clients.For(src); err != nil {
			return nil, fmt.Errorf("repository %q: %w", repo, err)
		}
		s.sources = append(s.sources, src)
	}

	return s, nil
}

// Modules lists the modules of every repository. None is enabled: the site
// documents what the registries publish. A repository that can not be
// listed keeps the modules it had at the last listing, so they neither leave
// the search nor move to another repository; the error is returned when no
// repository can be listed.
func (s *StaticSources) Modules(ctx context.Context) ([]Module, error) {
	s.mu.Lock()
	prev := s.first
	s.mu.Unlock()

	listed := make([][]string, len(s.sources))
	failed := make(map[int]bool)
	var errs []error
	for i, src := range s.sources {
		client, err := s.clients.For(src)
		if err == nil {
			listed[i], err = client.ListModules(ctx)
		}
		if err != nil {
			s.logger.Warn("list modules of the registry", slog.String("repository", src.Repo), log.Err(err))
			errs = append(errs, fmt.Errorf("repository %s: %w", src.Repo, err))
			failed[i] = true
		}
	}
	failedList := make([]int, 0, len(failed))
	for i := range s.sources {
		if failed[i] {
			failedList = append(failedList, i)
		}
	}
	if len(failed) == len(s.sources) {
		// The last listing stays; a request lists again after a while.
		s.mu.Lock()
		s.failed, s.listedAt = failedList, s.now().Add(staticListRetry-staticListTTL)
		s.mu.Unlock()
		return nil, errors.Join(errs...)
	}
	for name, i := range prev {
		if failed[i] {
			listed[i] = append(listed[i], name)
		}
	}

	first := make(map[string]int)
	var modules []Module
	for i, names := range listed {
		for _, name := range names {
			if _, ok := first[name]; !ok {
				first[name] = i
				modules = append(modules, Module{Name: name})
			}
		}
	}

	s.mu.Lock()
	s.first, s.failed, s.listedAt = first, failedList, s.now()
	s.mu.Unlock()

	return modules, nil
}

// ForModule returns the first repository that lists the module, listing
// them first when the last listing is older than staticListTTL. A module no
// repository listed may be in one that did not answer: those are returned,
// in order; with all of them listed the error wraps ErrNoSource.
func (s *StaticSources) ForModule(ctx context.Context, module string) ([]Source, error) {
	s.mu.Lock()
	old := s.now().Sub(s.listedAt) > staticListTTL
	s.mu.Unlock()
	if old {
		// The listing is shared by the requests waiting for it, so a reader
		// that goes away does not cancel it. The error is logged by Modules;
		// the last listing stays.
		_, _, _ = s.group.Do("list", func() (any, error) {
			lctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), staticListTimeout)
			defer cancel()
			return s.Modules(lctx)
		})
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if i, ok := s.first[module]; ok {
		return []Source{s.sources[i]}, nil
	}
	if len(s.failed) > 0 {
		out := make([]Source, 0, len(s.failed))
		for _, i := range s.failed {
			out = append(out, s.sources[i])
		}
		return out, nil
	}

	return nil, fmt.Errorf("module %s: %w", module, ErrNoSource)
}
