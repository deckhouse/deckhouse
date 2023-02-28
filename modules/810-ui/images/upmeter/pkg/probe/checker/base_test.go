/*
Copyright 2023 Flant JSC

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

package checker

import (
	"testing"
	"time"
)

func Test_withTimer(t *testing.T) {
	{
		msg := "timer #1 should not run out of time"
		timeout := time.Millisecond
		withTimer(
			timeout,
			func() {
				time.Sleep(timeout / 2)
			},
			func() {
				t.Errorf(msg)
			},
		)
	}

	{
		msg := "timer #2 should not ignore timeout callback"
		timeoutCallbackCalled := make(chan struct{})
		timeout := time.Millisecond
		withTimer(
			timeout,
			func() {
				time.Sleep(2 * timeout)
				select {
				case <-timeoutCallbackCalled:
					return
				default:
					t.Errorf(msg)
				}
			},
			func() {
				timeoutCallbackCalled <- struct{}{}
			},
		)
	}
}
