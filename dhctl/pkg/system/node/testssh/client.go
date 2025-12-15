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

package testssh

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/clissh/frontend"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
)

type (
	UploadScriptProvider func(host string, scriptPath string, args ...string) *Script
	CommandProvider      func(host string, scriptPath string, args ...string) *Command
	FileProvider         func() *File
)

type SSHProvider struct {
	once            bool
	initSession     *session.Session
	initPrivateKeys []session.AgentPrivateKey
	client          *Client

	scriptProvider  UploadScriptProvider
	commandProvider CommandProvider
	fileProvider    FileProvider
}

func NewSSHProvider(initSession *session.Session, once bool) *SSHProvider {
	return &SSHProvider{
		initSession:     initSession,
		initPrivateKeys: make([]session.AgentPrivateKey, 0),
		once:            once,
	}
}

func (p *SSHProvider) CommandProvider() CommandProvider {
	return p.commandProvider
}

func (p *SSHProvider) WithScriptProvider(f UploadScriptProvider) *SSHProvider {
	p.scriptProvider = f
	return p
}

func (p *SSHProvider) WithCommandProvider(f CommandProvider) *SSHProvider {
	p.commandProvider = f
	return p
}

func (p *SSHProvider) WithFileProvider(f FileProvider) *SSHProvider {
	p.fileProvider = f
	return p
}

func (p *SSHProvider) WithInitPrivateKeys(k []session.AgentPrivateKey) *SSHProvider {
	p.initPrivateKeys = k
	return p
}

func (p *SSHProvider) Client() (node.SSHClient, error) {
	if p.initSession == nil {
		return nil, fmt.Errorf("Init session is nil")
	}

	if p.once {
		if p.client == nil {
			p.client = p.newClient(p.initSession, p.initPrivateKeys)
		}

		return p.client, nil
	}

	return p.newClient(p.initSession, p.initPrivateKeys), nil
}
func (p *SSHProvider) SwitchClient(_ context.Context, sess *session.Session, privateKeys []session.AgentPrivateKey) (node.SSHClient, error) {
	return p.newClient(sess, privateKeys), nil
}

func (p *SSHProvider) newClient(session *session.Session, k []session.AgentPrivateKey) *Client {
	return NewClient(session, k).
		WithScriptProvider(p.scriptProvider).
		WithCommandProvider(p.commandProvider).
		WithFileProvider(p.fileProvider)
}

func NewClient(session *session.Session, privKeys []session.AgentPrivateKey) *Client {
	return &Client{
		Settings:    session,
		privateKeys: privKeys,
	}
}

type Client struct {
	Settings *session.Session

	scriptProvider  UploadScriptProvider
	commandProvider CommandProvider
	fileProvider    FileProvider

	privateKeys []session.AgentPrivateKey

	mu sync.Mutex

	kubeProxies []*kubeProxy
	started     bool
	stopped     bool
}

func (p *Client) WithScriptProvider(f UploadScriptProvider) *Client {
	p.scriptProvider = f
	return p
}

func (p *Client) WithCommandProvider(f CommandProvider) *Client {
	p.commandProvider = f
	return p
}

func (p *Client) WithFileProvider(f FileProvider) *Client {
	p.fileProvider = f
	return p
}

func (s *Client) OnlyPreparePrivateKeys() error {
	// Double start is safe here because for initializing private keys we are using sync.Once
	return s.Start()
}

func (s *Client) Start() error {
	if s.Settings == nil {
		return fmt.Errorf("Possible bug in ssh client: session should be created before start")
	}

	if s.isStopped() {
		return fmt.Errorf("Possible bug in ssh client: client stopped")
	}

	s.setStarted()

	return nil
}

// Easy access to frontends

// Tunnel is used to open local (L) and remote (R) tunnels
func (s *Client) Tunnel(address string) node.Tunnel {
	return &tunnel{address: address}
}

// ReverseTunnel is used to open remote (R) tunnel
func (s *Client) ReverseTunnel(address string) node.ReverseTunnel {
	return &reverseTunnel{address: address}
}

