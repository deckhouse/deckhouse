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
	type preparatorTest struct {
		preparator *MetaConfigPreparator
		cfg        *config.MetaConfig
		logger     *log.InMemoryLogger
	}

	setOriginCfg := func(preparator *MetaConfigPreparator, originCfg string) {
		preparator.WithAdditionalData(NewPreparatorAdditionalData(originCfg))
	}

	createPreparator := func(t *testing.T, sshKey, original string) *preparatorTest {
		c := metaConfigForPrepare(t, sshKey)
		logger := log.NewInMemoryLoggerWithParent(log.GetDefaultLogger())
		preparator := NewMetaConfigPreparator()
		preparator.WithLogger(logger)

		if original != "" {
			setOriginCfg(preparator, original)
		}

		return &preparatorTest{
			preparator: preparator,
			cfg:        c,
			logger:     logger,
		}
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

	assertSkipPrepare := func(t *testing.T, test *preparatorTest, sshKey, logMsg string) {
		cfg := test.cfg

		err := test.preparator.Prepare(context.TODO(), cfg)

		require.NoError(t, err, "should prepared with skip")
		assertMetaConfig(t, cfg, sshKey)
		if logMsg != "" {
			assertLog(t, test.logger, logMsg)
		}
	}

	assertSkipWithProviderConfigLen := func(t *testing.T, test *preparatorTest, l int, logMsg string) {
		cfg := test.cfg

		err := test.preparator.Prepare(context.TODO(), cfg)

		require.NoError(t, err, "should skip prepared")
		require.Len(t, cfg.ProviderClusterConfig, l, "should not change meta config")
		assertLog(t, test.logger, logMsg)
	}

	assertPrepared := func(t *testing.T, test *preparatorTest, sshKey string) {
		cfg := test.cfg

		err := test.preparator.Prepare(context.TODO(), cfg)

		require.NoError(t, err, "should prepared")
		assertMetaConfig(t, cfg, sshKey)
	}

	assertNotPrepared := func(t *testing.T, test *preparatorTest) {
		err := test.preparator.Prepare(context.TODO(), test.cfg)

		require.Error(t, err, "should not prepared")
	}

	t.Run("No additional data", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"

		test := createPreparator(t, sshKey, "")

		assertSkipPrepare(t, test, sshKey, "Additional data for cloud provider dvp not provided")
	})

	t.Run("Same key", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		original := fmt.Sprintf(`
layout: Standard
sshPublicKey: "%s"
`, sshKey)

		test := createPreparator(t, sshKey, original)

		assertSkipPrepare(t, test, sshKey, "Meta config ssh pub key equals to original ssh pub key")
	})

	t.Run("MetaConfig key contains new line", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		keyWithNewLine := fmt.Sprintf("%s\n", sshKey)
		original := fmt.Sprintf(`
layout: Standard
sshPublicKey: "%s"
`, sshKey)

		test := createPreparator(t, keyWithNewLine, original)

		assertSkipPrepare(t, test, keyWithNewLine, "Meta config ssh pub key already contains new line")
	})

	t.Run("Meta config key not found", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		original := `
layout: Standard
sshPublicKey: |
  ssh-rsa CCCCCC
`
		test := createPreparator(t, sshKey, original)
		delete(test.cfg.ProviderClusterConfig, "sshPublicKey")

		setOriginCfg(test.preparator, original)

		assertSkipWithProviderConfigLen(t, test, 1, "Meta config not provide key")
	})

	t.Run("Skip original key not found", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		original := `
layout: Standard
zones:
- default
`

		test := createPreparator(t, sshKey, original)

		assertSkipPrepare(t, test, sshKey, "Original provider cluster config does not contains ssh pub key")
	})

	t.Run("Skip original config not provided", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		test := createPreparator(t, sshKey, "")

		setOriginCfg(test.preparator, "")

		assertSkipPrepare(t, test, sshKey, "Original provider cluster config yaml key not provided")
	})

	t.Run("Skip provider config not provided in meta config", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		original := `
layout: Standard
sshPublicKey: |
  ssh-rsa CCCCCC
`
		test := createPreparator(t, sshKey, original)

		test.cfg.ProviderClusterConfig = nil

		assertSkipWithProviderConfigLen(t, test, 0, "Provider cluster config not provided")
	})

	t.Run("Same key when original with multiline new line use original", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		original := fmt.Sprintf(`
layout: Standard
sshPublicKey: |
  %s
`, sshKey)

		test := createPreparator(t, sshKey, original)

		assertPrepared(t, test, fmt.Sprintf("%s\n", sshKey))
	})

	t.Run("Same key when original with multiline new line in middle use original", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		original := fmt.Sprintf(`
layout: Standard
sshPublicKey: |
  %s
zones:
- default`, sshKey)

		test := createPreparator(t, sshKey, original)

		assertPrepared(t, test, fmt.Sprintf("%s\n", sshKey))
	})

	t.Run("Different keys use meta config key", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		// also test that another keys not changed
		original := fmt.Sprintf(`
layout: NonStandard
sshPublicKey: |
  %s
`, "ssh-rsa BBBBBB")

		test := createPreparator(t, sshKey, original)

		assertSkipPrepare(t, test, sshKey, "Original trimmed ssh key not equal to meta config ssh pub key")
	})

	t.Run("Original unmarshal error", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		original := `3"sntgt`

		test := createPreparator(t, sshKey, original)

		assertNotPrepared(t, test)
	})

	t.Run("Meta config unmarshal error", func(t *testing.T) {
		const sshKey = "ssh-rsa AAAAAA"
		original := `
layout: Standard
sshPublicKey: |
  ssh-rsa CCCCC
`
		test := createPreparator(t, sshKey, original)

		test.cfg.ProviderClusterConfig["sshPublicKey"] = json.RawMessage([]byte(`3"rfrifj`))

		assertNotPrepared(t, test)
	})
}

const testLayout = "Standard"

func metaConfigForPrepare(t *testing.T, sshKey string) *config.MetaConfig {
	sshKeyRaw, err := json.Marshal(sshKey)
	require.NoError(t, err, "cannot marshal ssh pub key")

	layoutRaw, err := json.Marshal(testLayout)
	require.NoError(t, err, "cannot marshal namespace")

	return &config.MetaConfig{
		ProviderClusterConfig: map[string]json.RawMessage{
			"sshPublicKey": sshKeyRaw,
			"layout":       layoutRaw,
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
