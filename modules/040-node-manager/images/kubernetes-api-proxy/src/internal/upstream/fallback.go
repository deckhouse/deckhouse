/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package upstream

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type FallbackListOption func(l *FallbackList) error

func WithUpstreamsFromArgs(upstreamsFromArgs []string) FallbackListOption {
	return func(l *FallbackList) error {
		upstreams := make([]*Upstream, 0, len(upstreamsFromArgs))
		for _, upstream := range upstreamsFromArgs {
			upstreams = append(upstreams, NewUpstream(upstream))
		}

		for _, upstream := range upstreams {
			l.nodes = append(l.nodes, node{
				backend: upstream,
			})
		}

		return nil
	}
}

func WithFileWatcher(filePath string) FallbackListOption {
	return func(l *FallbackList) error {
		watcher := newFileWatcher(
			filePath,
			func(upstreams []*Upstream) {
				l.Reconcile(upstreams, false)
			},
		)

		l.watcher = watcher

		return nil
	}
}

func WithFallbackLogger(logger *log.Logger) FallbackListOption {
	return func(l *FallbackList) error {
		l.logger = logger

		return nil
	}
}

// FallbackList represents a list of upstreams that can be populated from a file or flag
type FallbackList struct {
	*List
	watcher *fileWatcher
}

// NewFallbackList creates a new FallbackList instance
func NewFallbackList(
	options ...FallbackListOption,
) (*FallbackList, error) {
	fb := &FallbackList{
		List: &List{
			cmp: func(_, _ *Upstream) bool {
				return true
			},
			current: -1,
		},
	}

	for _, opt := range options {
		if err := opt(fb); err != nil {
			return nil, fmt.Errorf("failed to prepare fallback list: %w", err)
		}
	}

	return fb, nil
}

// Start starts the fallback list processing
func (fb *FallbackList) Start(ctx context.Context) error {
	if fb.watcher != nil {
		go fb.watcher.Start(ctx)
	}

	return nil
}

// Shutdown stops the fallback list processing
func (fb *FallbackList) Shutdown() {
	if fb.watcher != nil {
		fb.watcher.Stop()
	}
}

func (fb *FallbackList) Reconcile(newList []*Upstream, triggerWrite bool) {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	if newList == nil {
		newList = []*Upstream{}
	}

	newNodes := make([]node, 0, len(newList))
	for _, upstream := range newList {
		newNodes = append(newNodes, node{
			backend: upstream,
			score:   initialScore,
			tier:    fb.initTier,
		})
	}

	if fb.logger != nil {
		fb.logger.Debug(
			"fallback list: reconciled new fallback options",
			slog.Int("upstream_count", len(newNodes)),
		)
	}

	fb.nodes = newNodes

	if fb.watcher != nil {
		if !triggerWrite {
			return
		}

		if fb.logger != nil {
			fb.logger.Debug("fallback list: trigger file watcher to write")
		}

		fb.watcher.triggerChangedInside(newList)
	}
}

func (fb *FallbackList) UpdateFromList(list *List) {
	list.mu.RLock()
	nodes := list.nodes
	list.mu.RUnlock()

	upstreams := make([]*Upstream, 0, len(nodes))
	for _, node := range nodes {
		upstreams = append(upstreams, node.backend)
	}

	if len(upstreams) == 0 {
		return
	}

	fb.Reconcile(upstreams, true)
}

func (fb *FallbackList) Pick() (*Upstream, error) {
	fb.mu.RLock()
	defer fb.mu.RUnlock()

	if len(fb.nodes) == 0 {
		return nil, fmt.Errorf("no upstreams available")
	}

	selectedNode := (fb.current + 1) % len(fb.nodes)
	fb.current = selectedNode
	return fb.nodes[selectedNode].backend, nil
}

func (fb *FallbackList) PickAsString() (string, error) {
	upstream, err := fb.Pick()
	if err != nil {
		return "", err
	}

	return upstream.Address(), nil
}
