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

// Package revocationfeed serves the revocations of Dex to dex-authenticator as a stream of
// server-sent events, so that a session the controller or Dex revoked stops working within seconds
// and not at its next refresh.
//
// The feed runs on every replica of the controller, from the shared cache of Revocation objects. A
// DexAuthenticator signs in with the id and the secret of its Dex client and gets the revocations
// that name its client: a snapshot of the stored ones, then each new one as it comes. The feed
// carries no subjects, session or grant IDs, only keys salted per client (Key).
package revocationfeed

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
)

const (
	// Path is where the feed is served.
	Path = "/revocations"

	realm = `Basic realm="user-authn-revocations"`

	readHeaderTimeout = 5 * time.Second
	idleTimeout       = 120 * time.Second
	maxHeaderBytes    = 16 << 10
	shutdownTimeout   = 5 * time.Second

	// The Retry-After of the refusals, in seconds.
	retryAfterFailures    = "10"
	retryAfterStreams     = "30"
	retryAfterConnections = "5"

	// retryMillis is the reconnection delay the stream asks the client for.
	retryMillis = 5000
)

// Limits bound what the feed of one replica serves.
type Limits struct {
	// FailuresPerSecond and FailureBurst bound failed sign-ins from one address.
	FailuresPerSecond float64
	FailureBurst      int
	// ConnectionsPerSecond and ConnectionBurst bound new streams of one client.
	ConnectionsPerSecond float64
	ConnectionBurst      int
	// StreamsPerClient and Streams bound the open streams of one client and of the replica.
	StreamsPerClient int
	Streams          int
	// Queue is how many events a stream may fall behind before it is closed, and InstanceQueue the
	// same for the stream of a shared dex-authenticator instance, which carries the events of all
	// its clients.
	Queue         int
	InstanceQueue int
	// Ring is how many of the last events a reconnecting client can resume from.
	Ring int
	// SnapshotPart is how many entries one snapshot event holds.
	SnapshotPart int
	// Heartbeat is how often an idle stream writes, and WriteTimeout how long one write may take.
	Heartbeat    time.Duration
	WriteTimeout time.Duration
}

// DefaultLimits are the limits of the contract of the feed.
func DefaultLimits() Limits {
	return Limits{
		FailuresPerSecond:    1,
		FailureBurst:         10,
		ConnectionsPerSecond: 2,
		ConnectionBurst:      10,
		StreamsPerClient:     10,
		// A queue is allocated with its stream: 64 records are 8 KiB, so 1000 streams keep the queues of a
		// replica under 8 MiB of the 256 MiB of the controller. A stream that falls behind is closed and
		// resumes from the ring.
		Streams:       1000,
		Queue:         64,
		InstanceQueue: 1024,
		Ring:          4096,
		SnapshotPart:  512,
		Heartbeat:     10 * time.Second,
		WriteTimeout:  10 * time.Second,
	}
}

// Config configures a feed.
type Config struct {
	// Addr is where the feed listens, TLSDir where tls.crt and tls.key are.
	Addr   string
	TLSDir string
	// Retention is how long revocations are kept (revocation.Retention).
	Retention time.Duration
	Limits    Limits
	// TokenReview checks the ServiceAccount token of a shared dex-authenticator instance. Without
	// it the feed serves the clients only.
	TokenReview TokenReviewer
}

// Server is the revocation feed. It is a manager runnable that runs on every replica.
type Server struct {
	reader    client.Reader
	hub       *hub
	cfg       Config
	metrics   *metrics
	clients   *clientSet
	failures  *addressLimiter
	log       logr.Logger
	now       func() time.Time
	connMu    sync.Mutex
	connRates map[string]*rate.Limiter
}

var _ manager.LeaderElectionRunnable = (*Server)(nil)

// New returns a feed that reads Revocation and OAuth2Client objects through reader, which must be the
// cache with the indexes of controller.Indexes and the client cache of ClientCache. now defaults to
// time.Now when nil.
func New(reader client.Reader, cfg Config, log logr.Logger, now func() time.Time) (*Server, error) {
	if now == nil {
		now = time.Now
	}
	h, err := newHub(cfg.Limits)
	if err != nil {
		return nil, err
	}
	clients := newClientSet(h.announce)
	h.instanceOf = clients.instanceOf
	return &Server{
		reader:    reader,
		hub:       h,
		cfg:       cfg,
		metrics:   newMetrics(),
		clients:   clients,
		failures:  newAddressLimiter(cfg.Limits.FailuresPerSecond, cfg.Limits.FailureBurst),
		log:       log,
		now:       now,
		connRates: map[string]*rate.Limiter{},
	}, nil
}

