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

// Package register wires reconcilers into the manager.
//
// Reconcilers register themselves from an init function, so adding one is a
// single import in internal/register/controllers and cannot be half-done: a
// reconciler that compiles is a reconciler that runs.
package register

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Reconciler is what a registered controller must implement.
type Reconciler interface {
	// SetupWatches declares the secondary objects the reconciler reacts to. The
	// primary object is the one passed to RegisterController.
	SetupWatches(w Watcher)

	Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
}

// Watcher is the subset of the controller builder a reconciler is allowed to
// touch. Narrowing it keeps a reconciler from reconfiguring the controller
// itself (concurrency, rate limiting), which belongs to the manager wiring.
type Watcher interface {
	Owns(object client.Object, opts ...builder.OwnsOption)
	Watches(object client.Object, eventHandler handler.EventHandler, opts ...builder.WatchesOption)
	WatchesRawSource(src source.Source)
	WithEventFilter(p predicate.Predicate)
}

// Base carries the dependencies most reconcilers need. Embed it to get them
// injected.
type Base struct {
	Client   client.Client
	Recorder record.EventRecorder
}

func (b *Base) InjectClient(c client.Client)          { b.Client = c }
func (b *Base) InjectRecorder(r record.EventRecorder) { b.Recorder = r }

// Optional interfaces a reconciler may implement to receive dependencies or to
// hook into setup.
type (
	NeedsClient   interface{ InjectClient(client.Client) }
	NeedsRecorder interface{ InjectRecorder(record.EventRecorder) }
	NeedsSetup    interface{ Setup(mgr ctrl.Manager) error }

	// NeedsForPredicates lets a reconciler filter events of its primary object.
	NeedsForPredicates interface{ ForPredicates() []predicate.Predicate }

	// OptsOutOfLeaderElection lets a reconciler run on every replica rather than only
	// on the one holding the lease.
	//
	// For reconcilers that converge nothing — an exporter that only reflects what the
	// resources already say. Running those on the leader alone would make the picture
	// of the cluster depend on the health of one pod, which is backwards: the thing
	// that reports a problem should not be the thing most likely to have it.
	OptsOutOfLeaderElection interface{ SkipLeaderElection() bool }
)

type entry struct {
	name       string
	obj        client.Object
	reconciler Reconciler
}

var entries []entry

// RegisterController adds a reconciler to the registry. Called from init.
func RegisterController(name string, obj client.Object, r Reconciler) {
	entries = append(entries, entry{name: name, obj: obj, reconciler: r})
}

// SetupAll wires every registered reconciler into the manager.
//
// disabled names a controller to leave out. This exists for incident response:
// a reconciler that is misbehaving on a live cluster can be switched off
// without rolling back the whole image.
func SetupAll(mgr ctrl.Manager, c client.Client, disabled []string, maxConcurrentReconciles int) error {
	setupLog := ctrl.Log.WithName("setup")

	for _, name := range disabled {
		if !slices.ContainsFunc(entries, func(e entry) bool { return e.name == name }) {
			setupLog.Info("WARNING: unknown controller in --disable-controllers, ignoring",
				"controller", name, "registered", registeredNames())
		}
	}

	for _, e := range entries {
		if slices.Contains(disabled, e.name) {
			setupLog.Info("controller disabled", "controller", e.name)
			continue
		}

		if err := setupController(mgr, c, e.name, e.obj, e.reconciler, maxConcurrentReconciles); err != nil {
			return fmt.Errorf("setting up controller %s: %w", e.name, err)
		}
		setupLog.Info("controller enabled", "controller", e.name)
	}

	return nil
}

func registeredNames() string {
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.name)
	}
	return strings.Join(names, ", ")
}

func setupController(
	mgr ctrl.Manager, c client.Client, name string, obj client.Object, r Reconciler, maxConcurrentReconciles int,
) error {
	if maxConcurrentReconciles < 1 {
		maxConcurrentReconciles = 1
	}

	if v, ok := r.(NeedsClient); ok {
		v.InjectClient(c)
	}
	if v, ok := r.(NeedsRecorder); ok {
		v.InjectRecorder(mgr.GetEventRecorderFor(name))
	}
	if v, ok := r.(NeedsSetup); ok {
		if err := v.Setup(mgr); err != nil {
			return fmt.Errorf("setup: %w", err)
		}
	}

	options := controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}
	if v, ok := r.(OptsOutOfLeaderElection); ok && v.SkipLeaderElection() {
		options.NeedLeaderElection = ptr.To(false)
	}

	b := ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(options)

	if v, ok := r.(NeedsForPredicates); ok {
		b = b.For(obj, builder.WithPredicates(v.ForPredicates()...))
	} else {
		b = b.For(obj)
	}

	r.SetupWatches(&builderWatcher{b: b})

	if err := b.Complete(r); err != nil {
		return fmt.Errorf("build controller: %w", err)
	}
	return nil
}

var _ Watcher = (*builderWatcher)(nil)

type builderWatcher struct {
	b *ctrl.Builder
}

func (w *builderWatcher) Owns(object client.Object, opts ...builder.OwnsOption) {
	w.b.Owns(object, opts...)
}

func (w *builderWatcher) Watches(object client.Object, eventHandler handler.EventHandler, opts ...builder.WatchesOption) {
	w.b.Watches(object, eventHandler, opts...)
}

func (w *builderWatcher) WatchesRawSource(src source.Source) {
	w.b.WatchesRawSource(src)
}

func (w *builderWatcher) WithEventFilter(p predicate.Predicate) {
	w.b.WithEventFilter(p)
}