// Command is used to run commands on remote server
func (s *Client) Command(name string, arg ...string) node.Command {
	if s.commandProvider == nil {
		return NewCommand(nil).WithErr(fmt.Errorf("Command provider not passed: '%s'", name))
	}

	host := s.Settings.Host()

	cmd := s.commandProvider(host, name, arg...)
	if govalue.IsNil(cmd) {
		return NewCommand(nil).WithErr(fmt.Errorf("Provider returns nil command for '%s' for host '%s'", name, host))
	}

	return cmd
}

// KubeProxy is used to start kubectl proxy and create a tunnel from local port to proxy port
func (s *Client) KubeProxy() node.KubeProxy {
	p := &kubeProxy{}
	s.kubeProxies = append(s.kubeProxies, p)
	return p
}

// File is used to upload and download files and directories
func (s *Client) File() node.File {
	if s.fileProvider == nil {
		return NewFile(func(data []byte, dstPath string) error {
			return fmt.Errorf("File provider did not provided")
		}, func(srcPath string) ([]byte, error) {
			return nil, fmt.Errorf("File provider did not provided")
		})
	}
	return s.fileProvider()
}

// UploadScript is used to upload script and execute it on remote server
func (s *Client) UploadScript(scriptPath string, args ...string) node.Script {
	if s.scriptProvider == nil {
		return NewScript(nil).WithError(fmt.Errorf("Upload script provider not passed: %v", scriptPath))
	}

	return s.scriptProvider(s.Settings.Host(), scriptPath, args...)
}

// UploadScript is used to upload script and execute it on remote server
func (s *Client) Check() node.Check {
	return ssh.NewCheck(func(sess *session.Session, cmd string) node.Command {
		return frontend.NewCommand(sess, cmd)
	}, s.Settings)
}

// Stop the client
func (s *Client) Stop() {
	if s.isStopped() {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, p := range s.kubeProxies {
		p.StopAll()
	}

	s.kubeProxies = nil
	s.stopped = true
}

func (s *Client) Session() *session.Session {
	return s.Settings
}

func (s *Client) PrivateKeys() []session.AgentPrivateKey {
	return s.privateKeys
}

func (s *Client) RefreshPrivateKeys() error {
	return nil
}

// Loop Looping all available hosts
func (s *Client) Loop(fn node.SSHLoopHandler) error {
	var err error

	resetSession := func() {
		s.Settings = s.Settings.Copy()
		s.Settings.ChoiceNewHost()
	}
	defer resetSession()
	resetSession()

	for range s.Settings.AvailableHosts() {
		err = fn(s)
		if err != nil {
			return err
		}
		s.Settings.ChoiceNewHost()
	}

	return nil
}

func (s *Client) setStarted() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.started = true
}

func (s *Client) isStarted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.started
}

func (s *Client) isStopped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.stopped
}

func (s *Client) appendProxy(p *kubeProxy) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.kubeProxies = append(s.kubeProxies, p)
}

type kubeProxy struct{}

func (k *kubeProxy) Start(useLocalPort int) (port string, err error) {
	i := rand.New(rand.NewSource(time.Now().UnixNano())).Int()
	return fmt.Sprintf("%d", i), nil
}

func (k *kubeProxy) StopAll() {}

func (k *kubeProxy) Stop(startID int) {}

type tunnel struct {
	address string
}

func (t *tunnel) Up() error {
	return nil
}

func (t *tunnel) HealthMonitor(errorOutCh chan<- error) {}

func (t *tunnel) Stop() {}

func (t *tunnel) String() string {
	return "tunnel: " + t.address
}

type reverseTunnel struct {
	address string
}

func (t *reverseTunnel) Up() error {
	return nil
}

func (t *reverseTunnel) StartHealthMonitor(ctx context.Context, checker node.ReverseTunnelChecker, killer node.ReverseTunnelKiller) {
}

func (t *reverseTunnel) Stop() {}

func (t *reverseTunnel) String() string {
	return "reverseTunnel: " + t.address
}

var newLine = []byte("\n")

type Script struct {
	stdOut []byte
	err    error

	handler func(string)
	run     func()
}

func NewScript(stdOut []byte) *Script {
	return &Script{
		stdOut: stdOut,
	}
}

func (t *Script) WithError(err error) *Script {
	t.err = err
	return t
}

