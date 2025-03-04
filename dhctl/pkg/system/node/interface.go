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

package node

import (
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

type Interface interface {
	Command(name string, args ...string) Command
	File() File
	UploadScript(scriptPath string, args ...string) Script
}

type Command interface {
	Run() error
	Cmd()
	Sudo()

	StdoutBytes() []byte
	StderrBytes() []byte
	Output() ([]byte, []byte, error)
	CombinedOutput() ([]byte, error)

	OnCommandStart(fn func())
	WithEnv(env map[string]string)
	WithTimeout(timeout time.Duration)
	WithStdoutHandler(h func(line string))
	WithStderrHandler(h func(line string))
	WithSSHArgs(args ...string)
}

type File interface {
	Upload(srcPath, dstPath string) error
	Download(srcPath, dstPath string) error

	UploadBytes(data []byte, remotePath string) error
	DownloadBytes(remotePath string) ([]byte, error)
}

type Script interface {
	Execute() (stdout []byte, err error)
	ExecuteBundle(parentDir, bundleDir string) (stdout []byte, err error)

	Sudo()
	WithStdoutHandler(handler func(string))
	WithTimeout(timeout time.Duration)
	WithEnvs(envs map[string]string)
	WithCleanupAfterExec(doCleanup bool)
}

type Tunnel interface {
	Up() error

	HealthMonitor(errorOutCh chan<- error)

	Stop()

	String() string
}

type ReverseTunnelChecker interface {
	CheckTunnel() (string, error)
}

type ReverseTunnelKiller interface {
	KillTunnel() (string, error)
}

type ReverseTunnel interface {
	Up() error

	StartHealthMonitor(checker ReverseTunnelChecker, killer ReverseTunnelKiller)

	Stop()

	String() string
}

type KubeProxy interface {
	Start(useLocalPort int) (port string, err error)

	StopAll()

	Stop(startID int)
}

type Check interface {
	WithDelaySeconds(seconds int) Check

	AwaitAvailability() error

	CheckAvailability() error

	ExpectAvailable() ([]byte, error)

	String() string
}

type SSHLoopHandler func(s SSHClient) error

type SSHClient interface {
	Start() error

	// Tunnel is used to open local (L) and remote (R) tunnels
	Tunnel(address string) Tunnel

	// ReverseTunnel is used to open remote (R) tunnel
	ReverseTunnel(address string) ReverseTunnel

	// Command is used to run commands on remote server
	Command(name string, arg ...string) Command

	// KubeProxy is used to start kubectl proxy and create a tunnel from local port to proxy port
	KubeProxy() KubeProxy

	// File is used to upload and download files and directories
	File() File

	// UploadScript is used to upload script and execute it on remote server
	UploadScript(scriptPath string, args ...string) Script

	// UploadScript is used to upload script and execute it on remote server
	Check() Check

	// Stop the client
	Stop()

	// Loop Looping all available hosts
	Loop(fn SSHLoopHandler) error

	Session() *session.Session

	PrivateKeys() []session.AgentPrivateKey
}
