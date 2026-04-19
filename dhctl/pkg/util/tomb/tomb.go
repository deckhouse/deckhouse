// Copyright 2021 Flant JSC
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

package tomb

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

var callbacks teardownCallbacks

func init() {
	callbacks = teardownCallbacks{
		waitCh:        make(chan struct{}, 1),
		interruptedCh: make(chan struct{}, 1),
	}
}

type callback struct {
	Name string
	Do   func()
}

type teardownCallbacks struct {
	mutex    sync.RWMutex
	data     []*callback
	exitCode int

	exhausted        bool
	notInterruptable bool

	waitCh        chan struct{}
	interruptedCh chan struct{}
}

func (c *teardownCallbacks) registerOnShutdown(name string, cb func()) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.data = append(c.data, &callback{Name: name, Do: cb})
	log.DebugF("teardown callback '%s' added, callbacks in queue: %d\n", name, len(c.data))
}

func (c *teardownCallbacks) replaceOnShutdown(name string, cb func()) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, clb := range c.data {
		if clb.Name == name {
			clb.Do = cb
			log.DebugF("teardown callback '%s' replaced, callbacks in queue: %d\n", name, len(c.data))
			return
		}
	}

	log.DebugF("teardown callback '%s' not found, do nothing, callbacks in queue: %d\n", name, len(c.data))
}

func (c *teardownCallbacks) shutdown(exitCode int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Prevent double shutdown.
	if c.exhausted {
		return
	}

	c.exitCode = exitCode

	log.DebugF("teardown started, queue length: %d\n", len(c.data))

	// Run callbacks in FIFO order to shutdown fundamental things last.
	for i := len(c.data) - 1; i >= 0; i-- {
		cb := c.data[i]
		log.DebugF("teardown callback %d: '%s' started\n", i, cb.Name)
		cb.Do()
		c.data[i] = &callback{Name: "Stub", Do: func() {}}
		log.DebugF("teardown callback %d: '%s' done\n", i, cb.Name)
	}

	log.DebugLn("teardown is finished")
	c.exhausted = true
	close(c.waitCh)
}

func (c *teardownCallbacks) wait() {
	<-c.waitCh
}

func RegisterOnShutdown(process string, cb func()) {
	callbacks.registerOnShutdown(process, cb)
}

func ReplaceOnShutdown(process string, cb func()) {
	callbacks.replaceOnShutdown(process, cb)
}

func Shutdown(code int) {
	callbacks.shutdown(code)
}

func WaitShutdown() int {
	callbacks.wait()
	return callbacks.exitCode
}

func IsInterrupted() bool {
	select {
	case <-callbacks.interruptedCh:
		return true
	default:
	}
	return false
}

func WithoutInterruptions(fn func()) {
	callbacks.notInterruptable = true
	defer func() { callbacks.notInterruptable = false }()
	fn()
}

func printGorutinesStackTrace(shouldAlwaysPrint bool, msg string) {
	// collect stacktrace for debug
	buf := make([]byte, 20971520) // 20 mb
	l := runtime.Stack(buf, true)
	buf = buf[:l]
	if shouldAlwaysPrint || input.IsTerminal() {
		log.InfoF("\n%sGorutines stack for debug:\n%s\n", msg, string(buf))
	}

	buf = nil
}

type BeforeInterrupted []func(sig os.Signal)

func (b BeforeInterrupted) Handle(sig os.Signal) {
	if len(b) == 0 {
		return
	}

	for _, action := range b {
		action(sig)
	}
}

func WaitForProcessInterruption(beforeInterrupted BeforeInterrupted) {
	interruptCh := make(chan os.Signal, 1)
	signal.Notify(interruptCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2)

	for {
		s, ok := <-interruptCh
		if !ok {
			return
		}

		var exitCode int
		switch s {
		case syscall.SIGUSR2:
			printGorutinesStackTrace(true, "")
			continue
		case syscall.SIGUSR1:
			exitCode = 1
		case syscall.SIGTERM, syscall.SIGINT:
			exitCode = 0
		default:
			// will not exec anytime because we handle all
			beforeInterrupted.Handle(s)
			os.Exit(1)
			return
		}

		if callbacks.notInterruptable {
			continue
		}

		beforeInterrupted.Handle(s)
		graceShutdownForSignal(interruptCh, exitCode, s)
		return
	}
}

func graceShutdownForSignal(interruptCh <-chan os.Signal, exitCode int, s os.Signal) {
	// Wait for the second signal to kill the main process immediately.
	go func() {
		<-interruptCh

		printGorutinesStackTrace(false, "Killed by signal twice. Probably dhctl have problems. ")

		log.ErrorLn("Killed by signal twice.")
		os.Exit(1)
	}()

	// Close interrupted channel to signal interruptable loops to stop.
	close(callbacks.interruptedCh)

	// Run all registered teardown callbacks and print an explanation at the end.
	callbacks.data = append([]*callback{{
		Name: "Shutdown message",
		Do: func() {
			log.WarnLn(fmt.Sprintf("Graceful shutdown by %q signal ...", s.String()))
		},
	}}, callbacks.data...)
	Shutdown(exitCode)
}
