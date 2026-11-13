/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package agent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/adapters/configmap"
	"fencing-agent/internal/adapters/events"
	"fencing-agent/internal/adapters/fencingstate"
	"fencing-agent/internal/adapters/kubeclient"
	"fencing-agent/internal/adapters/metrics"
	"fencing-agent/internal/adapters/nodestatus"
	"fencing-agent/internal/adapters/proxyclient"
	wdadapter "fencing-agent/internal/adapters/watchdog"
	"fencing-agent/internal/config"
	"fencing-agent/internal/controllers/health"
	"fencing-agent/internal/controllers/localapi"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/usecase/cpview"
	"fencing-agent/internal/usecase/decision"
	"fencing-agent/internal/usecase/profile"
	"fencing-agent/internal/usecase/watchdog"
	"fencing-agent/internal/usecase/writer"
)

const (
	// leavingCopies is the number of «leaving» messages to each peer.
	leavingCopies = 3
	flushTimeout  = 2 * time.Second
	// stoppingTimeout limits the write of AgentStopping at the planned stop.
	// The stop must end before kubelet kills the agent.
	stoppingTimeout = time.Second
	// resumeReadTimeout is the timeout of one read of the local proxy before
	// the first decision, as for the reads of the control plane.
	resumeReadTimeout = 80 * time.Millisecond
	// resumeProbeTimeout is the timeout of one probe of /readyz before the
	// first decision, as for the probes of the apiservers.
	resumeProbeTimeout = proxyclient.ProbeTimeout
)

type Deps struct {
	K8sClient  kubernetes.Interface
	RestConfig *rest.Config
}

// ConfigSource gives the agent configuration.
type ConfigSource interface {
	// Run blocks until ctx ends.
	Run(ctx context.Context) error
	// Updates delivers each new valid Settings.
	Updates() <-chan config.Settings
}

// Network is the main network.
type Network interface {
	decision.Transport
	// Run runs the background loops until ctx ends.
	Run(ctx context.Context) error
	// Start waits for a valid key and the own address and creates memberlist.
	// Join runs in the background.
	Start(ctx context.Context, s config.Settings) error
	// Apply applies new Settings.
	Apply(s config.Settings) error
	// Members returns the number of alive members without the own node.
	Members() int
	// UpdateMeta changes the own NodeMeta and gossips it.
	UpdateMeta(update func(*domain.NodeMeta))
	// PublishMeta changes the own NodeMeta and waits up to 1 s for its broadcast.
	PublishMeta(update func(*domain.NodeMeta)) error
	// Remove removes a peer that sent «leaving» from the static peer list.
	Remove(node string)
	// Leave leaves memberlist with a timeout of 3 s and stops it.
	Leave()
}

// ControlPlane reads the control plane state.
type ControlPlane interface {
	decision.ControlPlane
	Run(ctx context.Context) error
	SetTimings(w, th time.Duration)
}

// NodeStatus writes the state of the agent to the own Node.
type NodeStatus interface {
	domain.Status
	// Run writes the changes until ctx ends.
	Run(ctx context.Context)
	// Flush writes the last state now.
	Flush(ctx context.Context) error
	// WriteCondition writes the Fencing condition now, with one request.
	WriteCondition(ctx context.Context, reason domain.Reason, message string) error
	RemoveLegacyLabels(ctx context.Context) error
}

// Engine is the decision engine.
type Engine interface {
	Run(ctx context.Context) error
	ShouldFeed() bool
	// Decided is closed after the first decision.
	Decided() <-chan struct{}
	SetParams(p decision.Params)
}