func (t *Script) WithRun(f func()) *Script {
	t.run = f
	return t
}

func (t *Script) Execute(context.Context) (stdout []byte, err error) {
	return t.execute()
}

func (t *Script) ExecuteBundle(ctx context.Context, parentDir, bundleDir string) (stdout []byte, err error) {
	return t.execute()
}

func (t *Script) Sudo() {}
func (t *Script) WithStdoutHandler(handler func(string)) {
	t.handler = handler
}
func (t *Script) WithTimeout(timeout time.Duration)   {}
func (t *Script) WithEnvs(envs map[string]string)     {}
func (t *Script) WithCleanupAfterExec(doCleanup bool) {}
func (t *Script) WithCommanderMode(enabled bool)      {}
func (t *Script) WithExecuteUploadDir(dir string)     {}
func (t *Script) execute() (stdout []byte, err error) {
	if t.handler != nil {
		t.handler(string(t.stdOut))
	}
	if t.run != nil {
		t.run()
	}

	return t.stdOut, t.err
}

type Command struct {
	stdOut, stdErr []byte
	err            error

	onStart func()
	run     func()

	stdOutFunc func(line string)
	stdErrFunc func(line string)
}

func NewCommand(stdOut []byte) *Command {
	return &Command{
		stdOut: stdOut,
	}
}

func (t *Command) WithStdErr(s []byte) *Command {
	t.stdErr = s
	return t
}

func (t *Command) WithErr(err error) *Command {
	t.err = err
	return t
}

func (t *Command) WithRun(f func()) *Command {
	t.run = f
	return t
}

func (t *Command) Run(ctx context.Context) error {
	return t.doRun()
}

func (t *Command) Cmd(ctx context.Context)  {}
func (t *Command) Sudo(ctx context.Context) {}

func (t *Command) StdoutBytes() []byte {
	return t.stdOut
}

func (t *Command) StderrBytes() []byte {
	return t.stdErr
}

func (t *Command) Output(context.Context) ([]byte, []byte, error) {
	return t.stdOut, t.stdErr, t.err
}

func (t *Command) CombinedOutput(context.Context) ([]byte, error) {
	return bytes.Join([][]byte{t.stdOut, t.stdErr}, newLine), t.err
}

func (t *Command) OnCommandStart(fn func()) {
	t.onStart = fn
}

func (t *Command) WithEnv(env map[string]string)     {}
func (t *Command) WithTimeout(timeout time.Duration) {}
func (t *Command) WithStdoutHandler(h func(line string)) {
	t.stdOutFunc = h
}
func (t *Command) WithStderrHandler(h func(line string)) {
	t.stdErrFunc = h
}
func (t *Command) WithSSHArgs(args ...string) {}

func (t *Command) doRun() error {
	if t.onStart != nil {
		t.onStart()
	}

	if t.stdOutFunc != nil {
		for _, line := range bytes.Split(t.stdOut, newLine) {
			t.stdOutFunc(string(line))
		}
	}
	if t.stdErrFunc != nil {
		for _, line := range bytes.Split(t.stdErr, newLine) {
			t.stdErrFunc(string(line))
		}
	}

	if t.run != nil {
		t.run()
	}

	return t.err
}

type (
	UploadFn   func(data []byte, dstPath string) error
	DownloadFn func(srcPath string) ([]byte, error)
)

type File struct {
	uploadFn   UploadFn
	downloadFn DownloadFn
}

func NewFile(upload UploadFn, download DownloadFn) *File {
	return &File{
		uploadFn:   upload,
		downloadFn: download,
	}
}

func (f *File) Upload(ctx context.Context, srcPath, dstPath string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	return f.uploadFn(data, dstPath)
}
func (f *File) Download(ctx context.Context, srcPath, dstPath string) error {
	data, err := f.DownloadBytes(ctx, srcPath)
	if err != nil {
		return err
	}

	return os.WriteFile(dstPath, data, os.ModePerm)
}

func (f *File) UploadBytes(ctx context.Context, data []byte, remotePath string) error {
	return f.uploadFn(data, remotePath)
}

func (f *File) DownloadBytes(ctx context.Context, remotePath string) ([]byte, error) {
	return f.downloadFn(remotePath)
}
