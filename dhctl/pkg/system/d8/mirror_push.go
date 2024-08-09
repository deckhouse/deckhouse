// Copyright 2024 Flant JSC
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

package d8

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/process"
)

type MirrorPush struct {
	*process.Executor

	mirrorCmd *exec.Cmd

	auth     bool
	username string
	password string

	insecure      bool
	tlsSkipVerify bool
}

func NewMirrorPush() *MirrorPush {
	return &MirrorPush{}
}

func (m *MirrorPush) WithRegistryAuth(username, password string) *MirrorPush {
	m.auth = true
	m.username = username
	m.password = password
	return m
}

func (m *MirrorPush) WithInsecure(insecure bool) *MirrorPush {
	m.insecure = insecure
	return m
}

func (m *MirrorPush) WithTlsSkipVerify(tlsSkipVerify bool) *MirrorPush {
	m.tlsSkipVerify = tlsSkipVerify
	return m
}

func (m *MirrorPush) MirrorPush(imagesBundlePath, registryAddress string) *MirrorPush {
	env := os.Environ()
	args := []string{
		"mirror",
		"push",
	}

	if m.auth {
		args = append(args, fmt.Sprintf("--registry-login=%s", m.username))
		args = append(args, fmt.Sprintf("--registry-password=%s", m.password))
	}

	if m.insecure {
		args = append(args, "--insecure")
	}

	if m.tlsSkipVerify {
		args = append(args, "--tls-skip-verify")
	}

	if app.IsDebug {
		env = append(env, "MIRROR_DEBUG_LOG=3")
	}

	args = append(
		args,
		imagesBundlePath,
		fmt.Sprintf("%s/sys/deckhouse", registryAddress),
	)

	m.mirrorCmd = exec.Command("/deckhouse/candi/d8", args...)
	m.mirrorCmd.Env = env
	m.Executor = process.NewDefaultExecutor(m.mirrorCmd)
	return m
}
