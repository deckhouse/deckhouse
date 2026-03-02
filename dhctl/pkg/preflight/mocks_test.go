// Copyright 2025 Flant JSC
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

package preflight

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

type mockNodeInterface struct {
	mock.Mock
}

func (m *mockNodeInterface) Command(name string, arg ...string) node.Command {
	args := m.Called(name, arg)
	return args.Get(0).(node.Command)
}

func (m *mockNodeInterface) File() node.File {
	args := m.Called()
	return args.Get(0).(node.File)
}

func (m *mockNodeInterface) UploadScript(scriptPath string, args ...string) node.Script {
	mockArgs := m.Called(scriptPath, args)
	return mockArgs.Get(0).(node.Script)
}

type mockScript struct {
	mock.Mock
}

func (m *mockScript) Execute(ctx context.Context) ([]byte, error) {
	args := m.Called(ctx)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockScript) ExecuteBundle(ctx context.Context, parentDir, bundleDir string) ([]byte, error) {
	args := m.Called(ctx, parentDir, bundleDir)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockScript) Sudo() {
	m.Called()
}

func (m *mockScript) WithStdoutHandler(handler func(string)) {
	m.Called(handler)
}

func (m *mockScript) WithTimeout(timeout time.Duration) {
	m.Called(timeout)
}

func (m *mockScript) WithEnvs(envs map[string]string) {
	m.Called(envs)
}

func (m *mockScript) WithCleanupAfterExec(doCleanup bool) {
	m.Called(doCleanup)
}

func (m *mockScript) WithCommanderMode(enabled bool) {
	m.Called(enabled)
}

func (m *mockScript) WithExecuteUploadDir(dir string) {
	m.Called(dir)
}

type mockCommand struct {
	mock.Mock
}

func (m *mockCommand) Run(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockCommand) Cmd(ctx context.Context) {
	m.Called(ctx)
}

func (m *mockCommand) Sudo(ctx context.Context) {
	m.Called(ctx)
}

func (m *mockCommand) StdoutBytes() []byte {
	args := m.Called()
	return args.Get(0).([]byte)
}

func (m *mockCommand) StderrBytes() []byte {
	args := m.Called()
	return args.Get(0).([]byte)
}

func (m *mockCommand) Output(ctx context.Context) ([]byte, []byte, error) {
	args := m.Called(ctx)
	return args.Get(0).([]byte), args.Get(1).([]byte), args.Error(2)
}

func (m *mockCommand) CombinedOutput(ctx context.Context) ([]byte, error) {
	args := m.Called(ctx)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockCommand) OnCommandStart(fn func()) {
	m.Called(fn)
}

func (m *mockCommand) WithEnv(env map[string]string) {
	m.Called(env)
}

func (m *mockCommand) WithTimeout(timeout time.Duration) {
	m.Called(timeout)
}

func (m *mockCommand) WithStdoutHandler(h func(line string)) {
	m.Called(h)
}

func (m *mockCommand) WithStderrHandler(h func(line string)) {
	m.Called(h)
}

func (m *mockCommand) WithSSHArgs(args ...string) {
	m.Called(args)
}

type mockSSHClient struct {
	mock.Mock
}

func (m *mockSSHClient) OnlyPreparePrivateKeys() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockSSHClient) Start() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockSSHClient) Tunnel(address string) node.Tunnel {
	args := m.Called(address)
	return args.Get(0).(node.Tunnel)
}

func (m *mockSSHClient) ReverseTunnel(address string) node.ReverseTunnel {
	args := m.Called(address)
	return args.Get(0).(node.ReverseTunnel)
}

func (m *mockSSHClient) Command(name string, arg ...string) node.Command {
	args := m.Called(name, arg)
	return args.Get(0).(node.Command)
}

func (m *mockSSHClient) KubeProxy() node.KubeProxy {
	args := m.Called()
	return args.Get(0).(node.KubeProxy)
}

func (m *mockSSHClient) File() node.File {
	args := m.Called()
	return args.Get(0).(node.File)
}

func (m *mockSSHClient) UploadScript(scriptPath string, args ...string) node.Script {
	mockArgs := m.Called(scriptPath, args)
	return mockArgs.Get(0).(node.Script)
}

func (m *mockSSHClient) Check() node.Check {
	args := m.Called()
	return args.Get(0).(node.Check)
}

func (m *mockSSHClient) Stop() {
	m.Called()
}

func (m *mockSSHClient) Loop(fn node.SSHLoopHandler) error {
	args := m.Called(fn)
	return args.Error(0)
}

func (m *mockSSHClient) Session() *session.Session {
	args := m.Called()
	return args.Get(0).(*session.Session)
}

func (m *mockSSHClient) PrivateKeys() []session.AgentPrivateKey {
	args := m.Called()
	return args.Get(0).([]session.AgentPrivateKey)
}

func (m *mockSSHClient) RefreshPrivateKeys() error {
	args := m.Called()
	return args.Error(0)
}

type mockCheck struct {
	mock.Mock
}

func (m *mockCheck) WithDelaySeconds(seconds int) node.Check {
	args := m.Called(seconds)
	return args.Get(0).(node.Check)
}

func (m *mockCheck) AwaitAvailability(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockCheck) CheckAvailability(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockCheck) ExpectAvailable(ctx context.Context) ([]byte, error) {
	args := m.Called(ctx)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockCheck) String() string {
	args := m.Called()
	return args.String(0)
}

type mockSession struct {
	mock.Mock
}

func (m *mockSession) AvailableHosts() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

type mockReverseTunnel struct {
	mock.Mock
}

func (m *mockReverseTunnel) Up() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockReverseTunnel) StartHealthMonitor(ctx context.Context, checker node.ReverseTunnelChecker, killer node.ReverseTunnelKiller) {
	m.Called(ctx, checker, killer)
}

func (m *mockReverseTunnel) Stop() {
	m.Called()
}

func (m *mockReverseTunnel) String() string {
	args := m.Called()
	return args.String(0)
}

type mockNodeInterfaceWrapper struct {
	mock.Mock
	client node.SSHClient
}

func (m *mockNodeInterfaceWrapper) Command(name string, arg ...string) node.Command {
	args := m.Called(name, arg)
	return args.Get(0).(node.Command)
}

func (m *mockNodeInterfaceWrapper) File() node.File {
	args := m.Called()
	return args.Get(0).(node.File)
}

func (m *mockNodeInterfaceWrapper) UploadScript(scriptPath string, args ...string) node.Script {
	mockArgs := m.Called(scriptPath, args)
	return mockArgs.Get(0).(node.Script)
}

func (m *mockNodeInterfaceWrapper) Client() node.SSHClient {
	return m.client
}

type mockState struct {
	mock.Mock
}

func (m *mockState) SetGlobalPreflightchecksWasRan() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockState) GlobalPreflightchecksWasRan() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *mockState) SetCloudPreflightchecksWasRan() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockState) SetPostCloudPreflightchecksWasRan() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockState) CloudPreflightchecksWasRan() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *mockState) PostCloudPreflightchecksWasRan() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *mockState) SetStaticPreflightchecksWasRan() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockState) StaticPreflightchecksWasRan() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}