// Register adds a feed to the manager: its informer handlers, its metrics, and the server. The
// informers are registered now, before the manager starts, so that the readiness check waits for
// them too.
func Register(ctx context.Context, mgr manager.Manager, cfg Config) error {
	s, err := New(mgr.GetClient(), cfg, mgr.GetLogger().WithName("revocation-feed"), nil)
	if err != nil {
		return err
	}
	revocations, err := mgr.GetCache().GetInformer(ctx, controller.Object(controller.RevocationGVK))
	if err != nil {
		return fmt.Errorf("get the revocation informer: %w", err)
	}
	if _, err := revocations.AddEventHandler(toolscache.ResourceEventHandlerDetailedFuncs{AddFunc: s.onRevocation}); err != nil {
		return fmt.Errorf("watch revocations: %w", err)
	}
	clients, err := mgr.GetCache().GetInformer(ctx, controller.Object(controller.OAuth2ClientGVK))
	if err != nil {
		return fmt.Errorf("get the oauth2 client informer: %w", err)
	}
	if _, err := clients.AddEventHandler(s.clients.handler()); err != nil {
		return fmt.Errorf("watch oauth2 clients: %w", err)
	}
	if err := register(ctrlmetrics.Registry, s); err != nil {
		return err
	}
	if err := mgr.Add(s); err != nil {
		return fmt.Errorf("add the revocation feed: %w", err)
	}
	return nil
}

// NeedLeaderElection makes the feed run on every replica: each serves its own subscribers from its
// own cache.
func (s *Server) NeedLeaderElection() bool {
	return false
}

// Start serves the feed with TLS until ctx ends; then it closes every stream, and the clients go to
// another replica. The listener drops the connection of a client that is gone within half a minute
// (listen), so its stream closes and leaves the subscribers gauge.
func (s *Server) Start(ctx context.Context) error {
	cert, err := tls.LoadX509KeyPair(filepath.Join(s.cfg.TLSDir, "tls.crt"), filepath.Join(s.cfg.TLSDir, "tls.key"))
	if err != nil {
		return fmt.Errorf("load the certificate of the revocation feed: %w", err)
	}
	ln, err := listen(ctx, s.cfg.Addr)
	if err != nil {
		return err
	}
	return s.serve(ctx, ln, &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}})
}

func (s *Server) serve(ctx context.Context, ln net.Listener, tlsConfig *tls.Config) error {
	srv := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		TLSConfig:         tlsConfig,
		// Every request ends with ctx: the streams close when the manager stops.
		BaseContext: func(net.Listener) context.Context { return ctx },
		ErrorLog:    log.New(debugWriter{log: s.log}, "", 0),
	}
	served := make(chan error, 1)
	go func() { served <- srv.ServeTLS(ln, "", "") }()
	s.log.Info("Serving the revocation feed", "addr", ln.Addr().String())

	select {
	case err := <-served:
		return fmt.Errorf("serve the revocation feed: %w", err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close()
	}
	return nil
}

// Handler is the HTTP handler of the feed.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+Path, s.serveRevocations)
	return mux
}

