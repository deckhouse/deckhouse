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

package interrupter

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// WaitForProcessInterruption wait for SIGINT or SIGTERM and run a callback function.
//
// First signal start a callback function, which should call os.Exit(0).
// Next signal will call os.Exit(128 + signal-value).
// If no cb is given,
func WaitForProcessInterruption(cb func()) {
	interruptCh := make(chan os.Signal, 1)
	signal.Notify(interruptCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-interruptCh
	fmt.Printf("Grace shutdown by '%s' signal\n", sig.String())
	cb()
}
