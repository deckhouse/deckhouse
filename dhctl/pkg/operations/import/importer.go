package _import

import (
	"context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
)

type Params struct {
	CommanderMode    bool
	TerraformContext *terraform.TerraformContext
	OnPhaseFunc      OnPhaseFunc
}

type Importer struct {
	Params *Params
}

func NewImporter(params *Params) *Importer {
	return &Importer{Params: params}
}

func (i *Importer) Import(ctx context.Context) error {
	// TODO(cluster-import): implement Scan, Capture and Check phases
	return nil
}
