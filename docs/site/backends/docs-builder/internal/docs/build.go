// Copyright 2023 Flant JSC
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

package docs

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/pkg/hugo"
)

func (svc *Service) buildHugo(ctx context.Context) error {
	flags := &hugo.Flags{
		LogLevel: "debug",
		Source:   svc.baseDir,
		CfgDir:   filepath.Join(svc.baseDir, "config"),

		WrapDestination: svc.wrapDestination,
	}

	for {
		// A single hugo pass runs to completion once started, but this
		// self-heal loop can rerun it once per broken module it strips —
		// several full-site builds in one call. Bail between passes so a
		// canceled request stops multiplying rebuilds instead of holding the
		// build slot for every remaining broken module.
		if err := ctx.Err(); err != nil {
			return err
		}

		buildErr := hugo.Build(ctx, flags, svc.logger)
		if buildErr == nil {
			return nil
		}

		if moduleName, ok := getModuleNameFromErrorPath(buildErr.Error()); ok {
			paths := []string{
				filepath.Join(svc.baseDir, contentDir, moduleName),
				filepath.Join(svc.baseDir, modulesDir, moduleName),
			}

			for _, path := range paths {
				err := os.RemoveAll(path)
				if err != nil {
					return fmt.Errorf("remove module: %w", err)
				}
			}

			svc.logger.Warn("removed broken module", slog.String("name", moduleName), log.Err(buildErr))
			continue
		}

		return buildErr
	}
}

func getModuleNameFromErrorPath(errorMessage string) (string, bool) {
	match := assembleErrorRegexp.FindStringSubmatch(errorMessage)
	if len(match) == 6 {
		// return only module name
		return match[2], true
	}

	return "", false
}
