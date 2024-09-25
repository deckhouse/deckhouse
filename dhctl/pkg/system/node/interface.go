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

import "time"

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
