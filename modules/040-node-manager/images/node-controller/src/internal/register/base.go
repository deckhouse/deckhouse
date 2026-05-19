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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

type NeedsSetup interface {
	Setup(mgr ctrl.Manager) error
}
