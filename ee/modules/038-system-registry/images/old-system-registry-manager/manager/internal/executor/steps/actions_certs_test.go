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

func TestCreateCertBundle(t *testing.T) {
	err := generateInputConfigForTest()
	assert.NoError(t, err)

	manifestsSpec := pkg_cfg.NewManifestsSpecForTest()

	for _, cert := range manifestsSpec.GeneratedCertificates {
		_, err := CreateCertBundle(context.Background(), &cert)
		assert.NoError(t, err)
	}
}