type Agent struct {
	env    config.Env
	logger *log.Logger
	self   *kubeclient.SelfWatcher

	config   ConfigSource
	network  Network
	newCP    func(s config.Settings) ControlPlane
	status   NodeStatus
	events   watchdog.EventRecorder
	metrics  domain.Metrics
	open     func() (watchdog.Device, error)
	nowayout func() (bool, error)
	active   func() (bool, error)
	// upstreams reads /upstreams of the local kubernetes-api-proxy.
	upstreams func(ctx context.Context) (proxyclient.Upstreams, error)
	// readyz probes /readyz through the API client of the agent. readyzCluster
	// probes /readyz on the ClusterIP of the service kubernetes. It is nil when
	// the API client of the agent goes to that ClusterIP.
	readyz        func(ctx context.Context) error
	readyzCluster func(ctx context.Context) error
	decide        func(decision.Deps) Engine
	// procDir is the directory of sys/kernel/panic and cmdline.
	procDir string
	// metricsHandler serves /metrics on the address of the health server.
	metricsHandler http.Handler
	// created gets the node of each «created» message. It can be nil.
	created func(node string)
	// attest gets the node of each accepted DeclareSuicide. It can be nil.
	attest func(node string)
	// ownCP applies the own FencingFailedNodeState to the control plane state
	// of the decision. ownReady tells whether the agent may feed and disarm
	// the watchdog of the previous process by it. They can be nil.
	ownCP    func(domain.CPState) domain.CPState
	ownReady func() bool
	// runners run beside the agent until the end. observers get each CPState.
	// stoppers run at the start of the planned stop, closers run at its end.
	runners   []func(ctx context.Context) error
	observers []func(domain.CPState)
	stoppers  []func()
	closers   []func()
	// failed gets the error of a part that asks to stop the agent. Run
	// returns it.
	failed chan error

	// The fields below are set by Run.
	engine Engine
	wd     *watchdog.Manager

	mu       sync.Mutex
	settings config.Settings
	cp       ControlPlane
}

func New(env config.Env, deps Deps, logger *log.Logger) (*Agent, error) {
	if deps.K8sClient == nil || deps.RestConfig == nil {
		return nil, errors.New("agent dependencies are not wired: K8sClient and RestConfig are required")
	}

	self, err := kubeclient.NewSelfWatcher(deps.K8sClient, env.NodeName, logger)
	if err != nil {
		return nil, fmt.Errorf("create own node watcher: %w", err)
	}

	reg := metrics.New(env.NodeName)
	rec := events.New(deps.K8sClient, env.NodeName, logger)
	status := nodestatus.New(deps.K8sClient, env.NodeName, rec, reg, logger)
	api := localapi.New(env.APISocketPath, env.NodeName, logger)

	ctrl, err := fencingstate.NewClient(deps.RestConfig)
	if err != nil {
		return nil, fmt.Errorf("create fencing client: %w", err)
	}

	source, err := configmap.NewSource(deps.K8sClient, self, fencingstate.NewProfiles(ctrl), reg, logger)
	if err != nil {
		return nil, fmt.Errorf("create configuration source: %w", err)
	}

	// The writer needs the network and the network gives its events to the
	// writer. The events start with Run, after New sets wr.
	var wr *writer.Writer

	network, err := newMainNetwork(env, deps, self, status, reg,
		func(name string, meta *domain.NodeMeta) { wr.Member(name, meta) }, logger)
	if err != nil {
		return nil, fmt.Errorf("create main network: %w", err)
	}

	proxy := proxyclient.New(deps.RestConfig, deps.K8sClient)

	// The DaemonSet sets KUBERNETES_SERVICE_HOST to the local proxy. kubelet
	// also sets KUBERNETES_PORT to the ClusterIP of the service kubernetes.
	readyzCluster, err := clusterReadyz(deps.RestConfig, os.Getenv("KUBERNETES_PORT"))
	if err != nil {
		return nil, err
	}

	ownIPs := func() []string {
		node, _ := self.Snapshot()

		return append(node.InternalIPs, node.ExternalIPs...)
	}

	a := &Agent{
		env:     env,
		logger:  logger,
		self:    self,
		config:  source,
		network: network,
		newCP: func(s config.Settings) ControlPlane {
			return cpview.New(proxy, status, ownIPs, s.Profile.ControlPlaneLostAfter.Duration, s.Config.CommonTimings.FastReadAfter.Duration, logger)
		},
		status:         status,
		events:         rec,
		metrics:        reg,
		metricsHandler: reg.Handler(),
		open: func() (watchdog.Device, error) {
			return wdadapter.Open(env.WatchdogDevice, logger)
		},
		nowayout:      func() (bool, error) { return wdadapter.Nowayout(env.WatchdogDevice) },
		active:        func() (bool, error) { return wdadapter.Active(env.WatchdogDevice) },
		upstreams:     proxy.Upstreams,
		readyz:        func(ctx context.Context) error { return apiReady(ctx, deps.K8sClient) },
		readyzCluster: readyzCluster,
		decide:        func(d decision.Deps) Engine { return decision.New(d, logger) },
		procDir:       "/proc",
		runners: []func(ctx context.Context) error{
			func(ctx context.Context) error { proxy.Run(ctx); return nil },
			api.Run,
		},
		observers: []func(domain.CPState){api.Observe},
		closers:   []func(){rec.Shutdown},
		failed:    make(chan error, 1),
	}

	wr = writer.New(writer.Deps{
		Self:     env.NodeName,
		API:      fencingstate.NewIncidents(ctrl),
		Network:  network,
		Settings: a.currentSettings,
		Maintenance: func() bool {
			node, ok := self.Snapshot()

			return !ok || node.Maintenance
		},
		Profile: func() v1alpha1.FencingSLAProfileSpec { return decisionProfile(a.currentSettings(), a.wd.Armed()) },
		Final:   func() bool { return a.wd.Final() },
	}, logger)

	// Run sets a.wd before the runners start.
	own, err := fencingstate.NewOwn(ctrl, env.NodeName, func(ffns *v1alpha1.FencingFailedNodeState) {
		wr.SetOwn(ffns)
		phase := ""
		if writer.Proven(ffns) {
			phase = string(ffns.Status.Phase)
		}

		a.wd.SetProven(phase)
	})
	if err != nil {
		return nil, fmt.Errorf("create own fencingfailednodestate watcher: %w", err)
	}

	a.runners = append(a.runners, wr.Run, own.Run)
	a.stoppers = append(a.stoppers, wr.Stop)
	a.observers = append(a.observers, wr.ObserveCP)
	a.created = wr.Created
	a.attest = wr.Attest
	a.ownCP = wr.OwnCP
	a.ownReady = wr.Ready

	return a, nil
}

