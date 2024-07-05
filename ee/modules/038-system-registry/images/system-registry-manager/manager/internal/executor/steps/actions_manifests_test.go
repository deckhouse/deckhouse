/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

import (
	"context"
	"github.com/stretchr/testify/assert"
	pkg_cfg "system-registry-manager/pkg/cfg"
	"testing"
)

func TestCreateManifestBundle(t *testing.T) {
	err := generateInputConfigForTest()
	assert.NoError(t, err)

	manifestsSpec := pkg_cfg.NewManifestsSpecForTest()
	params := InputParams{
		Certs:     struct{ UpdateOrCreate bool }{UpdateOrCreate: true},
		Manifests: struct{ UpdateOrCreate bool }{UpdateOrCreate: true},
		StaticPods: struct {
			UpdateOrCreate bool
			Options        struct {
				MasterPeers     []string
				IsRaftBootstrap bool
			}
			Check struct {
				WithMasterPeers     bool
				WithIsRaftBootstrap bool
			}
		}{
			UpdateOrCreate: true,
			Options: struct {
				MasterPeers     []string
				IsRaftBootstrap bool
			}{
				MasterPeers:     []string{"123", "321"},
				IsRaftBootstrap: true,
			},
			Check: struct {
				WithMasterPeers     bool
				WithIsRaftBootstrap bool
			}{
				WithMasterPeers:     true,
				WithIsRaftBootstrap: true,
			},
		},
	}

	renderData, err := pkg_cfg.GetDataForManifestRendering(
		pkg_cfg.NewExtraDataForManifestRendering(
			params.StaticPods.Options.MasterPeers,
			params.StaticPods.Options.IsRaftBootstrap,
		),
	)
	assert.NoError(t, err)

	for _, manifest := range manifestsSpec.Manifests {
		_, err := CreateManifestBundle(context.Background(), &manifest, &renderData)
		assert.NoError(t, err)
	}
}
