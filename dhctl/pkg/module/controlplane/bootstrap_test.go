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

package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	"github.com/deckhouse/lib-connection/pkg/ssh/testssh"
	"github.com/deckhouse/lib-dhctl/pkg/log"
	"github.com/deckhouse/lib-dhctl/pkg/retry"
)

func TestPrepare(t *testing.T) {
	assertCreateSignDir := func(t *testing.T, cmds []*testCmdRun) {
		expectedArgs := []string{
			"-c",
			"umask 0022 ; mkdir -p -m 1777 /opt/deckhouse/signature",
		}

		require.Len(t, cmds, 1, "should not run 1 command got: %v", cmds)

		c := cmds[0]
		require.Equal(t, "sh", c.cmd, "should run sh")
		require.Equal(t, expectedArgs, c.args, "should run sh with args")
		require.True(t, c.ran, "command should ran")
	}

	type key struct {
		Kty string `json:"kty,omitempty"`
		Crv string `json:"crv,omitempty"`
		Use string `json:"use,omitempty"`
		Kid string `json:"kid,omitempty"`
		X   string `json:"x,omitempty"`
		D   string `json:"d,omitempty"`
		Alg string `json:"alg,omitempty"`
	}

	assertGeneralFieldsKey := func(t *testing.T, k *key, msg string) {
		require.Equal(t, "OKP", k.Kty, "%s should have correct kty", msg)
		require.Equal(t, "EdDSA", k.Alg, "%s should have correct alg", msg)
		require.Equal(t, "Ed25519", k.Crv, "%s should have correct crv", msg)
		require.Equal(t, "sig", k.Use, "%s should have correct use", msg)
		require.NotEmpty(t, k.Kid, "%s should have kid", msg)
		require.NotEmpty(t, k.X, "%s should have x", msg)
	}

	assertPrivateKey := func(t *testing.T, content []byte) {
		require.NotEmpty(t, content, "private key should have content")

		k := key{}
		err := json.Unmarshal(content, &k)
		require.NoError(t, err, "should unmarshal private key")

		assertGeneralFieldsKey(t, &k, "private key")
		require.NotEmpty(t, k.D, "private key should have d")
	}

	assertPublicKey := func(t *testing.T, content []byte) {
		require.NotEmpty(t, content, "public key should have content")

		type jwks struct {
			Keys []*key `json:"keys,omitempty"`
		}

		j := jwks{}
		err := json.Unmarshal(content, &j)
		require.NoError(t, err, "should unmarshal jwks public key")

		require.Len(t, j.Keys, 1, "should have one public key in jwks")

		k := j.Keys[0]
		assertGeneralFieldsKey(t, k, "public key")
		require.Empty(t, k.D, "public key should expose d")
	}

	assertConfig := func(t *testing.T, content []byte, mode string) {
		require.NotEmpty(t, content, "config should have content")

		ec := EncryptionConfiguration{}
		err := yaml.Unmarshal(content, &ec)
		require.NoError(t, err, "should unmarshal config")

		require.Equal(
			t,
			"apiserver.config.k8s.io/v1",
			ec.APIVersion,
			"config should have correct api version",
		)
		require.Equal(
			t,
			"EncryptionConfiguration",
			ec.Kind,
			"config should have correct kind",
		)
		require.Equal(
			t,
			mode,
			ec.Signature.Mode,
			"config should have correct mode",
		)
		require.Equal(
			t,
			"/etc/kubernetes/pki/signature-private.jwk",
			ec.Signature.PrivKeyPath,
			"config should have correct private key path",
		)
		require.Equal(
			t,
			"/etc/kubernetes/pki/signature-public.jwks",
			ec.Signature.PubKeyPath,
			"config should have correct public key path",
		)
	}

	assertUploadKeyAndConf := func(t *testing.T, m map[string][]byte, mode string) {
		require.Len(t, m, 3, "should not 3 uploads got: %v", m)
		const (
			keyPath    = "/opt/deckhouse/signature/signature-private.jwk"
			jwksPath   = "/opt/deckhouse/signature/signature-public.jwks"
			configPath = "/opt/deckhouse/signature/encryption-config.yaml"
		)

		require.Contains(t, m, keyPath, "should upload private key")
		require.Contains(t, m, jwksPath, "should upload public key")
		require.Contains(t, m, configPath, "should upload config")

		assertPrivateKey(t, m[keyPath])
		assertPublicKey(t, m[jwksPath])
		assertConfig(t, m[configPath], mode)
	}

	assertNoUploads := func(t *testing.T, m map[string][]byte) {
		require.Len(t, m, 0, "should not any uploads got: %v", m)
	}

	tests := []*bootstrapTest{
		{
			name:          "no signature mode",
			mode:          NoSignatureMode,
			isErr:         false,
			assertUploads: assertNoUploads,
			assertCommands: func(t *testing.T, cmds []*testCmdRun) {
				require.Len(t, cmds, 0, "should not any commands got: %v", cmds)
			},
		},

		{
			name:  "default signature mode",
			mode:  "Migrate",
			isErr: false,
			assertUploads: func(t *testing.T, m map[string][]byte) {
				assertUploadKeyAndConf(t, m, "migrate")
			},
			assertCommands: assertCreateSignDir,
		},

		{
			name:  "not default signature mode",
			mode:  "Enforce",
			isErr: false,
			assertUploads: func(t *testing.T, m map[string][]byte) {
				assertUploadKeyAndConf(t, m, "enforce")
			},
			assertCommands: assertCreateSignDir,
		},

		{
			name:           "create dir err",
			mode:           "Enforce",
			commandErr:     fmt.Errorf("not create"),
			isErr:          true,
			assertUploads:  assertNoUploads,
			assertCommands: assertCreateSignDir,
		},
		{
			name:       "upload err",
			mode:       "Enforce",
			uploadsErr: fmt.Errorf("not copy"),
			isErr:      true,
			assertUploads: func(t *testing.T, m map[string][]byte) {
				require.True(t, len(m) != 3, "not upload all files with error got %d", len(m))
			},
			assertCommands: assertCreateSignDir,
		},
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			assertError := require.NoError
			if tst.isErr {
				assertError = require.Error
			}

			logger := log.NewInMemoryLoggerWithParent(log.NewDummyLogger(true))

			s := newTestModuleSettings(tst.mode)

			n := tst.node(t)

			preparator := NewBootstrapPreparator(s, n, log.SimpleLoggerProvider(logger))
			preparator.WithLoopsParams(LoopsParams{
				CreateSigDir: retry.NewEmptyParams(),
			})

			err := preparator.PrepareModule(context.TODO())
			assertError(t, err)

			tst.assertUploads(t, tst.uploads)
			tst.assertCommands(t, tst.commands)
		})
	}
}

