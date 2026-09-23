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

package apis

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	crdinstaller "github.com/deckhouse/module-sdk/pkg/crd-installer"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// moduleCRDFile is the legacy Module CRD, replaced by moduleV2CRDFile when the Module v2 gate is on.
	moduleCRDFile = "module.yaml"
	// moduleV2CRDFile is the Module v2 CRD, installed only when the Module v2 gate is on.
	moduleV2CRDFile = "modulev2.yaml"
)

// list of CRDs to delete, like "externalmodulesources.deckhouse.io"
var deprecatedCRDs = []string{}

type kubeClient interface {
	kubernetes.Interface
	Dynamic() dynamic.Interface
	InvalidateDiscoveryCache()
}

var defaultLabels = map[string]string{
	crdinstaller.LabelHeritage: "deckhouse",
}

// EnsureCRDs installs or update primary CRDs for deckhouse-controller
func EnsureCRDs(ctx context.Context, client kubeClient, crdsGlob string) error {
	crds, err := filepath.Glob(crdsGlob)
	if err != nil {
		return fmt.Errorf("glob %q: %w", crdsGlob, err)
	}

	skip := moduleV2CRDFile
	if app.ModuleV2Enabled() {
		skip = moduleCRDFile
	}

	filter := func(crdPath string) bool {
		name := filepath.Base(crdPath)
		if name == skip {
			return false
		}

		return !strings.HasPrefix(name, "doc-")
	}

	inst := crdinstaller.NewCRDsInstaller(
		client.Dynamic(),
		crds,
		crdinstaller.WithExtraLabels(defaultLabels),
		crdinstaller.WithFileFilter(filter),
	)

	deletedCRDs, err := inst.DeleteCRDs(ctx, deprecatedCRDs)
	if err != nil {
		log.Warn("Couldn't delete deprecated CRDs", log.Err(err))
	} else {
		log.Info("The following deprecated CRDs were deleted", slog.String("crds", strings.Join(deletedCRDs, ",")))
	}

	err = inst.Run(ctx)

	// it's not necessary, but it could speed up a bit further api discovery
	client.InvalidateDiscoveryCache()

	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	return nil
}
