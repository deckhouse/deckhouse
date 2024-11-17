/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry_controller

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var reconcileRequestHandler = handler.TypedEnqueueRequestsFromMapFunc(
	func(ctx context.Context, req reconcile.Request) []reconcile.Request {
		return []reconcile.Request{req}
	},
)

func oneshotSource(name, namespace string) source.Source {
	req := reconcile.Request{}
	req.Name = name
	req.Namespace = namespace

	ch := make(chan event.TypedGenericEvent[reconcile.Request], 1)
	ch <- event.TypedGenericEvent[reconcile.Request]{Object: req}
	close(ch)

	return source.Channel(ch, reconcileRequestHandler)
}
