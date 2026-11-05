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

package toy

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/watch"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	"github.com/deckhouse/deckhouse/pkg/api-extension-sdk/scope"
)

// Storage is an in-memory REST storage of gizmos. List, Get and Watch apply
// the scope decision of the request; Create and Delete are not scoped.
type Storage struct {
	mu          sync.RWMutex
	items       map[string]*Gizmo // key: namespace/name
	rv          uint64
	broadcaster *watch.Broadcaster
}

var (
	_ rest.Storage              = &Storage{}
	_ rest.Scoper               = &Storage{}
	_ rest.SingularNameProvider = &Storage{}
	_ rest.KindProvider         = &Storage{}
	_ rest.Lister               = &Storage{}
	_ rest.Getter               = &Storage{}
	_ rest.Watcher              = &Storage{}
	_ rest.Creater              = &Storage{} //nolint:misspell // upstream interface name
	_ rest.GracefulDeleter      = &Storage{}
	_ rest.TableConvertor       = &Storage{}
)

func NewStorage() *Storage {
	return &Storage{
		items:       map[string]*Gizmo{},
		broadcaster: watch.NewBroadcaster(1000, watch.WaitIfChannelFull),
	}
}

// Seed creates gizmo-<ns> in every namespace of namespaces.
func (s *Storage) Seed(namespaces []string) error {
	for _, ns := range namespaces {
		g := &Gizmo{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "gizmo-" + ns}, Spec: GizmoSpec{Color: "seeded"}}
		ctx := genericapirequest.WithNamespace(context.Background(), ns)
		if _, err := s.Create(ctx, g, nil, &metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (s *Storage) New() runtime.Object     { return &Gizmo{} }
func (s *Storage) NewList() runtime.Object { return &GizmoList{} }
func (s *Storage) Destroy()                { s.broadcaster.Shutdown() }
func (s *Storage) NamespaceScoped() bool   { return true }
func (s *Storage) GetSingularName() string { return "gizmo" }
func (s *Storage) Kind() string            { return "Gizmo" }

func key(ns, name string) string { return ns + "/" + name }

type matcher struct {
	namespace string
	labels    labels.Selector
	fields    fields.Selector
}

func newMatcher(ctx context.Context, opts *metainternalversion.ListOptions) matcher {
	m := matcher{namespace: genericapirequest.NamespaceValue(ctx), labels: labels.Everything(), fields: fields.Everything()}
	if opts != nil {
		if opts.LabelSelector != nil {
			m.labels = opts.LabelSelector
		}
		if opts.FieldSelector != nil {
			m.fields = opts.FieldSelector
		}
	}
	return m
}

func (m matcher) matches(g *Gizmo) bool {
	if m.namespace != "" && g.Namespace != m.namespace {
		return false
	}
	return m.labels.Matches(labels.Set(g.Labels)) &&
		m.fields.Matches(fields.Set{"metadata.name": g.Name, "metadata.namespace": g.Namespace})
}

func (s *Storage) List(ctx context.Context, opts *metainternalversion.ListOptions) (runtime.Object, error) {
	m := newMatcher(ctx, opts)
	s.mu.RLock()
	list := &GizmoList{ListMeta: metav1.ListMeta{ResourceVersion: strconv.FormatUint(s.rv, 10)}}
	for _, g := range s.items {
		if m.matches(g) {
			list.Items = append(list.Items, *g.DeepCopy())
		}
	}
	s.mu.RUnlock()
	sort.Slice(list.Items, func(i, j int) bool {
		return key(list.Items[i].Namespace, list.Items[i].Name) < key(list.Items[j].Namespace, list.Items[j].Name)
	})
	total := len(list.Items)
	// Filter in the storage: the handler converts the result to a Table
	// after List returns.
	if err := scope.FilterList(ctx, list); err != nil {
		return nil, err
	}
	logRead(ctx, "list", total, len(list.Items))
	return list, nil
}

func (s *Storage) Get(ctx context.Context, name string, _ *metav1.GetOptions) (runtime.Object, error) {
	ns := genericapirequest.NamespaceValue(ctx)
	if err := scope.CheckGet(ctx, GizmoResource, ns, name); err != nil {
		logRead(ctx, "get", 1, 0)
		return nil, err
	}
	s.mu.RLock()
	g, ok := s.items[key(ns, name)]
	s.mu.RUnlock()
	if !ok {
		return nil, apierrors.NewNotFound(GizmoResource, name)
	}
	logRead(ctx, "get", 1, 1)
	return g.DeepCopy(), nil
}

// Watch streams changes from now on. With an empty resourceVersion it first
// sends the current objects as ADDED, like a watch without a version does.
func (s *Storage) Watch(ctx context.Context, opts *metainternalversion.ListOptions) (watch.Interface, error) {
	m := newMatcher(ctx, opts)
	var initial []watch.Event
	s.mu.RLock()
	if opts == nil || opts.ResourceVersion == "" {
		for _, g := range s.items {
			if m.matches(g) {
				initial = append(initial, watch.Event{Type: watch.Added, Object: g.DeepCopy()})
			}
		}
	}
	// Subscribe under the lock so no event between the snapshot and the
	// subscription is lost.
	w, err := s.broadcaster.WatchWithPrefix(initial)
	s.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	byRequest := watch.Filter(w, func(e watch.Event) (watch.Event, bool) {
		g, ok := e.Object.(*Gizmo)
		return e, ok && m.matches(g)
	})
	logRead(ctx, "watch", -1, -1)
	return scope.WatchFilter(ctx, byRequest), nil
}

func (s *Storage) Create(ctx context.Context, obj runtime.Object, validate rest.ValidateObjectFunc, _ *metav1.CreateOptions) (runtime.Object, error) {
	g, ok := obj.(*Gizmo)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("not a Gizmo: %T", obj))
	}
	ns := genericapirequest.NamespaceValue(ctx)
	if ns == "" {
		return nil, apierrors.NewBadRequest("namespace is required")
	}
	if g.Namespace != "" && g.Namespace != ns {
		return nil, apierrors.NewBadRequest("the namespace of the object does not match the namespace of the request")
	}
	g = g.DeepCopy()
	g.Namespace = ns
	if g.Name == "" {
		return nil, apierrors.NewBadRequest("name is required")
	}
	if validate != nil {
		if err := validate(ctx, g); err != nil {
			return nil, err
		}
	}
	rest.FillObjectMetaSystemFields(g)

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.items[key(ns, g.Name)]; exists {
		return nil, apierrors.NewAlreadyExists(GizmoResource, g.Name)
	}
	s.rv++
	g.ResourceVersion = strconv.FormatUint(s.rv, 10)
	s.items[key(ns, g.Name)] = g
	if err := s.broadcaster.Action(watch.Added, g.DeepCopy()); err != nil {
		klog.ErrorS(err, "broadcast")
	}
	return g.DeepCopy(), nil
}

