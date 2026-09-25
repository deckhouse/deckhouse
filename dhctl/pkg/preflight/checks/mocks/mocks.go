// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mocks

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"

	libcon "github.com/deckhouse/lib-connection/pkg"
)

type MockNodeInterface struct {
	mock.Mock
}

func (m *MockNodeInterface) Command(name string, arg ...string) libcon.Command {
	args := m.Called(name, arg)
	return args.Get(0).(libcon.Command)
}

func (m *MockNodeInterface) File() libcon.File {
	args := m.Called()
	return args.Get(0).(libcon.File)
}

func (m *MockNodeInterface) UploadScript(scriptPath string, args ...string) libcon.Script {
	mockArgs := m.Called(scriptPath, args)
	return mockArgs.Get(0).(libcon.Script)
}

type MockScript struct {
	mock.Mock
}

func (m *MockScript) Execute(ctx context.Context) ([]byte, error) {
	args := m.Called(ctx)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockScript) ExecuteBundle(ctx context.Context, parentDir, bundleDir string) ([]byte, error) {
	args := m.Called(ctx, parentDir, bundleDir)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockScript) Sudo() {
	m.Called()
}

func (m *MockScript) WithStdoutHandler(handler func(string)) {
	m.Called(handler)
}

func (m *MockScript) WithTimeout(timeout time.Duration) {
	m.Called(timeout)
}

func (m *MockScript) WithEnvs(envs map[string]string) {
	m.Called(envs)
}

func (m *MockScript) WithCleanupAfterExec(doCleanup bool) {
	m.Called(doCleanup)
}

func (m *MockScript) WithCommanderMode(enabled bool) {
	m.Called(enabled)
}

func (m *MockScript) WithExecuteUploadDir(dir string) {
	m.Called(dir)
}

func (m *MockScript) WithNoLogStepOutOnError(enabled bool) {
	m.Called(enabled)
}

func (m *MockScript) WithBundlerOpts(opts ...libcon.BundlerOption) {
	m.Called(opts)
}

type MockCommand struct {
	mock.Mock
}

func (m *MockCommand) Run(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockCommand) Cmd(ctx context.Context) {
	m.Called(ctx)
}

func (m *MockCommand) Sudo(ctx context.Context) {
	m.Called(ctx)
}

func (m *MockCommand) StdoutBytes() []byte {
	args := m.Called()
	return args.Get(0).([]byte)
}

func (m *MockCommand) StderrBytes() []byte {
	args := m.Called()
	return args.Get(0).([]byte)
}

func (m *MockCommand) Output(ctx context.Context) ([]byte, []byte, error) {
	args := m.Called(ctx)
	return args.Get(0).([]byte), args.Get(1).([]byte), args.Error(2)
}

func (m *MockCommand) CombinedOutput(ctx context.Context) ([]byte, error) {
	args := m.Called(ctx)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockCommand) OnCommandStart(fn func()) {
	m.Called(fn)
}

func (m *MockCommand) WithEnv(env map[string]string) {
	m.Called(env)
}

func (m *MockCommand) WithTimeout(timeout time.Duration) {
	m.Called(timeout)
}

func (m *MockCommand) WithStdoutHandler(h func(line string)) {
	m.Called(h)
}

func (m *MockCommand) WithStderrHandler(h func(line string)) {
	m.Called(h)
}

func (m *MockCommand) WithSSHArgs(args ...string) {
	m.Called(args)
}

type MockSession struct {
	mock.Mock
}

func (m *MockSession) AvailableHosts() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

type MockNodeInterfaceWrapper struct {
	mock.Mock
	client libcon.SSHClient
}

func (m *MockNodeInterfaceWrapper) Command(name string, arg ...string) libcon.Command {
	args := m.Called(name, arg)
	return args.Get(0).(libcon.Command)
}

func (m *MockNodeInterfaceWrapper) File() libcon.File {
	args := m.Called()
	return args.Get(0).(libcon.File)
}

func (m *MockNodeInterfaceWrapper) UploadScript(scriptPath string, args ...string) libcon.Script {
	mockArgs := m.Called(scriptPath, args)
	return mockArgs.Get(0).(libcon.Script)
}

func (m *MockNodeInterfaceWrapper) Client() libcon.SSHClient {
	return m.client
}

type MockState struct {
	mock.Mock
}

func (m *MockState) SetGlobalPreflightchecksWasRan() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockState) GlobalPreflightchecksWasRan() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *MockState) SetCloudPreflightchecksWasRan() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockState) SetPostCloudPreflightchecksWasRan() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockState) CloudPreflightchecksWasRan() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *MockState) PostCloudPreflightchecksWasRan() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *MockState) SetStaticPreflightchecksWasRan() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockState) StaticPreflightchecksWasRan() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}
