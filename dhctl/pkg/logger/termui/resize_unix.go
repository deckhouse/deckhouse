// Copyright 2026 Flant JSC
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

//go:build !windows

package termui

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/pterm/pterm"
)

func terminalWidth() int  { return pterm.GetTerminalWidth() }
func terminalHeight() int { return pterm.GetTerminalHeight() }

// notifyResize bridges SIGWINCH onto a plain struct{} channel and stops on `stop`.
func notifyResize(stop <-chan struct{}) <-chan struct{} {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGWINCH)
	out := make(chan struct{}, 1)
	go func() {
		defer signal.Stop(sig)
		for {
			select {
			case <-stop:
				return
			case <-sig:
				select {
				case out <- struct{}{}:
				default:
				}
			}
		}
	}()
	return out
}
