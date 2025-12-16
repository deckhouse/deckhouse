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
	"reflect"
	"sync"
	"time"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/clissh/frontend"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
)

type (
	UploadScriptProvider func(scriptPath string, args ...string) *Script
	CommandProvider      func(scriptPath string, args ...string) *Command
	FileProvider         func() *File
	SwitchHandler        func(s Switch)

	Switch struct {
		Session     *session.Session
		PrivateKeys []session.AgentPrivateKey
	}
)

type SSHProvider struct {
	once            bool
	initSession     *session.Session
	initPrivateKeys []session.AgentPrivateKey
	client          *Client

	scriptProviders  *providersMap[UploadScriptProvider]
	commandProviders *providersMap[CommandProvider]
	fileProviders    *providersMap[FileProvider]

	switchHandler SwitchHandler
	switches      []Switch
}

func NewSSHProvider(initSession *session.Session, once bool) *SSHProvider {
	return &SSHProvider{
		initSession:     initSession,
		initPrivateKeys: make([]session.AgentPrivateKey, 0),
		once:            once,

		scriptProviders:  newProvidersMap[UploadScriptProvider](),
		commandProviders: newProvidersMap[CommandProvider](),
		fileProviders:    newOneProviderMap[FileProvider](),

		switches: make([]Switch, 0),
	}
}

func (p *SSHProvider) AddScriptProvider(host string, f UploadScriptProvider) *SSHProvider {
	p.scriptProviders.add(host, f)
	return p
}

func (p *SSHProvider) AddCommandProvider(host string, f CommandProvider) *SSHProvider {
	p.commandProviders.add(host, f)
	return p
}

func (p *SSHProvider) SetFileProvider(host string, f FileProvider) *SSHProvider {
	p.fileProviders.add(host, f)
	return p
}

func (p *SSHProvider) WithInitPrivateKeys(k []session.AgentPrivateKey) *SSHProvider {
	p.initPrivateKeys = k
	return p
}

func (p *SSHProvider) WithSwitchHandler(f SwitchHandler) *SSHProvider {
	p.switchHandler = f
	return p
}

func (p *SSHProvider) Switches() []Switch {
	return p.switches
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
	privateKeysCpy := make([]session.AgentPrivateKey, len(privateKeys))
	copy(privateKeysCpy, privateKeys)

	sessCopy := sess.Copy()
	// copy reset current host
	sessCopy.ChoiceNewHost()

	s := Switch{
		Session:     sessCopy,
		PrivateKeys: privateKeysCpy,
	}

	if !govalue.IsNil(p.switchHandler) {
		p.switchHandler(s)
	}

	p.switches = append(p.switches, s)

	return p.newClient(sess, privateKeys), nil
}

func (p *SSHProvider) newClient(session *session.Session, k []session.AgentPrivateKey) *Client {
	c := NewClient(session, k)

	p.scriptProviders.copyTo(p.scriptProviders)
	p.commandProviders.copyTo(c.commandProviders)
	p.fileProviders.copyTo(c.fileProviders)

	return c
}

func NewClient(session *session.Session, privKeys []session.AgentPrivateKey) *Client {
	return &Client{
		Settings:    session,
		privateKeys: privKeys,

		scriptProviders:  newProvidersMap[UploadScriptProvider](),
		commandProviders: newProvidersMap[CommandProvider](),
		fileProviders:    newOneProviderMap[FileProvider](),
	}
}

type Client struct {
	Settings *session.Session

	commandProviders *providersMap[CommandProvider]
	scriptProviders  *providersMap[UploadScriptProvider]
	fileProviders    *providersMap[FileProvider]

	privateKeys []session.AgentPrivateKey

	mu sync.Mutex

	kubeProxies []*kubeProxy
	started     bool
	stopped     bool
}

func (c *Client) AddScriptProvider(host string, f UploadScriptProvider) *Client {
	c.scriptProviders.add(host, f)
	return c
}

func (c *Client) AddCommandProvider(host string, f CommandProvider) *Client {
	c.commandProviders.add(host, f)
	return c
}

func (c *Client) SetFileProvider(host string, f FileProvider) *Client {
	c.fileProviders.add(host, f)
	return c
}

func (c *Client) OnlyPreparePrivateKeys() error {
	// Double start is safe here because for initializing private keys we are using sync.Once
	return c.Start()
}

func (c *Client) Start() error {
	if c.Settings == nil {
		return fmt.Errorf("Possible bug in ssh client: session should be created before start")
	}

	if c.isStopped() {
		return fmt.Errorf("Possible bug in ssh client: client stopped")
	}

	c.setStarted()

	return nil
}

// Easy access to frontends

// Tunnel is used to open local (L) and remote (R) tunnels
func (c *Client) Tunnel(address string) node.Tunnel {
	return &tunnel{address: address}
}

// ReverseTunnel is used to open remote (R) tunnel
func (c *Client) ReverseTunnel(address string) node.ReverseTunnel {
	return &reverseTunnel{address: address}
}

func errorCommand(name, errStr string) node.Command {
	return NewCommand(nil).WithErr(fmt.Errorf("%s: '%s'", errStr, name))
}

// Command is used to run commands on remote server
func (c *Client) Command(name string, arg ...string) node.Command {
	host := c.Settings.Host()
	providers, err := c.commandProviders.get(host)
	if err != nil {
		return errorCommand(name, err.Error())
	}

	for _, provider := range providers {
		cmd := provider(name, arg...)
		if !govalue.IsNil(cmd) {
			return cmd
		}
	}

	return errorCommand(name, fmt.Sprintf("All commands providers (%d) returns nil command for host: %s", len(providers), host))
}