func (s *Server) serveRevocations(w http.ResponseWriter, r *http.Request) {
	now := s.now()
	address := remoteAddress(r)
	if s.failures.limited(address, now) {
		s.refuse(w, http.StatusTooManyRequests, retryAfterFailures)
		return
	}
	c, instance, ok, err := s.authenticate(r)
	if err != nil {
		s.log.Error(err, "Failed to authenticate a client of the revocation feed")
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	if !ok {
		s.failures.fail(address, now)
		s.metrics.connections.WithLabelValues(resultUnauthorized).Inc()
		w.Header().Set("WWW-Authenticate", realm)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !acceptsEventStream(r.Header.Values("Accept")) {
		http.Error(w, "the feed is served as text/event-stream", http.StatusNotAcceptable)
		return
	}
	if !s.connectionRate(c.id).AllowN(now, 1) {
		s.refuse(w, http.StatusTooManyRequests, retryAfterConnections)
		return
	}

	var since *cursor
	raw := r.Header.Get("Last-Event-ID")
	if raw == "" {
		raw = r.URL.Query().Get("since")
	}
	if cur, ok := parseCursor(raw); ok {
		since = &cur
	}
	sub, err := s.hub.subscribe(c, instance, since, now)
	if err != nil {
		s.refuse(w, http.StatusTooManyRequests, retryAfterStreams)
		return
	}
	defer s.hub.unsubscribe(sub.stream)

	s.metrics.connections.WithLabelValues(resultOK).Inc()
	if sub.resumed {
		s.metrics.connections.WithLabelValues(resultResumed).Inc()
	} else {
		s.metrics.connections.WithLabelValues(resultSnapshot).Inc()
	}
	s.stream(r.Context(), w, sub)
}

// stream writes the start of the stream, a snapshot or the backlog, then the events of sub and the
// heartbeats until the request ends, the stream falls behind, or a write fails.
func (s *Server) stream(ctx context.Context, w http.ResponseWriter, sub subscription) {
	st := sub.stream
	out := &eventWriter{w: w, rc: http.NewResponseController(w), timeout: s.cfg.Limits.WriteTimeout, now: s.now, stream: st}
	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-store")
	header.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if err := out.write("retry: " + strconv.Itoa(retryMillis) + "\n\n"); err != nil {
		return
	}

	last := sub.position.seq
	if sub.resumed {
		for _, rec := range sub.backlog {
			if err := s.writeRecord(out, st, rec); err != nil {
				return
			}
			last = rec.seq
		}
	} else if err := s.writeSnapshot(ctx, out, st, sub.position); err != nil {
		if ctx.Err() == nil {
			s.log.Error(err, "Failed to send a snapshot of the revocation feed", "clientID", st.client.id)
		}
		return
	}

	heartbeat := time.NewTicker(s.cfg.Limits.Heartbeat)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-st.done:
			return
		case it := <-st.events:
			if it.announced != "" {
				if err := s.writeAnnounced(ctx, out, st, it.announced, last); err != nil {
					return
				}
				continue
			}
			if err := s.writeRecord(out, st, it.rec); err != nil {
				return
			}
			last = it.rec.seq
		case <-heartbeat.C:
			data, err := heartbeatEvent(s.now())
			if err != nil {
				return
			}
			if err := out.event(eventHeartbeat, s.hub.idleCursor(st, last), data); err != nil {
				return
			}
		}
	}
}