// Run works until ctx ends. The end of ctx is SIGTERM: the agent does the
// planned stop. An error of a part stops the agent without Magic Close.
func (a *Agent) Run(ctx context.Context) error {
	// The parts stop after the planned stop, not at SIGTERM.
	wctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()

	g, gctx := errgroup.WithContext(wctx)

	view := newCPView(a.observers...)
	view.gate = a.ownCP

	a.engine = a.decide(decision.Deps{
		Transport:    a.network,
		ControlPlane: view,
		StopFeed:     func() bool { return a.wd.StopFeed() },
		ResumeFeed:   func() bool { return a.wd.ResumeFeed() },
		Finalize:     func() { a.wd.Finalize() },
		Metrics:      a.metrics,
		Leaving:      a.network.Remove,
		Created:      a.created,
		Attest:       a.attest,
	})

	a.wd = watchdog.New(watchdog.Deps{
		Open:       a.open,
		Nowayout:   a.nowayout,
		Active:     a.active,
		DevicePath: a.env.WatchdogDevice,
		Node:       a.self,
		HasPeer:    func() bool { return len(a.network.Peers()) > 0 },
		ShouldFeed: a.engine.ShouldFeed,
		Decided:    a.engine.Decided(),
		Status:     a.status,
		Events:     a.events,
		UpdateMeta: a.network.UpdateMeta,
		OnArmed:    func(bool) { a.tune() },
		Fail:       a.fail,

		KernelPanicPath: filepath.Join(a.procDir, "sys/kernel/panic"),
		CmdlinePath:     filepath.Join(a.procDir, "cmdline"),
	}, a.logger)

	isReady := func() bool {
		return ready(a.wd.Reason(), view.working(), a.network.Peers(), a.network.Members())
	}

	a.logger.Info("fencing-agent starting", "node", a.env.NodeName)

	// The device of the previous process can be armed. Disarm it when the
	// control plane is visible, without the configuration. With the own
	// FencingFailedNodeState, disarm it only after a heartbeat in it.
	g.Go(func() error {
		a.wd.Resume(gctx, func(ctx context.Context) bool {
			return a.resumeVisible(ctx) && (a.ownReady == nil || a.ownReady())
		})

		return nil
	})

	g.Go(func() error {
		return health.NewServer(a.env.HealthProbeBindAddress, a.logger, isReady, func() bool { return true }, a.metricsHandler).Run(gctx)
	})
	g.Go(func() error { a.self.Run(gctx); return nil })
	g.Go(func() error { a.status.Run(gctx); return nil })
	g.Go(func() error { return a.config.Run(gctx) })
	g.Go(func() error { return a.network.Run(gctx) })
	g.Go(func() error { return a.engine.Run(gctx) })

	for _, run := range a.runners {
		g.Go(func() error { return run(gctx) })
	}

	g.Go(func() error { return a.work(gctx, g, view) })
	g.Go(func() error {
		select {
		case err := <-a.failed:
			return err
		case <-gctx.Done():
			return nil
		}
	})

	select {
	case <-ctx.Done():
		a.stop()
	case <-gctx.Done():
		a.logger.Error("a part of the agent failed, the watchdog stays armed")
	}

	cancel()

	if err := g.Wait(); err != nil {
		return err
	}

	a.logger.Info("fencing-agent stopped")

	return nil
}

