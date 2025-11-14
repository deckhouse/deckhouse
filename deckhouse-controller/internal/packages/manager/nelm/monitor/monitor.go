// Copyright 2025 Flant JSC
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

package monitor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/nelm"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	monitorTracer = "nelm-monitor"

	// scanInterval defines how often the monitor checks for absent resources
	scanInterval = 4 * time.Minute

	// default number of workers
	workerNumber = 5
)

// ErrAbsentManifest is returned when one or more expected resources are missing from the cluster
var ErrAbsentManifest = errors.New("absent manifest")

// AbsentCallback is invoked when absent resources are detected
type AbsentCallback func(name string)

// resourcesMonitor periodically checks if all Helm release resources exist in the cluster
type resourcesMonitor struct {
	ctx    context.Context
	cancel context.CancelFunc

	mtx        sync.Mutex
	pauseCount atomic.Int32 // reference counter for pause/resume operations
	once       sync.Once    // ensures Start() goroutine is created only once
	wg         *sync.WaitGroup

	name      string                                // Helm release name
	namespace string                                // Release namespace
	rendered  string                                // rendered manifest YAML (cleared after parsing to save memory)
	resources map[namespacedGVK]map[string]struct{} // expected resources: GVK+namespace -> set of resource names

	nelm  *nelm.Client
	cache runtimecache.Cache

	logger *log.Logger
}

// namespacedGVK uniquely identifies a resource type within a namespace
type namespacedGVK struct {
	gvk       schema.GroupVersionKind
	namespace string // empty for cluster-scoped resources
}

func newMonitor(cache runtimecache.Cache, nelm *nelm.Client, namespace, name, rendered string, logger *log.Logger) *resourcesMonitor {
	return &resourcesMonitor{
		wg:   new(sync.WaitGroup),
		once: sync.Once{},

		namespace: namespace,
		name:      name,
		rendered:  rendered,
		resources: make(map[namespacedGVK]map[string]struct{}),

		cache: cache,
		nelm:  nelm,

		logger: logger.Named(fmt.Sprintf("monitor-%s", name)),
	}
}

// Stop gracefully shuts down the resources monitor.
// It cancels the handler's context to signal the event loop to terminate,
// then waits for the goroutine to finish processing.
//
// This method is safe to call even if Start() was never called or called multiple times,
// as sync.Once ensures the goroutine is created at most once.
//
// Blocks until the event processing goroutine exits.
func (m *resourcesMonitor) Stop() {
	m.logger.Info("stop loop")

	if m.cancel != nil {
		m.cancel()
		m.wg.Wait()
	}
}

// Pause prevents execution of absent callback.
// Multiple goroutines can call Pause() concurrently; each call increments the pause counter.
// The monitor will remain paused until Resume() is called an equal number of times.
func (m *resourcesMonitor) Pause() {
	m.logger.Info("pause loop")

	m.pauseCount.Add(1)
}

// Resume allows execution of absent callback.
// Decrements the pause counter. The monitor resumes only when the counter reaches zero.
// Safe to call even if pause counter is already zero (no-op).
func (m *resourcesMonitor) Resume() {
	// lock to avoid negative counter
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.logger.Info("resume loop")

	if m.pauseCount.Load() > 0 {
		m.pauseCount.Add(-1)
	}
}

// Start creates a timer and checks if all deployed manifests are present in the cluster.
func (m *resourcesMonitor) Start(ctx context.Context, callback AbsentCallback) {
	m.once.Do(func() {
		m.logger.Info("start loop")

		// Create cancellable context before starting goroutine
		m.ctx, m.cancel = context.WithCancel(ctx)

		// Increment WaitGroup before goroutine starts
		m.wg.Add(1)

		go func() {
			// Ensure WaitGroup is decremented when goroutine exits
			defer m.wg.Done()

			rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
			randDelay := time.Second * time.Duration(rnd.Int31n(60))

			timer := time.NewTicker(scanInterval + randDelay)
			defer timer.Stop()

			for {
				select {
				case <-m.ctx.Done():
					m.logger.Info("loop stopped, context cancelled")
					return

				case <-timer.C:
					if m.pauseCount.Load() > 0 {
						m.logger.Info("loop paused")
						continue
					}

					// check release status
					_, status, err := m.nelm.LastStatus(m.ctx, m.namespace, m.name)
					if err != nil {
						m.logger.Error("failed to get helm release status", log.Err(err))
						continue
					}

					if status != "deployed" {
						m.logger.Warn("helm release is not deployed, skipping")
						continue
					}

					err = m.checkResources(m.ctx)
					if err != nil && !errors.Is(err, ErrAbsentManifest) {
						m.logger.Error("failed to detect absent resources", log.Err(err))
						continue
					}

					if errors.Is(err, ErrAbsentManifest) {
						m.logger.Debug("absent resources detected")
						if callback != nil {
							callback(m.name)
						}
					}
				}
			}
		}()
	})
}

