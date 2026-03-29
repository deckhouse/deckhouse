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

package dvp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func TestKubeconfigDataBase64HappyPath(t *testing.T) {
	preparator := NewMetaConfigPreparator()

	kubeconfig := base64.StdEncoding.EncodeToString([]byte(testKubeconfig()))
	metaCfg := metaConfigWithProvider(t, DVPProviderSpec{KubeconfigDataBase64: kubeconfig})

	_, err := preparator.KubeconfigDataBase64(metaCfg)
	require.NoError(t, err)
}

func TestPrepare(t *testing.T) {
	createPreparator := func(t *testing.T, sshKey string) (*MetaConfigPreparator, *config.MetaConfig, *log.InMemoryLogger) {
		c := metaConfigForPrepare(t, sshKey)
		logger := log.NewInMemoryLoggerWithParent(log.GetDefaultLogger())
		preparator := NewMetaConfigPreparator()
		preparator.WithLogger(logger)

		return preparator, c, logger
	}

	setOriginCfg := func(preparator *MetaConfigPreparator, originCfg string) {
		preparator.WithAdditionalData(NewPreparatorAdditionalData(originCfg))
	}

	assertMetaConfig := func(t *testing.T, cfg *config.MetaConfig, sshKey string) {
		require.Len(t, cfg.ProviderClusterConfig, 2, "should not delete and add another keys")
		layout := json.RawMessage(fmt.Sprintf(`"%s"`, testLayout))
		require.Equal(t, layout, cfg.ProviderClusterConfig["layout"], "should not change another keys")
		require.Contains(t, cfg.ProviderClusterConfig, "sshPublicKey", "should contains sshPublicKey")

		sshKeyFromCfgRaw := cfg.ProviderClusterConfig["sshPublicKey"]
		var sshKeyFromConfig string
		err := json.Unmarshal(sshKeyFromCfgRaw, &sshKeyFromConfig)
		require.NoError(t, err, "ssh key from config should unmarshaled")

		require.Equal(t, sshKey, sshKeyFromConfig, "should has correct ssh key")
	}

	assertLog := func(t *testing.T, logger *log.InMemoryLogger, msg string) {
		find, err := logger.FirstMatch(&log.Match{
			Prefix: []string{msg},
		})

		require.NoError(t, err)
		require.NotEmpty(t, find, "log '%s' should be found", msg)
	}

	ctx := context.TODO()

	t.Run("No additional data", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		preparator, cfg, logger := createPreparator(t, sshKey)
		err := preparator.Prepare(ctx, cfg)

		require.NoError(t, err, "should prepared")
		assertMetaConfig(t, cfg, sshKey)
		assertLog(t, logger, "Additional data for cloud provider dvp not provided")
	})

	t.Run("Same key", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		preparator, cfg, logger := createPreparator(t, sshKey)

		original := fmt.Sprintf(`
layout: Standard
sshPublicKey: "%s"
`, sshKey)

		setOriginCfg(preparator, original)

		err := preparator.Prepare(ctx, cfg)

		require.NoError(t, err, "should prepared")
		assertMetaConfig(t, cfg, sshKey)
		assertLog(t, logger, "Meta config ssh pub key equals to original ssh pub key")
	})

	t.Run("MetaConfig key contains new line", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		keyWithNewLine := fmt.Sprintf("%s\n", sshKey)
		preparator, cfg, logger := createPreparator(t, keyWithNewLine)

		original := fmt.Sprintf(`
layout: Standard
sshPublicKey: "%s"
`, sshKey)

		setOriginCfg(preparator, original)

		err := preparator.Prepare(ctx, cfg)

		require.NoError(t, err, "should prepared")
		assertMetaConfig(t, cfg, keyWithNewLine)
		assertLog(t, logger, "Meta config ssh pub key already contains new line")
	})

	t.Run("Meta config key not found", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		preparator, cfg, logger := createPreparator(t, sshKey)
		delete(cfg.ProviderClusterConfig, "sshPublicKey")

		original := `
layout: Standard
sshPublicKey: |
  ssh-rsa CCCCCC
`

		setOriginCfg(preparator, original)

		err := preparator.Prepare(ctx, cfg)

		require.NoError(t, err, "should prepared")
		require.Len(t, cfg.ProviderClusterConfig, 1, "should not change meta config")
		assertLog(t, logger, "Is not provided key")
	})

	t.Run("Skip original key not found", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		preparator, cfg, logger := createPreparator(t, sshKey)

		original := `
layout: Standard
zones:
- default
`

		setOriginCfg(preparator, original)

		err := preparator.Prepare(ctx, cfg)

		require.NoError(t, err, "should prepared")
		assertMetaConfig(t, cfg, sshKey)
		assertLog(t, logger, "Original provider cluster config does not contains ssh pub key")
	})

	t.Run("Skip original config not provided", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		preparator, cfg, logger := createPreparator(t, sshKey)

		setOriginCfg(preparator, "")

		err := preparator.Prepare(ctx, cfg)

		require.NoError(t, err, "should prepared")
		assertMetaConfig(t, cfg, sshKey)
		assertLog(t, logger, "Original provider cluster config yaml key not provided")
	})

	t.Run("Same key when original with multiline new line use original", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		preparator, cfg, _ := createPreparator(t, sshKey)

		original := fmt.Sprintf(`
layout: Standard
sshPublicKey: |
  %s
`, sshKey)

		setOriginCfg(preparator, original)

		err := preparator.Prepare(ctx, cfg)

		require.NoError(t, err, "should prepared")
		assertMetaConfig(t, cfg, fmt.Sprintf("%s\n", sshKey))
	})

	t.Run("Same key when original with multiline new line in middle use original", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		preparator, cfg, _ := createPreparator(t, sshKey)

		original := fmt.Sprintf(`
layout: Standard
sshPublicKey: |
  %s
zones:
- default`, sshKey)

		setOriginCfg(preparator, original)

		err := preparator.Prepare(ctx, cfg)

		require.NoError(t, err, "should prepared")
		assertMetaConfig(t, cfg, fmt.Sprintf("%s\n", sshKey))
	})

	t.Run("Different keys use meta config key", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		preparator, cfg, logger := createPreparator(t, sshKey)

		// also test that another keys not changed
		original := fmt.Sprintf(`
layout: NonStandard
sshPublicKey: |
  %s
`, "ssh-rsa BBBBBB")

		setOriginCfg(preparator, original)

		err := preparator.Prepare(ctx, cfg)

		require.NoError(t, err, "should prepared")
		assertMetaConfig(t, cfg, sshKey)
		assertLog(t, logger, "Original trimmed ssh key not equal to meta config ssh pub key")
	})

	t.Run("Original unmarshal error", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		preparator, cfg, _ := createPreparator(t, sshKey)

		// also test that another keys not changed
		original := `3"sntgt`

		setOriginCfg(preparator, original)

		err := preparator.Prepare(ctx, cfg)

		require.Error(t, err, "should not prepared")
	})

	t.Run("Meta config unmarshal error", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		preparator, cfg, _ := createPreparator(t, sshKey)

		cfg.ProviderClusterConfig["sshPublicKey"] = json.RawMessage([]byte(`3"rfrifj`))

		// also test that another keys not changed
		original := `
layout: Standard
sshPublicKey: |
  ssh-rsa CCCCC
`

		setOriginCfg(preparator, original)

		err := preparator.Prepare(ctx, cfg)

		require.Error(t, err, "should not prepared")
	})
}

const testLayout = "Standard"

func metaConfigForPrepare(t *testing.T, sshKey string) *config.MetaConfig {
	sshKeyRaw, err := json.Marshal(sshKey)
	require.NoError(t, err, "cannot marshal ssh pub key")

	nsRaw, err := json.Marshal(testLayout)
	require.NoError(t, err, "cannot marshal namespace")

	return &config.MetaConfig{
		ProviderClusterConfig: map[string]json.RawMessage{
			"sshPublicKey": sshKeyRaw,
			"layout":       nsRaw,
		},
	}
}

func metaConfigWithProvider(t *testing.T, spec DVPProviderSpec) *config.MetaConfig {
	raw, err := json.Marshal(spec)
	require.NoError(t, err)

	return &config.MetaConfig{
		ProviderClusterConfig: map[string]json.RawMessage{
			"provider": raw,
		},
	}
}

func testKubeconfig() string {
	return `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://flat.com
    insecure-skip-tls-verify: true
contexts:
- name: c
  context:
    cluster: c
    user: u
users:
- name: u
  user:
    token: bobobbob==
current-context: c
`
}
