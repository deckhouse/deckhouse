/*
Copyright 2025 Flant JSC

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

package dynctrl

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type Watcher interface {
	Owns(object client.Object, opts ...builder.OwnsOption)
	Watches(object client.Object, eventHandler handler.EventHandler, opts ...builder.WatchesOption)
	WatchesRawSource(src source.Source)
	WithEventFilter(p predicate.Predicate)
}

type Reconciler interface {
	SetupWatches(w Watcher)
	Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
}
