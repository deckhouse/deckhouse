// Copyright 2025 Flant JSC
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

package dhctl

import (
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

// newRequestOptions returns a per-request *options.Options seeded with the
// values every RPC handler used to write into the dhctl/pkg/app globals.
//
// Building a fresh struct per request — instead of mutating package-level vars —
// is what makes the gRPC server safe to handle concurrent operations.
//
// resourcesTimeout/deckhouseTimeout default to zero when unset (Bootstrap does
// not carry these in its request); the consuming operation falls back to its
// own default in that case.
func newRequestOptions(cacheDir string, skipPreflightChecks []string, timeouts ...time.Duration) *options.Options {
	opts := options.New()
	opts.Global.SanityCheck = true
	opts.Cache.UseTfCache = options.UseStateCacheYes
	opts.Cache.Dir = cacheDir
	opts.Preflight.ApplySkips(skipPreflightChecks)
	if len(timeouts) > 0 {
		opts.Bootstrap.ResourcesTimeout = timeouts[0]
	}
	if len(timeouts) > 1 {
		opts.Bootstrap.DeckhouseTimeout = timeouts[1]
	}
	return opts
}

type logAfterReturnFunc func()

func logInformationAboutInstance(params ServiceParams, logger log.Logger) logAfterReturnFunc {
	podName := params.PodName
	podWithPrefix := fmt.Sprintf("pod/%s", podName)
	warnAboutNs := ""
	ns := params.PodNamespace

	if ns == "" {
		warnAboutNs = "Warning! Use default namespace\n"
		ns = "d8-commander"
	}

	logger.LogInfoF("Task is running by DHCTL Server %s\n", podWithPrefix)

	if warnAboutNs != "" {
		logger.LogInfoLn(warnAboutNs)
	}

	logger.LogInfoF("DHCTL logs: d8 k -n %s logs %s\n", ns, podName)

	return func() { logger.LogInfoF("Task done by DHCTL Server %s\n", podWithPrefix) }
}
