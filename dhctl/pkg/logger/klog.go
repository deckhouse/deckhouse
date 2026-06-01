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

package logger

import (
	"log/slog"

	"k8s.io/klog/v2"
)

// BindKlog routes k8s.io/klog/v2 output into the given slog logger. Sensitive-keyword
// sanitization happens in the handler (via Sanitize), so klog output is filtered too.
func BindKlog(l *slog.Logger) {
	klog.SetSlogLogger(l)
}