// fail stops Run with err without the planned stop. It does not block.
func (a *Agent) fail(err error) {
	select {
	case a.failed <- err:
	default:
	}
}

// work does the start steps in a fixed order and then applies each new
// Settings, one at a time.
func (a *Agent) work(ctx context.Context, g *errgroup.Group, view *cpView) error {
	if err := a.status.RemoveLegacyLabels(ctx); err != nil {
		// The status writer tries again by itself.
		a.logger.Warn("remove the fencing 1.0 labels", "error", err)
	}

	var s config.Settings

	select {
	case <-ctx.Done():
		return nil
	case s = <-a.config.Updates():
	}

	if !a.self.WaitForSync(ctx) {
		return nil
	}

	a.publishProfile(s)

	// The memberlist instance joins the peers in the background.
	if err := a.network.Start(ctx, s); err != nil {
		if ctx.Err() != nil {
			return nil
		}

		return fmt.Errorf("start the main network: %w", err)
	}

	cp := a.newCP(s)

	a.mu.Lock()
	a.settings, a.cp = s, cp
	a.mu.Unlock()

	a.tune()
	g.Go(func() error { return cp.Run(ctx) })
	g.Go(func() error { view.run(ctx, cp); return nil })

	// Start checks kernel.panic and arms the watchdog when the conditions are true.
	a.wd.Apply(a.params(s))
	a.wd.Start()

	// Run publishes the condition and NodeMeta first.
	a.status.SetProfile(publishedProfile(s))
	g.Go(func() error { a.wd.Run(ctx); return nil })

	for {
		select {
		case <-ctx.Done():
			return nil
		case s = <-a.config.Updates():
		}

		a.logger.Info("configuration received", "node_group", s.NodeGroup, "mode", s.Mode, "profile", s.ProfileName)
		a.publishProfile(s)

		if err := a.network.Apply(s); err != nil {
			a.logger.Error("apply the main network configuration", "error", err)
		}

		a.mu.Lock()
		a.settings = s
		a.mu.Unlock()

		a.tune()
		a.wd.Apply(a.params(s))
		a.status.SetProfile(publishedProfile(s))
	}
}

