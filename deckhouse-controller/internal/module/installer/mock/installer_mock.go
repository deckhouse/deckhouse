package mock

import (
	"context"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

type Installer struct {
}

func (i *Installer) Install(ctx context.Context, module, version, tempModulePath string) error {
	return nil
}

func (i *Installer) Uninstall(ctx context.Context, module string) error {
	return nil
}

func (i *Installer) Download(ctx context.Context, source *v1alpha1.ModuleSource, moduleName string, moduleVersion string) (string, error) {
	return "testdata//validation/module", nil
}
