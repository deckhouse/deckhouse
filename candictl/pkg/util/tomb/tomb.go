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

type teardownCallbacks struct {
	mutex sync.RWMutex
	data  []func()

	exhausted bool

	waitCh chan struct{}
	stopCh chan struct{}

	Ctx    context.Context
	Cancel context.CancelFunc
}

func (c *teardownCallbacks) registerOnShutdown(cbs []func()) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.data = append(c.data, cbs...)
	log.DebugF("Callback added: %T\n", cbs)
}

func (c *teardownCallbacks) shutdown() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// FIFO order to shutdown fundamental things last
	log.DebugF("Teardown started, queue length: %d\n", len(c.data))
	// FIFO order to shutdown fundamental things last

	for i := len(c.data) - 1; i >= 0; i-- {
		cb := c.data[i]
		cb()
		c.data[i] = func() {}
		log.DebugF("Callback called: %d\n", i)
	}

	log.DebugF("Teardown stopped\n")
	c.exhausted = true
	c.waitCh <- struct{}{}
}

func (c *teardownCallbacks) wait() {
	<-c.waitCh
}

func RegisterOnShutdown(cbs ...func()) {
	callbacks.registerOnShutdown(cbs)
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

func WaitForProcessInterruption() {
	interruptCh := make(chan os.Signal, 1)
	signal.Notify(interruptCh, syscall.SIGINT, syscall.SIGTERM)

	s := <-interruptCh
	go func() {
		<-interruptCh
		log.ErrorLn("Killed by interrupting process twice.")
		os.Exit(1)
	}()

	switch s {
	case syscall.SIGTERM, syscall.SIGINT:
		callbacks.Cancel()
		log.Warning(fmt.Sprintf("Graceful shutdown by \"%s\" signal ...\n", s.String()))
		StopCh() <- struct{}{}
		Shutdown()
	default:
		os.Exit(1)
	}
}