// resumeVisible reads the local proxy once. The control plane is visible when
// an upstream was ready no more than maxProbeAge ago. Without a lost probe,
// lastReadyAgo is not more than maxProbeAge. After a round that lost the
// probes to all upstreams, lastReadyAgo can be up to 2.3 s for up to 0.6 s.
// Then Resume reads again. When the proxy does not answer or is old, one
// probe of /readyz through the API client of the agent tells it. When that
// probe does not connect, one probe of /readyz on the ClusterIP of the
// service kubernetes tells it. The profile is not known yet, so the control
// plane visibility rule does not apply.
func (a *Agent) resumeVisible(ctx context.Context) bool {
	rctx, cancel := context.WithTimeout(ctx, resumeReadTimeout)
	u, err := a.upstreams(rctx)

	cancel()

	if err == nil && u.HasProbe {
		return len(u.LastReadyAgo) > 0 && slices.Min(u.LastReadyAgo) <= profile.MaxProbeAge
	}

	err = probe(ctx, a.readyz)
	if notConnected(err) && a.readyzCluster != nil {
		err = probe(ctx, a.readyzCluster)
	}

	return err == nil
}

// probe runs one probe with the timeout resumeProbeTimeout.
func probe(ctx context.Context, readyz func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, resumeProbeTimeout)
	defer cancel()

	return readyz(ctx)
}

// apiReady returns nil when /readyz of the API answers 200. The client goes to
// the API address of the agent: the local kubernetes-api-proxy, or the
// ClusterIP on a Deckhouse Engine node.
func apiReady(ctx context.Context, client kubernetes.Interface) error {
	return client.Discovery().RESTClient().Get().AbsPath("/readyz").Do(ctx).Error()
}

// notConnected tells whether the probe failed to connect. An answer of the
// server, also an error, is not this case.
func notConnected(err error) bool {
	var op *net.OpError

	return errors.As(err, &op) && op.Op == "dial"
}

// clusterReadyz returns the probe of /readyz on the ClusterIP of
// kubernetesPort with the TLS settings and the credentials of cfg. It returns
// nil when cfg already goes to this ClusterIP or kubernetesPort is not valid.
func clusterReadyz(cfg *rest.Config, kubernetesPort string) (func(ctx context.Context) error, error) {
	host := clusterHost(kubernetesPort, cfg.Host)
	if host == "" {
		return nil, nil
	}

	cfg = rest.CopyConfig(cfg)
	cfg.Host = host

	client, err := kubeclient.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("create the client of the ClusterIP %s: %w", host, err)
	}

	return func(ctx context.Context) error { return apiReady(ctx, client) }, nil
}

// clusterHost returns the URL of the ClusterIP of the service kubernetes from
// KUBERNETES_PORT, as tcp://10.222.0.1:443. It returns "" when the value is not
// valid or when restHost already goes to this ClusterIP.
func clusterHost(kubernetesPort, restHost string) string {
	u, err := url.Parse(kubernetesPort)
	if err != nil || u.Scheme != "tcp" || u.Port() == "" || u.Hostname() == "" {
		return ""
	}

	if r, err := url.Parse(restHost); err == nil && r.Host == u.Host {
		return ""
	}

	return "https://" + u.Host
}

// currentSettings returns the applied Settings. They are empty before the start.
func (a *Agent) currentSettings() config.Settings {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.settings
}

// tune gives the decision parameters to the engine and to the control plane
// view: the profile of the group for an armed agent, else the memberlist profile.
func (a *Agent) tune() {
	a.mu.Lock()
	s, cp := a.settings, a.cp
	a.mu.Unlock()

	if cp == nil {
		return
	}

	s.Profile = decisionProfile(s, a.wd.Armed())
	a.engine.SetParams(decision.ParamsFrom(s))
	cp.SetTimings(s.Profile.ControlPlaneLostAfter.Duration, s.Config.CommonTimings.FastReadAfter.Duration)
}

// decisionProfile returns the profile of the decision. An agent without an
// armed watchdog uses the memberlist profile.
func decisionProfile(s config.Settings, armed bool) v1alpha1.FencingSLAProfileSpec {
	if armed {
		return s.Profile
	}

	return s.Memberlist
}

