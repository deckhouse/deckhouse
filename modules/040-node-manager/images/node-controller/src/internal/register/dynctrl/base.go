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
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Base struct {
	Client   client.Client
	Cache    cache.Cache
	Scheme   *runtime.Scheme
	Logger   logr.Logger
	Recorder record.EventRecorder
}

var (
	_ NeedsClient   = (*Base)(nil)
	_ NeedsCache    = (*Base)(nil)
	_ NeedsScheme   = (*Base)(nil)
	_ NeedsLogger   = (*Base)(nil)
	_ NeedsRecorder = (*Base)(nil)
)

func (b *Base) InjectClient(c client.Client)          { b.Client = c }
func (b *Base) InjectCache(c cache.Cache)             { b.Cache = c }
func (b *Base) InjectScheme(s *runtime.Scheme)        { b.Scheme = s }
func (b *Base) InjectLogger(l logr.Logger)            { b.Logger = l }
func (b *Base) InjectRecorder(r record.EventRecorder) { b.Recorder = r }
