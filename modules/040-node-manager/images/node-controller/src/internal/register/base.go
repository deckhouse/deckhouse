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

package register

import (
	"context"

	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type Base struct {
	Client   client.Client
	Recorder record.EventRecorder
}

func (b *Base) InjectClient(c client.Client)          { b.Client = c }
func (b *Base) InjectRecorder(r record.EventRecorder) { b.Recorder = r }

type NeedsClient interface {
	InjectClient(client.Client)
}

type NeedsRecorder interface {
	InjectRecorder(record.EventRecorder)
}

// NeedsSetup is run once before the controller is built. The context is the manager's own —
// it is cancelled when the process is asked to stop, so a Setup that talks to the API server
// does not outlive it.
type NeedsSetup interface {
	Setup(ctx context.Context, mgr ctrl.Manager) error
}

// NeedsForPredicates lets a reconciler filter events of its primary (For) object.
type NeedsForPredicates interface {
	ForPredicates() []predicate.Predicate
}

// NeedsMaxConcurrentReconciles lets a reconciler cap the number of workers it is
// run with, for the ones whose correctness depends on that number rather than on
// how they happen to be deployed. The cap wins over the flag: a reconciler that
// is only correct single-threaded says so here, once, instead of depending on an
// argument that one typo anywhere in the same string silently discards.
type NeedsMaxConcurrentReconciles interface {
	MaxConcurrentReconciles() int
}
