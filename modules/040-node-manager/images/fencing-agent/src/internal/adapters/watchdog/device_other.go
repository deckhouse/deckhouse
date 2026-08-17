//go:build !linux

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

package watchdog

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// The watchdog API is Linux-only. These stubs keep the module building and
// testable on developer machines; the agent itself only runs in a Linux
// container, where the build tags select the real implementation.

func Open(_ string, _ *log.Logger) (Device, error) {
	return nil, fmt.Errorf("watchdog device on %s: %w", runtime.GOOS, errors.ErrUnsupported)
}

func Nowayout(_ string) (bool, error) {
	return false, fmt.Errorf("watchdog nowayout on %s: %w", runtime.GOOS, errors.ErrUnsupported)
}
