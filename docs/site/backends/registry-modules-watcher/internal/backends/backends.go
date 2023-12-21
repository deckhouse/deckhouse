// Copyright 2023 Flant JSC
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

package backends

import (
	"context"
	"sync"

	"k8s.io/klog"
)

type Sender interface {
	Send(ctx context.Context, listBackends map[string]struct{}, versions []Version) error
}

type RegistryScaner interface {
	GetState() []Version
	SubscribeOnUpdate(updateHandler func([]Version) error)
}

var instance *backends = nil

type Version struct {
	Registry        string
	Module          string
	Version         string
	ReleaseChannels []string
	TarFile         []byte
}

type backends struct {
	registryScaner RegistryScaner
	sender         Sender

	m            sync.RWMutex
	listBackends map[string]struct{} // list of backends ip addreses
}

func New(registryScaner RegistryScaner, sender Sender) *backends {
	if instance == nil {
		instance = &backends{
			registryScaner: registryScaner,
			sender:         sender,
			listBackends:   make(map[string]struct{}),
		}
	}
	registryScaner.SubscribeOnUpdate(instance.updateHandler)

	return instance
}

func Get() (b *backends, ok bool) {
	if instance == nil {
		return nil, false
	}

	return instance, true
}

// Add new backend to list backends
func (b *backends) Add(backend string) {
	klog.V(3).Infof(`backend Add call for: %v`, backend)

	b.m.Lock()
	defer b.m.Unlock()

	b.listBackends[backend] = struct{}{}
	state := b.registryScaner.GetState()
	err := b.sender.Send(context.Background(), map[string]struct{}{backend: {}}, state)
	if err != nil {
		klog.Fatal("error sending docs to new backend: ", err)
	}
}

func (b *backends) Delete(backend string) {
	klog.V(3).Infof(`backend Delete call for: %v`, backend)

	b.m.Lock()
	defer b.m.Unlock()

	delete(b.listBackends, backend)
}

// UpdateDocks send update dock request to all backends
func (b *backends) updateHandler(versions []Version) error {
	klog.V(3).Infof(`"registryScaner" produce update event`)

	b.m.RLock()
	defer b.m.RUnlock()

	err := b.sender.Send(context.Background(), b.listBackends, versions)
	if err != nil {
		return err
	}

	return nil
}
