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

package deckhouse

import (
	"context"
	"fmt"
	"os"

	crdinstaller "github.com/deckhouse/module-sdk/pkg/crd-installer"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

// EnsureModuleConfigCRD applies the ModuleConfig CRD shipped in the installer
// image (or in the downloaded candi image for standalone dhctl) so that
// ModuleConfigs can be created without waiting for deckhouse-controller to
// start. The same crd-installer library and heritage label are used as in
// deckhouse-controller's EnsureCRDs, which re-applies the CRD on every startup
// and remains its long-term owner.
//
// A missing file is not an error: bootstrap falls back to the previous
// behavior where ModuleConfig creation retries until deckhouse-controller
// installs the CRD.
func EnsureModuleConfigCRD(ctx context.Context, kubeCl *client.KubernetesClient, crdPath string) error {
	if crdPath == "" {
		log.WarnLn("ModuleConfig CRD path is not set. The CRD will be installed by deckhouse-controller.")
		return nil
	}

	if _, err := os.Stat(crdPath); err != nil {
		log.WarnF("ModuleConfig CRD file %q is not available: %v. The CRD will be installed by deckhouse-controller.\n", crdPath, err)
		return nil
	}

	return log.ProcessCtx(ctx, "default", "Install ModuleConfig CRD", func(ctx context.Context) error {
		inst := crdinstaller.NewCRDsInstaller(
			kubeCl.Dynamic(),
			[]string{crdPath},
			crdinstaller.WithExtraLabels(map[string]string{crdinstaller.LabelHeritage: "deckhouse"}),
		)

		if err := inst.Run(ctx); err != nil {
			return fmt.Errorf("install ModuleConfig CRD: %w", err)
		}

		kubeCl.InvalidateDiscoveryCache()

		return nil
	})
}