// KubeProxy is used to start kubectl proxy and create a tunnel from local port to proxy port
func (c *Client) KubeProxy() node.KubeProxy {
	p := &kubeProxy{}
	c.kubeProxies = append(c.kubeProxies, p)
	return p
}

func errorFile(errStr string) node.File {
	err := fmt.Errorf(errStr)

	upload := func(data []byte, dstPath string) error {
		return err
	}

	download := func(srcPath string) ([]byte, error) {
		return nil, err
	}

	return NewFile(upload, download)
}

// File is used to upload and download files and directories
func (c *Client) File() node.File {
	host := c.Settings.Host()

	provider, err := c.fileProviders.get(host)
	if err != nil {
		return errorFile(err.Error())
	}

	// get returns error if not found
	file := provider[0]()
	if govalue.IsNil(file) {
		return errorFile(fmt.Sprintf("File provider returns nil File for host: %s", host))
	}

	return file
}

func errorScript(path, errStr string) node.Script {
	return NewScript(nil).WithError(fmt.Errorf("%s: %s", errStr, path))
}

// UploadScript is used to upload script and execute it on remote server
func (c *Client) UploadScript(scriptPath string, args ...string) node.Script {
	host := c.Settings.Host()

	providers, err := c.scriptProviders.get(host)
	if err != nil {
		return errorScript(scriptPath, err.Error())
	}

	for _, provider := range providers {
		s := provider(scriptPath, args...)
		if !govalue.IsNil(s) {
			return s
		}
	}

	return errorScript(scriptPath, fmt.Sprintf("All script providers (%d) returns nil command for host: %s", len(providers), host))
}

// UploadScript is used to upload script and execute it on remote server
func (c *Client) Check() node.Check {
	return ssh.NewCheck(func(sess *session.Session, cmd string) node.Command {
		return frontend.NewCommand(sess, cmd)
	}, c.Settings)
}

// Stop the client
func (c *Client) Stop() {
	if c.isStopped() {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, p := range c.kubeProxies {
		p.StopAll()
	}

	c.kubeProxies = nil
	c.stopped = true
}

func (c *Client) Session() *session.Session {
	return c.Settings
}

func (c *Client) PrivateKeys() []session.AgentPrivateKey {
	return c.privateKeys
}

func (c *Client) RefreshPrivateKeys() error {
	return nil
}

// Loop Looping all available hosts
func (c *Client) Loop(fn node.SSHLoopHandler) error {
	var err error

	resetSession := func() {
		c.Settings = c.Settings.Copy()
		c.Settings.ChoiceNewHost()
	}
	defer resetSession()
	resetSession()

	for range c.Settings.AvailableHosts() {
		err = fn(c)
		if err != nil {
			return err
		}
		c.Settings.ChoiceNewHost()
	}

	return nil
}

func (c *Client) setStarted() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.started = true
}

func (c *Client) isStarted() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.started
}

func (c *Client) isStopped() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.stopped
}

func (c *Client) appendProxy(p *kubeProxy) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.kubeProxies = append(c.kubeProxies, p)
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
	if govalue.IsNil(f.uploadFn) {
		return fmt.Errorf("uploadFn is nil for path '%s'", dstPath)
	}

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
	if govalue.IsNil(f.uploadFn) {
		return fmt.Errorf("uploadFn is nil for path '%s'", remotePath)
	}
	return f.uploadFn(data, remotePath)
}

func (f *File) DownloadBytes(ctx context.Context, remotePath string) ([]byte, error) {
	if govalue.IsNil(f.downloadFn) {
		return nil, fmt.Errorf("downloadFn is nil for path '%s'", remotePath)
	}
	return f.downloadFn(remotePath)
}

type providersMap[T any] struct {
	hostToProviders map[string][]T
	hasOne          bool
}

func newProvidersMap[T any]() *providersMap[T] {
	return &providersMap[T]{
		hostToProviders: make(map[string][]T),
		hasOne:          false,
	}
}

func newOneProviderMap[T any]() *providersMap[T] {
	return &providersMap[T]{
		hostToProviders: make(map[string][]T),
	}
}

func (m *providersMap[T]) add(host string, provider T) {
	mp := m.hostToProviders
	if len(mp) == 0 {
		mp = make(map[string][]T)
	}

	providers, ok := mp[host]
	if !ok || len(providers) == 0 {
		providers = make([]T, 0, 1)
	}

	if m.hasOne {
		providers = []T{provider}
	} else {
		providers = append(providers, provider)
	}

	mp[host] = providers

	m.hostToProviders = mp
}

func (m *providersMap[T]) copyTo(dst *providersMap[T]) {
	for host, providers := range m.hostToProviders {
		for _, provider := range providers {
			dst.add(host, provider)
		}
	}
}

func (m *providersMap[T]) createErr(host, err string) error {
	tp := reflect.TypeFor[T]()
	return fmt.Errorf("Providers for %s %s for host '%s'", tp.String(), err, host)
}

func (m *providersMap[T]) get(host string) ([]T, error) {
	if len(m.hostToProviders) == 0 {
		return nil, m.createErr(host, "not initialized")
	}

	providers, ok := m.hostToProviders[host]
	if !ok || len(providers) == 0 {
		return nil, m.createErr(host, "no providers found")
	}

	return providers, nil
}