type testCmdRun struct {
	cmd  string
	args []string
	ran  bool
}

type bootstrapTest struct {
	name           string
	mode           string
	isErr          bool
	uploads        map[string][]byte
	uploadsErr     error
	assertUploads  func(*testing.T, map[string][]byte)
	commands       []*testCmdRun
	commandErr     error
	assertCommands func(*testing.T, []*testCmdRun)
}

func (b *bootstrapTest) node(t *testing.T) libcon.Interface {
	host := "127.0.0.1"
	cl := testssh.NewClient(session.NewSession(session.Input{
		AvailableHosts: []session.Host{{
			Host: host,
			Name: "localhost",
		}},
	}), []session.AgentPrivateKey{})

	b.uploads = make(map[string][]byte)

	upload := func(data []byte, dstPath string) error {
		if b.uploadsErr != nil {
			return b.uploadsErr
		}

		b.uploads[dstPath] = data
		return nil
	}

	download := func(srcPath string) ([]byte, error) {
		return nil, fmt.Errorf("Download should not call for %s", srcPath)
	}

	file := testssh.NewFile(upload, download)

	cl.SetFileProvider(host, func(testssh.Bastion) *testssh.File {
		return file
	})

	cl.AddCommandProvider(host, func(_ testssh.Bastion, c string, args ...string) *testssh.Command {
		r := &testCmdRun{
			cmd:  c,
			args: append([]string{}, args...),
		}

		b.commands = append(b.commands, r)

		return testssh.NewCommand(nil).WithRun(func() {
			r.ran = true
		}).WithErr(b.commandErr)
	})

	err := cl.Start()
	require.NoError(t, err, "client should started")

	return cl
}

type testModuleSettings struct {
	mode string
}

func newTestModuleSettings(m string) *testModuleSettings {
	return &testModuleSettings{
		mode: m,
	}
}

func (s *testModuleSettings) SignatureMode() (string, error) {
	return s.mode, nil
}
