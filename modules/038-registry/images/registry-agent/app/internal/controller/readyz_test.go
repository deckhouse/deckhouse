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

package controller

import (
	"net/http"
	"sync/atomic"
	"testing"
)

func TestReadyzCheck(t *testing.T) {
	ready := &atomic.Bool{}
	check := ReadyzCheck(ready)
	req, _ := http.NewRequest(http.MethodGet, "/readyz", nil)

	if err := check(req); err == nil {
		t.Error("expected not-ready before first reconcile")
	}
	ready.Store(true)
	if err := check(req); err != nil {
		t.Errorf("expected ready after flag set, got %v", err)
	}
}
