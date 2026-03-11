package dynctrl

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

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