func (s *Storage) Delete(ctx context.Context, name string, validate rest.ValidateObjectFunc, _ *metav1.DeleteOptions) (runtime.Object, bool, error) {
	ns := genericapirequest.NamespaceValue(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.items[key(ns, name)]
	if !ok {
		return nil, false, apierrors.NewNotFound(GizmoResource, name)
	}
	if validate != nil {
		if err := validate(ctx, g); err != nil {
			return nil, false, err
		}
	}
	delete(s.items, key(ns, name))
	s.rv++
	g = g.DeepCopy()
	g.ResourceVersion = strconv.FormatUint(s.rv, 10)
	if err := s.broadcaster.Action(watch.Deleted, g.DeepCopy()); err != nil {
		klog.ErrorS(err, "broadcast")
	}
	return g, true, nil
}

func (s *Storage) ConvertToTable(_ context.Context, obj runtime.Object, _ runtime.Object) (*metav1.Table, error) {
	table := &metav1.Table{
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name"},
			{Name: "Color", Type: "string"},
			{Name: "Age", Type: "string"},
		},
	}
	row := func(g *Gizmo) metav1.TableRow {
		age := "<unknown>"
		if !g.CreationTimestamp.IsZero() {
			age = duration.HumanDuration(time.Since(g.CreationTimestamp.Time))
		}
		return metav1.TableRow{Cells: []any{g.Name, g.Spec.Color, age}, Object: runtime.RawExtension{Object: g}}
	}
	switch t := obj.(type) {
	case *Gizmo:
		table.ResourceVersion = t.ResourceVersion
		table.Rows = append(table.Rows, row(t))
	case *GizmoList:
		table.ResourceVersion = t.ResourceVersion
		table.Continue = t.Continue
		for i := range t.Items {
			table.Rows = append(table.Rows, row(&t.Items[i]))
		}
	default:
		return nil, fmt.Errorf("unexpected object %T", obj)
	}
	return table, nil
}

// logRead logs the decision applied to a read. total and served are -1 for
// a watch.
func logRead(ctx context.Context, verb string, total, served int) {
	userName := ""
	if u, ok := genericapirequest.UserFrom(ctx); ok {
		userName = u.GetName()
	}
	ns := genericapirequest.NamespaceValue(ctx)
	d, ok := scope.FromContext(ctx)
	if !ok {
		klog.InfoS("read", "verb", verb, "user", userName, "namespace", ns, "scope", "none", "total", total, "served", served)
		return
	}
	klog.InfoS("read", "verb", verb, "user", userName, "namespace", ns, "scope", d.Mode, "setSize", d.Namespaces.Len(), "total", total, "served", served)
}
