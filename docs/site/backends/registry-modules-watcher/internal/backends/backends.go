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
	// SubscribeOnUpdate()
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

	return instance
}

func Get() (b *backends, ok bool) {
	if instance == nil {
		return nil, false
	}

	return instance, true
}

// Add new backend to list backends
func (b *backends) Add(backends ...string) {
	b.m.Lock()
	defer b.m.Unlock()
	for _, backend := range backends {
		b.listBackends[backend] = struct{}{}
	}

	state := b.registryScaner.GetState()
	x := map[string]struct{}{
		"localhost:8081": {},
	}
	err := b.sender.Send(context.TODO(), x, state)
	if err != nil {
		klog.Fatal("error sending docs to new backend: ", err)
	}
}

func (b *backends) Delete(backend string) {
	b.m.Lock()
	defer b.m.Unlock()

	delete(b.listBackends, backend)
}

// UpdateDocks send update dock request to all backends
func (b *backends) subscribeOnUpdates() {
	// eventChan := SubscribeOnUpdate()
	// for event := range eventChan {
	//    foreach backend {
	//	     sender.Send(backend, event.docState)
	//    }
	// }
}
