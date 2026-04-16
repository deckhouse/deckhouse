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
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
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

type MockSSHClient struct {
	mock.Mock
}

func (m *MockSSHClient) OnlyPreparePrivateKeys() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHClient) Start() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHClient) Tunnel(address string) libcon.Tunnel {
	args := m.Called(address)
	return args.Get(0).(libcon.Tunnel)
}

func (m *MockSSHClient) ReverseTunnel(address string) libcon.ReverseTunnel {
	args := m.Called(address)
	return args.Get(0).(libcon.ReverseTunnel)
}

func (m *MockSSHClient) Command(name string, arg ...string) libcon.Command {
	args := m.Called(name, arg)
	return args.Get(0).(libcon.Command)
}

func (m *MockSSHClient) KubeProxy() libcon.KubeProxy {
	args := m.Called()
	return args.Get(0).(libcon.KubeProxy)
}

func (m *MockSSHClient) File() libcon.File {
	args := m.Called()
	return args.Get(0).(libcon.File)
}

func (m *MockSSHClient) UploadScript(scriptPath string, args ...string) libcon.Script {
	mockArgs := m.Called(scriptPath, args)
	return mockArgs.Get(0).(libcon.Script)
}

func (m *MockSSHClient) Check() libcon.Check {
	args := m.Called()
	return args.Get(0).(libcon.Check)
}

func (m *MockSSHClient) Stop() {
	m.Called()
}

func (m *MockSSHClient) Loop(fn libcon.SSHLoopHandler) error {
	args := m.Called(fn)
	return args.Error(0)
}

func (m *MockSSHClient) Session() *session.Session {
	args := m.Called()
	return args.Get(0).(*session.Session)
}

func (m *MockSSHClient) PrivateKeys() []session.AgentPrivateKey {
	args := m.Called()
	return args.Get(0).([]session.AgentPrivateKey)
}

func (m *MockSSHClient) RefreshPrivateKeys() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHClient) IsStopped() bool {
	return false
}

type MockCheck struct {
	mock.Mock
}

func (m *MockCheck) WithDelaySeconds(seconds int) libcon.Check {
	args := m.Called(seconds)
	return args.Get(0).(libcon.Check)
}

func (m *MockCheck) AwaitAvailability(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockCheck) CheckAvailability(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockCheck) ExpectAvailable(ctx context.Context) ([]byte, error) {
	args := m.Called(ctx)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockCheck) String() string {
	args := m.Called()
	return args.String(0)
}

type MockSession struct {
	mock.Mock
}

func (m *MockSession) AvailableHosts() []string {
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

func (m *mockReverseTunnel) StartHealthMonitor(ctx context.Context, checker libcon.ReverseTunnelChecker, killer libcon.ReverseTunnelKiller) {
	m.Called(ctx, checker, killer)
}

func (m *mockReverseTunnel) Stop() {
	m.Called()
}

func (m *mockReverseTunnel) String() string {
	args := m.Called()
	return args.String(0)
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