// writeRecord writes rec to the stream of a client, or, to the stream of an instance, once for
// every client of the instance that rec names, each with the salt of that client.
func (s *Server) writeRecord(out *eventWriter, st *stream, rec record) error {
	position := cursor{epoch: s.hub.epoch, seq: rec.seq}
	if st.instance == "" {
		return s.writeRevocation(out, position, rec, st.client.salt)
	}
	for _, id := range rec.clients {
		c, ok := s.clients.ofInstance(st.instance, id)
		if !ok {
			continue
		}
		if err := s.writeRevocation(out, position, rec, c.salt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) writeRevocation(out *eventWriter, position cursor, rec record, salt []byte) error {
	data, err := revocationEvent(rec, salt)
	if err != nil {
		return err
	}
	if err := out.event(eventRevocation, position, data); err != nil {
		return err
	}
	s.metrics.observeDelivery(rec.created, s.now())
	return nil
}

// writeAnnounced writes what is revoked for a client that has appeared among the clients of the
// instance of st, as revocation events: the subscriber merges them into what it has.
func (s *Server) writeAnnounced(ctx context.Context, out *eventWriter, st *stream, clientID string, last uint64) error {
	c, ok := s.clients.ofInstance(st.instance, clientID)
	if !ok {
		return nil
	}
	revs, err := s.revocationsOf(ctx, clientID)
	if err != nil {
		s.log.Error(err, "Failed to send the revocations of a new client of an instance", "clientID", clientID)
		return err
	}
	position := s.hub.idleCursor(st, last)
	for _, rev := range revs {
		rec, ok := recordOf(rev, s.cfg.Retention)
		if !ok || rec.exp < s.now().Unix() {
			continue
		}
		if err := s.writeRevocation(out, position, rec, c.salt); err != nil {
			return err
		}
	}
	return nil
}

// revocationsOf lists the revocations that name a client.
func (s *Server) revocationsOf(ctx context.Context, clientID string) ([]revocation.Revocation, error) {
	list := controller.List(controller.RevocationGVK)
	if err := s.reader.List(ctx, list, client.InNamespace(naming.DexNamespace), client.MatchingFields{controller.RevocationClientField: clientID}); err != nil {
		return nil, fmt.Errorf("list revocations: %w", err)
	}
	revs := make([]revocation.Revocation, 0, len(list.Items))
	for i := range list.Items {
		rev, err := revocation.FromObject(&list.Items[i])
		if err != nil {
			s.log.Error(err, "Leaving a revocation out", "revocation", list.Items[i].GetName())
			continue
		}
		revs = append(revs, rev)
	}
	return revs, nil
}

// authenticate tells who asks for the feed: a client with its secret, or a shared dex-authenticator
// instance with the token of its ServiceAccount. For an instance the client is a name the limits
// count its streams under.
func (s *Server) authenticate(r *http.Request) (feedClient, string, bool, error) {
	token, isInstance := bearerToken(r)
	if !isInstance {
		c, ok, err := authenticate(r.Context(), s.reader, r)
		return c, "", ok, err
	}
	instance, ok, err := authenticateInstance(r.Context(), s.cfg.TokenReview, token)
	if err != nil || !ok {
		return feedClient{}, "", false, err
	}
	return feedClient{id: instanceID(instance)}, instance, true, nil
}

// writeSnapshot writes the snapshot of the client of st, or one snapshot of all the clients of the
// instance of st.
func (s *Server) writeSnapshot(ctx context.Context, out *eventWriter, st *stream, position cursor) error {
	clients := []feedClient{st.client}
	if st.instance != "" {
		clients = s.clients.byInstance(st.instance)
	}
	groups := make([]salted, 0, len(clients))
	for _, c := range clients {
		revs, err := s.revocationsOf(ctx, c.id)
		if err != nil {
			return err
		}
		groups = append(groups, salted{salt: c.salt, revs: revs})
	}
	parts, err := snapshotPartsOf(groups, s.cfg.Retention, s.now(), s.cfg.Limits.SnapshotPart)
	if err != nil {
		return err
	}
	for _, part := range parts {
		if err := out.event(eventSnapshot, position, part); err != nil {
			return err
		}
	}
	return nil
}

// onRevocation hands a new Revocation to the hub. The objects of the initial list, and those of
// a relist, are not numbered: they go into snapshots only.
func (s *Server) onRevocation(obj any, isInInitialList bool) {
	if isInInitialList {
		return
	}
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return
	}
	rev, err := revocation.FromObject(u)
	if err != nil {
		s.log.Error(err, "Failed to decode a revocation", "revocation", u.GetName())
		return
	}
	rec, ok := recordOf(rev, s.cfg.Retention)
	if !ok {
		return
	}
	s.hub.publish(rec)
}

func (s *Server) refuse(w http.ResponseWriter, status int, retryAfter string) {
	s.metrics.connections.WithLabelValues(resultLimited).Inc()
	w.Header().Set("Retry-After", retryAfter)
	http.Error(w, http.StatusText(status), status)
}

func (s *Server) connectionRate(clientID string) *rate.Limiter {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	l, ok := s.connRates[clientID]
	if !ok {
		l = rate.NewLimiter(rate.Limit(s.cfg.Limits.ConnectionsPerSecond), s.cfg.Limits.ConnectionBurst)
		s.connRates[clientID] = l
	}
	return l
}

// eventWriter writes the events of one stream, each with its own write deadline, and flushes them.
type eventWriter struct {
	w       io.Writer
	rc      *http.ResponseController
	timeout time.Duration
	now     func() time.Time
	stream  *stream
}

func (e *eventWriter) event(name string, id cursor, data []byte) error {
	return e.write("event: " + name + "\nid: " + id.String() + "\ndata: " + string(data) + "\n\n")
}

func (e *eventWriter) write(text string) error {
	// The deadline of the connection is wall-clock time; the injected clock only stamps events.
	if err := e.rc.SetWriteDeadline(time.Now().Add(e.timeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	if _, err := io.WriteString(e.w, text); err != nil {
		return err
	}
	if err := e.rc.Flush(); err != nil {
		return err
	}
	e.stream.lastWrite.Store(e.now().UnixNano())
	return nil
}

// acceptsEventStream reports whether the Accept headers name text/event-stream.
func acceptsEventStream(values []string) bool {
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(part))
			if err == nil && mediaType == "text/event-stream" {
				return true
			}
		}
	}
	return false
}

func remoteAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// addressLimiter bounds the failed sign-ins of each address.
type addressLimiter struct {
	mu       sync.Mutex
	limit    rate.Limit
	burst    int
	limiters map[string]*rate.Limiter
}

// sweepAbove is how many addresses the limiter keeps before it forgets those back at full burst.
const sweepAbove = 1024

func newAddressLimiter(perSecond float64, burst int) *addressLimiter {
	return &addressLimiter{limit: rate.Limit(perSecond), burst: burst, limiters: map[string]*rate.Limiter{}}
}