// publishProfile puts the node group and the profile of s in the own NodeMeta.
// The writers of the peers put this profile in the FFNS of this node.
func (a *Agent) publishProfile(s config.Settings) {
	a.network.UpdateMeta(func(m *domain.NodeMeta) { m.NodeGroup, m.Profile = s.NodeGroup, s.ProfileName })
}

// publishedProfile returns the value of the profile annotation of the Node:
// "disabled" for a Disabled group, else the profile of the group. The state
// of the watchdog does not change it.
func publishedProfile(s config.Settings) string {
	if s.Mode == config.ModeDisabled {
		return "disabled"
	}

	return s.ProfileName
}

func (a *Agent) params(s config.Settings) watchdog.Params {
	return watchdog.Params{
		Disabled:     s.Mode == config.ModeDisabled,
		Timeout:      s.Profile.Watchdog.Timeout.Duration,
		FeedInterval: s.Profile.Watchdog.FeedInterval.Duration,
		Engine:       s.Engine,
	}
}

// stop is the planned stop on SIGTERM. When the watchdog is armed and its
// feed is not stopped, the agent writes AgentStopping to the own Node, then
// does Magic Close and sends «leaving». The controller does not taint a node
// with AgentStopping, so the agent disarms only after a successful write.
// Without it the watchdog stays armed and the agent does not send «leaving»:
// the peers see the node Dead and create its FFNS, as after a crash. An agent
// without the armed watchdog always sends «leaving»: the node does not stop.
// On a worker shutdown kubelet kills the agent 5s after SIGTERM.
func (a *Agent) stop() {
	a.logger.Info("fencing-agent stopping")

	armed, feedStopped := a.wd.Close()

	// The agent takes no events after SIGTERM.
	for _, stopFn := range a.stoppers {
		stopFn()
	}

	const stays = "the watchdog stays armed, the agent does not send leaving"

	switch {
	case !armed:
		a.writeStopping()
		a.leave()
	case feedStopped:
		a.logger.Warn(stays, "reason", "the feed was stopped before SIGTERM")
	case !a.writeStopping():
		a.logger.Warn(stays, "reason", "AgentStopping is not written")
	case !a.wd.Disarm():
		a.logger.Warn(stays, "reason", "the feed stopped during the write of AgentStopping")
	default:
		a.leave()
	}

	// Flush writes the last state if the status writer did not write it yet.
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()

	if err := a.status.Flush(ctx); err != nil {
		a.logger.Warn("write the last state of the agent", "error", err)
	}

	for _, closeFn := range a.closers {
		closeFn()
	}
}

// writeStopping writes AgentStopping to the own Node. It tells whether the
// write is successful.
func (a *Agent) writeStopping() bool {
	ctx, cancel := context.WithTimeout(context.Background(), stoppingTimeout)
	defer cancel()

	if err := a.status.WriteCondition(ctx, domain.ReasonAgentStopping, ""); err != nil {
		a.logger.Warn("write AgentStopping", "error", err)

		return false
	}

	return true
}

// leave sends «leaving» to the peers and leaves memberlist.
func (a *Agent) leave() {
	leaving := domain.Leaving{Header: domain.Header{Version: domain.ProtocolVersion, From: a.env.NodeName}}

	peers, sendErrors := 0, 0

	for range leavingCopies {
		list := a.network.Peers()
		peers = len(list)

		for _, peer := range list {
			if err := a.network.Send(peer, leaving); err != nil {
				sendErrors++

				a.logger.Debug("send leaving", "peer", peer.Name, "error", err)
			}
		}
	}

	// The peers get «leaving» in the NodeMeta before Leave tells them Dead.
	metaPublished := true

	if err := a.network.PublishMeta(func(m *domain.NodeMeta) { m.Leaving = true }); err != nil {
		metaPublished = false

		a.logger.Warn("publish leaving in the NodeMeta", "error", err)
	}

	a.network.Leave()

	a.logger.Info("leaving sent", "peers", peers, "copies", leavingCopies, "send_errors", sendErrors, "meta_published", metaPublished)
}
