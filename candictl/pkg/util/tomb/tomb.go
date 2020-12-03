package tomb

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"flant/candictl/pkg/log"
)

var callbacks teardownCallbacks

func init() {
	ctx, cancel := context.WithCancel(context.Background())

	callbacks = teardownCallbacks{
		waitCh: make(chan struct{}, 1),
		stopCh: make(chan struct{}, 1),
		Ctx:    ctx,
		Cancel: cancel,
	}
}

type callback struct {
	Name string
	Do   func()
}

type teardownCallbacks struct {
	mutex sync.RWMutex
	data  []callback

	exhausted        bool
	notInterruptable bool

	waitCh chan struct{}
	stopCh chan struct{}

	Ctx    context.Context
	Cancel context.CancelFunc
}

func (c *teardownCallbacks) registerOnShutdown(name string, cb func()) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.data = append(c.data, callback{Name: name, Do: cb})
	log.DebugF("callback added, callbacks in queue: %d\n", len(c.data))
}

func (c *teardownCallbacks) shutdown() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// FIFO order to shutdown fundamental things last
	log.DebugF("teardown started, queue length: %d\n", len(c.data))
	// FIFO order to shutdown fundamental things last

	for i := len(c.data) - 1; i >= 0; i-- {
		cb := c.data[i]
		cb.Do()
		c.data[i] = callback{Name: "Stub", Do: func() {}}
		log.DebugF("callback called: %s %d\n", cb.Name, i)
	}

	log.DebugLn("teardown stopped")
	c.exhausted = true
	c.waitCh <- struct{}{}
}

func (c *teardownCallbacks) wait() {
	<-c.waitCh
}

func RegisterOnShutdown(process string, cb func()) {
	callbacks.registerOnShutdown(process, cb)
}

func Shutdown() {
	callbacks.shutdown()
}

func WaitShutdown() {
	callbacks.wait()
}

func Ctx() context.Context {
	return callbacks.Ctx
}

func StopCh() chan struct{} {
	return callbacks.stopCh
}

func WithoutInterruptions(fn func()) {
	callbacks.notInterruptable = true
	defer func() { callbacks.notInterruptable = false }()
	fn()
}

func WaitForProcessInterruption() {
	interruptCh := make(chan os.Signal, 1)
	signal.Notify(interruptCh, syscall.SIGINT, syscall.SIGTERM)

Select:
	s := <-interruptCh

	switch s {
	case syscall.SIGTERM, syscall.SIGINT:
		if callbacks.notInterruptable {
			goto Select
		}
		go func() {
			<-interruptCh
			log.ErrorLn("Killed by interrupting process twice.")
			os.Exit(1)
		}()
		callbacks.Cancel()

		StopCh() <- struct{}{}
		callbacks.data = append([]callback{{
			Name: "Shutdown message",
			Do: func() {
				log.WarnLn(fmt.Sprintf("Graceful shutdown by %q signal ...", s.String()))
			},
		}}, callbacks.data...)

		Shutdown()
	default:
		os.Exit(1)
	}
}