// checkResources checks that all release manifests are present in the cluster.
// On first run, it parses the rendered manifest to build the expected resource index.
// Resource checks are performed in parallel for better performance.
func (m *resourcesMonitor) checkResources(ctx context.Context) error {
	_, span := otel.Tracer(monitorTracer).Start(ctx, "checkResources")
	defer span.End()

	span.SetAttributes(attribute.String("name", m.name))

	m.logger.Debug("check resources")

	// Lazy initialization: parse manifest on first check (mutex protected)
	m.mtx.Lock()
	if len(m.resources) == 0 {
		if err := m.buildNamespacedGVK(); err != nil {
			m.mtx.Unlock()
			return fmt.Errorf("build namespaced gvk: %w", err)
		}
	}
	m.mtx.Unlock()

	// Check all resources in parallel using errgroup
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(workerNumber)

	for res := range m.resources {
		g.Go(func() error {
			return m.checkResource(ctx, res)
		})
	}

	// Wait for all checks to complete
	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

// buildNamespacedGVK parses the rendered manifest and builds an index of expected resources.
// It groups resources by their GVK and namespace, storing the expected resource names.
func (m *resourcesMonitor) buildNamespacedGVK() error {
	objs, err := m.parseManifest(m.rendered)
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	for _, obj := range objs {
		// Skip list kinds rendered by Helm, if any
		if strings.HasSuffix(obj.Kind, "List") {
			continue
		}

		name := obj.GetName()
		if name == "" {
			// Skip resources without names (generateName or templating gaps)
			// Cannot verify existence without a concrete name
			continue
		}

		key := namespacedGVK{gvk: obj.GroupVersionKind(), namespace: obj.GetNamespace()}
		if m.resources[key] == nil {
			m.resources[key] = make(map[string]struct{})
		}

		m.resources[key][name] = struct{}{}
	}

	// Clear rendered manifest to free memory (can be several MB for large releases)
	m.rendered = ""

	return nil
}

// parseManifest parses a multi-document YAML manifest into PartialObjectMetadata.
// Only extracts metadata (name, namespace, GVK), not the full resource spec.
func (m *resourcesMonitor) parseManifest(rendered string) ([]*metav1.PartialObjectMetadata, error) {
	dec := yaml.NewYAMLOrJSONDecoder(strings.NewReader(rendered), 4096)

	var res []*metav1.PartialObjectMetadata
	for {
		obj := new(metav1.PartialObjectMetadata)
		if err := dec.Decode(obj); err != nil {
			if err == io.EOF {
				break
			}

			// Skip empty YAML documents (e.g., standalone '---')
			if strings.Contains(err.Error(), "empty") {
				continue
			}

			return nil, err
		}

		// Skip completely empty objects
		if obj.APIVersion == "" && obj.Kind == "" {
			continue
		}

		gvk := obj.GetObjectKind().GroupVersionKind()
		if gvk.Empty() {
			return nil, errors.New("object has no gvk")
		}

		res = append(res, obj)
	}

	return res, nil
}

// checkResource checks if all expected resources of a given type are present in the cluster.
// Returns ErrAbsentManifest if any expected resource is missing.
func (m *resourcesMonitor) checkResource(ctx context.Context, res namespacedGVK) error {
	_, span := otel.Tracer(monitorTracer).Start(ctx, "checkResource")
	defer span.End()

	span.SetAttributes(attribute.String("name", m.name))
	span.SetAttributes(attribute.String("namespace", res.namespace))
	span.SetAttributes(attribute.String("gvk", res.gvk.String()))

	// Early exit if context was already canceled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	m.logger.Debug("check resource",
		slog.String("namespace", res.namespace),
		slog.String("gvk", res.gvk.String()))

	// List all resources of this type currently in the cluster
	objects, err := m.listResources(ctx, res.namespace, res.gvk)
	if err != nil {
		return fmt.Errorf("list resources: %w", err)
	}

	// Check if each expected resource name exists in the cluster
	for obj := range m.resources[res] {
		if _, ok := objects[obj]; !ok {
			return ErrAbsentManifest
		}
	}

	return nil
}

// listResources lists all resources of the given GVK in a namespace (or cluster-wide if ns is empty).
// Returns a set of resource names currently present in the cluster.
func (m *resourcesMonitor) listResources(ctx context.Context, ns string, gvk schema.GroupVersionKind) (map[string]struct{}, error) {
	objList := new(metav1.PartialObjectMetadataList)

	// Set the List kind for the API request (e.g., DeploymentList for Deployment)
	objList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind + "List",
	})

	var opts []client.ListOption
	if ns != "" {
		// Namespace-scoped resources
		opts = append(opts, client.InNamespace(ns))
	}
	// Empty namespace means cluster-scoped resources (e.g., ClusterRole, CRD)

	if err := m.cache.List(ctx, objList, opts...); err != nil {
		return nil, fmt.Errorf("list objects: %w", err)
	}

	// Convert to a set of names for fast lookup
	res := make(map[string]struct{}, len(objList.Items))
	for _, obj := range objList.Items {
		res[obj.GetName()] = struct{}{}
	}

	return res, nil
}
