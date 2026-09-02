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

package drift

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
	"k8s.io/client-go/metadata"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/nelm"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// monitorTracer identifies tracing spans emitted by the resource monitor.
	monitorTracer = "nelm-monitor"

	// scanInterval defines how often the monitor checks for absent resources.
	scanInterval = 4 * time.Minute

	// workerNumber limits concurrent Kubernetes API requests per monitor.
	workerNumber = 5

	// resourceListTimeout bounds one metadata list request.
	resourceListTimeout = 30 * time.Second
)

// ErrAbsentManifest is returned when one or more expected resources are missing from the cluster.
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

	name        string                              // Helm release name
	namespace   string                              // Release namespace
	rendered    string                              // rendered manifest YAML (cleared after parsing to save memory)
	resources   map[resourceKey]map[string]struct{} // expected resources grouped by API endpoint and namespace
	initialized bool                                // resources were successfully resolved through discovery

	nelm       *nelm.Client
	kubeClient resourceClient

	logger *log.Logger
}

// resourceClient resolves API resources and provides direct metadata access.
type resourceClient interface {
	APIResource(apiVersion, kind string) (*metav1.APIResource, error)
	Metadata() metadata.Interface
}

// resourceKey identifies one Kubernetes list endpoint and its namespace scope.
type resourceKey struct {
	GVR       schema.GroupVersionResource
	Namespace string
}

// newMonitor creates a resource monitor without starting its event loop.
func newMonitor(kubeClient resourceClient, nelm *nelm.Client, namespace, name, rendered string, logger *log.Logger) *resourcesMonitor {
	return &resourcesMonitor{
		wg:   new(sync.WaitGroup),
		once: sync.Once{},

		namespace: namespace,
		name:      name,
		rendered:  rendered,
		resources: make(map[resourceKey]map[string]struct{}),

		kubeClient: kubeClient,
		nelm:       nelm,

		logger: logger.Named(fmt.Sprintf("monitor.%s", name)),
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
	ctx, span := otel.Tracer(monitorTracer).Start(ctx, "checkResources")
	defer span.End()

	span.SetAttributes(attribute.String("name", m.name))
	span.SetAttributes(attribute.Int("resources", len(m.resources)))

	m.logger.Debug("check resources")

	// Lazy initialization: parse manifest on first check (mutex protected)
	m.mtx.Lock()
	if !m.initialized {
		if err := m.buildResourcesMap(); err != nil {
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

// buildResourcesMap resolves rendered objects and groups their names by API endpoint and namespace.
func (m *resourcesMonitor) buildResourcesMap() error {
	objs, err := m.parseManifest(m.rendered)
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	m.logger.Debug("build namespaced gvk", slog.Int("parsed", len(objs)))
	resources := make(map[resourceKey]map[string]struct{})

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

		apiResource, err := m.kubeClient.APIResource(obj.APIVersion, obj.Kind)
		if err != nil {
			return fmt.Errorf("resolve resource %s %s: %w", obj.APIVersion, obj.Kind, err)
		}

		groupVersion, err := schema.ParseGroupVersion(obj.APIVersion)
		if err != nil {
			return fmt.Errorf("parse apiVersion %q: %w", obj.APIVersion, err)
		}

		namespace := obj.Namespace
		switch {
		case !apiResource.Namespaced:
			namespace = ""
		case namespace == "":
			namespace = m.namespace
		}

		key := resourceKey{
			GVR: schema.GroupVersionResource{
				Group:    groupVersion.Group,
				Version:  groupVersion.Version,
				Resource: apiResource.Name,
			},
			Namespace: namespace,
		}
		if resources[key] == nil {
			resources[key] = make(map[string]struct{})
		}

		resources[key][name] = struct{}{}
	}

	m.resources = resources
	m.initialized = true
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

		if obj.GetObjectKind().GroupVersionKind().Empty() {
			return nil, errors.New("object has no gvk")
		}

		res = append(res, obj)
	}

	return res, nil
}

// checkResource checks if all expected resources at one API endpoint are present in the cluster.
// Returns ErrAbsentManifest if any expected resource is missing.
func (m *resourcesMonitor) checkResource(ctx context.Context, res resourceKey) error {
	ctx, span := otel.Tracer(monitorTracer).Start(ctx, "checkResource")
	defer span.End()

	span.SetAttributes(attribute.String("name", m.name))
	span.SetAttributes(attribute.String("namespace", res.Namespace))
	span.SetAttributes(attribute.String("gvr", res.GVR.String()))

	// Early exit if context was already canceled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	m.logger.Debug("check resource",
		slog.String("namespace", res.Namespace),
		slog.String("gvr", res.GVR.String()))

	objects, err := m.listResources(ctx, res)
	if err != nil {
		return fmt.Errorf("list resources: %w", err)
	}

	span.SetAttributes(attribute.Int("resources", len(objects)))
	m.logger.Debug("found resources",
		slog.Int("resources", len(objects)),
		slog.String("namespace", res.Namespace),
		slog.String("gvr", res.GVR.String()))

	// Check if each expected resource name exists in the cluster
	for obj := range m.resources[res] {
		if _, ok := objects[obj]; !ok {
			return ErrAbsentManifest
		}
	}

	return nil
}

// listResources directly lists resource metadata in the requested namespace scope.
// Returns a set of resource names currently present in the cluster.
func (m *resourcesMonitor) listResources(ctx context.Context, res resourceKey) (map[string]struct{}, error) {
	ctx, cancel := context.WithTimeout(ctx, resourceListTimeout)
	defer cancel()

	resource := m.kubeClient.Metadata().Resource(res.GVR)
	var (
		objList *metav1.PartialObjectMetadataList
		err     error
	)
	if res.Namespace == "" {
		objList, err = resource.List(ctx, metav1.ListOptions{})
	} else {
		objList, err = resource.Namespace(res.Namespace).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("list metadata for %s in namespace %q: %w", res.GVR, res.Namespace, err)
	}

	// Convert to a set of names for fast lookup
	objects := make(map[string]struct{}, len(objList.Items))
	for _, obj := range objList.Items {
		objects[obj.GetName()] = struct{}{}
	}

	return objects, nil
}