// limited reports whether address has no failures left.
func (a *addressLimiter) limited(address string, now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	l, ok := a.limiters[address]
	return ok && l.TokensAt(now) < 1
}

// fail records a failed sign-in of address.
func (a *addressLimiter) fail(address string, now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	l, ok := a.limiters[address]
	if !ok {
		if len(a.limiters) >= sweepAbove {
			for addr, other := range a.limiters {
				if other.TokensAt(now) >= float64(a.burst) {
					delete(a.limiters, addr)
				}
			}
		}
		l = rate.NewLimiter(a.limit, a.burst)
		a.limiters[address] = l
	}
	l.AllowN(now, 1)
}

// clientSet is the set of DexAuthenticator clients the cache knows: for the subscribers gauge, and
// for the streams of the shared dex-authenticator instances, which carry the events of every
// client of their instance.
type clientSet struct {
	mu sync.Mutex
	// ids maps an object name to its client id, and byID a client id to the client: the salt of
	// its keys and its instance.
	ids  map[string]string
	byID map[string]feedClient
	// announce is told of a client that has appeared in an instance or has got another secret.
	announce func(instance, clientID string)
}

func newClientSet(announce func(instance, clientID string)) *clientSet {
	return &clientSet{ids: map[string]string{}, byID: map[string]feedClient{}, announce: announce}
}

func (c *clientSet) handler() toolscache.ResourceEventHandler {
	return toolscache.ResourceEventHandlerFuncs{
		AddFunc:    c.set,
		UpdateFunc: func(_, obj any) { c.set(obj) },
		DeleteFunc: c.remove,
	}
}

func (c *clientSet) set(obj any) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return
	}
	ids := controller.OAuth2ClientID(u)
	labels := u.GetLabels()
	_, isAuthenticator := labels[DexAuthenticatorLabel]
	if len(ids) == 0 || !isAuthenticator {
		c.forget(u.GetName())
		return
	}

	salt, err := hex.DecodeString(stringField(u, saltField))
	if err != nil {
		salt = nil
	}
	current := feedClient{id: ids[0], salt: salt, instance: naming.AuthenticatorInstance(labels[DexAuthenticatorNamespaceLabel])}

	c.mu.Lock()
	if previousID, isKnown := c.ids[u.GetName()]; isKnown && previousID != current.id {
		delete(c.byID, previousID)
	}
	previous, isKnown := c.byID[current.id]
	c.ids[u.GetName()] = current.id
	c.byID[current.id] = current
	c.mu.Unlock()

	// The keys of a client with another secret are other keys: the instance gets them anew.
	isSame := isKnown && previous.instance == current.instance && bytes.Equal(previous.salt, current.salt)
	if !isSame && len(salt) > 0 && c.announce != nil {
		c.announce(current.instance, current.id)
	}
}

func (c *clientSet) remove(obj any) {
	if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return
	}
	c.forget(u.GetName())
}

// forget drops the client of the object with name.
func (c *clientSet) forget(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if id, ok := c.ids[name]; ok {
		delete(c.byID, id)
		delete(c.ids, name)
	}
}

func (c *clientSet) list() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	ids := make([]string, 0, len(c.byID))
	for id := range c.byID {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// instanceOf is the instance of the client with id, empty for a client the set does not know.
func (c *clientSet) instanceOf(id string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.byID[id].instance
}

// ofInstance returns the client with id when it belongs to instance and its salt is known.
func (c *clientSet) ofInstance(instance, id string) (feedClient, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	known, ok := c.byID[id]
	if !ok || known.instance != instance || len(known.salt) == 0 {
		return feedClient{}, false
	}
	return known, true
}

// byInstance returns the clients of instance whose salt is known, by id.
func (c *clientSet) byInstance(instance string) []feedClient {
	c.mu.Lock()
	defer c.mu.Unlock()
	clients := make([]feedClient, 0, len(c.byID))
	for _, known := range c.byID {
		if known.instance == instance && len(known.salt) > 0 {
			clients = append(clients, known)
		}
	}
	slices.SortFunc(clients, func(a, b feedClient) int { return strings.Compare(a.id, b.id) })
	return clients
}

// debugWriter sends the messages of the HTTP server (TLS handshake errors of probes and scanners) to
// the debug level.
type debugWriter struct {
	log logr.Logger
}

func (d debugWriter) Write(p []byte) (int, error) {
	d.log.V(1).Info(strings.TrimSpace(string(p)))
	return len(p), nil
}
