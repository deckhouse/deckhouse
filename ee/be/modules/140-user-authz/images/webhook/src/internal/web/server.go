/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package web

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kcache "k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/go_lib/user-authz/binding"
	"github.com/deckhouse/deckhouse/go_lib/user-authz/decision"
	"github.com/deckhouse/deckhouse/go_lib/user-authz/metrics"
	"github.com/deckhouse/deckhouse/go_lib/user-authz/source"

	discoverycache "webhook/internal/cache"
	"webhook/internal/web/hook"
)

const (
	// Webhook tls certificates
	sslWebhookPath = "/etc/ssl/user-authz-webhook/"
	sslListenCert  = sslWebhookPath + "tls.crt"
	sslListenKey   = sslWebhookPath + "tls.key"

	// CA to verify kube-apiserver client certificate
	// https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#authenticate-apiservers
	authClientCA = "/etc/ssl/apiserver-authentication-requestheader-client-ca/ca.crt"

	ListenAddr = "127.0.0.1:40443"

	// MetricsListenAddr is a node-local plaintext listener that serves only /metrics. A
	// kube-rbac-proxy sidecar fronts it on the pod IP for Prometheus; the authorization listener
	// above stays mutually authenticated for the API server alone. The webhook runs on the host
	// network, so this port must be free on the node - it is not declared as a containerPort
	// anywhere, being loopback-only; the DaemonSet names it once, in the sidecar's UPSTREAM_URL,
	// and the two have to agree.
	MetricsListenAddr = "127.0.0.1:4243"

	metricsNamespace = "user_authz_webhook"
)

func buildTLSConfig() (*tls.Config, error) {
	clientCertPool := x509.NewCertPool()

	{ // kube-apiserver requests
		clientCertBytes, err := os.ReadFile(authClientCA)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", authClientCA, err)
		}
		clientCertPool.AppendCertsFromPEM(clientCertBytes)
	}
	{ // kubelet liveness probe requests
		clientCertBytes, err := os.ReadFile(sslListenCert)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", sslListenCert, err)
		}
		clientCertPool.AppendCertsFromPEM(clientCertBytes)
	}

	return &tls.Config{
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  clientCertPool,
	}, nil
}

type Server struct {
	cache           discoverycache.Cache
	handler         *hook.Handler
	logger          *log.Logger
	informerFactory informers.SharedInformerFactory
	informersSynced []kcache.InformerSynced
	rules           *source.Source
	registry        *prometheus.Registry

	// synced is set once every cache the decision depends on has been filled. The listener opens
	// before that, so the field is what tells the two apart.
	synced atomic.Bool

	// startupRefusals throttles the line gateOnCaches writes while the caches fill.
	startupRefusals decision.Throttle
}

func NewServer(logger *log.Logger) (*Server, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	c := discoverycache.NewNamespacedDiscoveryCache(logger, config.Host)
	informerFactory := informers.NewSharedInformerFactory(clientSet, 0)
	nsInformer := informerFactory.Core().V1().Namespaces()

	// RBAC informers back the CAR-independent grants check: requests allowed
	// by RoleBindings or non-CAR ClusterRoleBindings must not be denied.
	rbacEvaluator, err := hook.NewRBACEvaluator(logger, informerFactory)
	if err != nil {
		return nil, err
	}

	// The index of rule bindings is fed from the same ClusterRoleBinding informer: it tells the
	// handler which rules bind a subject, so a rule the webhook has not observed yet still restricts.
	//
	// The registration's own HasSynced is what the listener has to wait for, not the informer's:
	// the informer reports synced once the initial list has been popped, while handlers are fed
	// from a separate queue. Serving before the index is filled would let a rule-bound subject look
	// unbound, and the cluster-wide binding of its rule would then grant it every namespace. The
	// API server caches that for unauthorizedTTL: this webhook never answers Allow, so no answer
	// of its own is ever cached under authorizedTTL, the no-opinion ones included.
	ruleBindings := binding.NewIndex()
	ruleBindingsSynced, err := informerFactory.Rbac().V1().ClusterRoleBindings().Informer().AddEventHandler(ruleBindings.EventHandler())
	if err != nil {
		return nil, fmt.Errorf("register rule bindings index: %w", err)
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	rulesMetrics := metrics.New(metricsNamespace)
	if err := rulesMetrics.Register(registry); err != nil {
		return nil, fmt.Errorf("register rules metrics: %w", err)
	}

	// The rules are read from the ClusterAuthorizationRules themselves. The source is not part of
	// the caches the listener waits for: at bootstrap the CRD may not exist yet, and a webhook that
	// does not listen denies every request of the cluster (the authorizer is fail-closed). Until the
	// rules are listed, the bindings index makes every subject of a rule maximally restricted.
	rulesSource := source.New(dynamicClient, source.Options{
		Observer: rulesMetrics,
		Logf:     logger.Printf,
	})

	logDecisions := hook.DecisionLogModeFrom(os.Getenv(hook.DecisionLogEnv), logger)
	h, err := hook.NewHandler(logger, c, nsInformer.Lister(), nsInformer.Informer().HasSynced, rbacEvaluator, rulesSource, ruleBindings, logDecisions)
	if err != nil {
		return nil, err
	}
	return &Server{
		logger:          logger,
		cache:           c,
		handler:         h,
		informerFactory: informerFactory,
		informersSynced: append([]kcache.InformerSynced{
			nsInformer.Informer().HasSynced,
			ruleBindingsSynced.HasSynced,
			rulesListed(rulesSource),
		}, rbacEvaluator.Synced()...),
		rules:           rulesSource,
		registry:        registry,
		startupRefusals: decision.Throttle{Every: time.Second},
	}, nil
}

// rulesListed reports whether the rules are known well enough to decide with.
//
// It has to be part of the serving gate, not just of readiness. The bindings index and the rules
// come over independent watches, and the ordering guard turns "this subject is bound by a rule I
// have not observed" into a maximally restricted entry - which is right while the rules are merely
// behind, and catastrophic if the rules were never listed at all: with an empty directory and a
// filled index, EVERY subject whose access comes only from a ClusterAuthorizationRule is denied,
// and the API server caches each of those denials for unauthorizedTTL. Serving that is strictly
// worse than serving 503, which is not cached. Gating on it costs nothing, because a webhook that
// cannot decide has nothing to say.
//
// A cluster whose CRD is not served is ready: there can be no rules, so the empty directory is the
// truth rather than a gap.
//
// A directory that HAS been listed and has since gone stale still counts as listed here, and that
// is deliberate. Stale means the watch is broken, not that the rules are unknown; the directory is
// a real snapshot of the cluster as of whenever the watch broke, and answering from it is far
// better than what closing this gate would do - 503 on every authorization request in the cluster.
// Staleness belongs in readiness, which holds a rollout and shows up in monitoring without taking
// anything away. See the readyz handler below.
func rulesListed(src *source.Source) kcache.InformerSynced {
	return func() bool {
		return src.State() != source.StateUnsynced
	}
}

func (s *Server) prepareHTTPServer() (*http.Server, error) {
	router := http.NewServeMux()

	router.Handle("/", s.gateOnCaches(s.handler))

	// Liveness. It answers from this process alone, and a restart is the right response to it
	// failing. It deliberately does not ask the API server: the webhook decides from its caches, so
	// it keeps working while the API server is away, and restarting it then only throws those
	// caches away and rebuilds them against an API server that is still away. Tying liveness to the
	// API server is what let the kubelet kill this container at the moment the API server came
	// back, taking authorization for the whole cluster with it.
	router.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "Ok. rules: %s", s.rules.State())
	})

	// Readiness. Here the API server does belong: a webhook that cannot reach it will not see new
	// rules, and a rollout must not move on to the next master while that is true. Nothing routes
	// to this Pod, so the only effect of being unready is to hold the rollout and show up.
	router.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if !s.synced.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintln(w, "the informer caches are still filling")
			return
		}
		if err := s.cache.Check(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, "the api server is unreachable: %v", err)
			return
		}
		// A watch that has been failing for long enough that the directory no longer tracks the
		// cluster. The webhook keeps answering from what it has - see rulesListed - but it should
		// not look healthy while doing it, and a rollout should not move on to the next master.
		if s.rules.State() == source.StateStale {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, "the rules are stale: %v", s.rules.LastError())
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "Ok. rules: %s", s.rules.State())
	})

	tlsCfg, err := buildTLSConfig()
	if err != nil {
		return nil, err
	}

	srv := &http.Server{
		Addr:         ListenAddr,
		TLSConfig:    tlsCfg,
		Handler:      router,
		ErrorLog:     s.logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	return srv, nil
}

// gateOnCaches refuses authorization requests until the caches the decision reads are filled.
//
// It answers 503 rather than a denial on purpose. A denial is an answer, and the API server caches
// answers for unauthorizedTTL - a deny issued during startup would outlive the startup by that long
// and keep denying after the webhook was ready. An error is not cached, so the very first request
// after the caches fill is decided properly. It is also the same outcome the closed port used to
// produce, so nothing becomes more permissive; it just stops being a mystery in the logs.
func (s *Server) gateOnCaches(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.synced.Load() {
			// One line per second, not one per request. Every authorization request in the cluster
			// arrives here while the caches fill, and on the cluster where that takes longest -
			// tens of thousands of bindings - the request rate is highest too, so the unthrottled
			// line turned a slow startup into a flood in the master's log at the moment the
			// operator most needs to read it.
			if write, suppressed := s.startupRefusals.Allow(time.Now()); write {
				if suppressed > 0 {
					s.logger.Printf("refusing authorization requests: the informer caches are still filling (%d more refused in the last second)", suppressed)
				} else {
					s.logger.Printf("refusing an authorization request: the informer caches are still filling")
				}
			}
			http.Error(w, "user-authz webhook: the informer caches are still filling", http.StatusServiceUnavailable)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Run starts webhook server and its configuration renewal. It will exit only if the webserver stops listening.
func (s *Server) Run() error {
	httpServer, err := s.prepareHTTPServer()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.informerFactory.Start(ctx.Done())

	// Open the listener before the caches are warm. On a cluster with tens of thousands of
	// ClusterRoleBindings the sync takes long enough that the port used to appear minutes after the
	// process started, and until it did the API server got a connection error on every request and
	// the kubelet's probes killed a container that was making progress. Requests that arrive early
	// are refused by gateOnCaches, which is the same outcome as a closed port but says why.
	// Start the rules source before waiting on it: rulesListed is one of the predicates below.
	go s.rules.Run(ctx)

	go func() {
		if ok := kcache.WaitForCacheSync(ctx.Done(), s.informersSynced...); !ok {
			s.logger.Println("informer caches were not synced before shutdown")
			return
		}
		s.synced.Store(true)
		s.logger.Println("informer caches are synced; serving authorization decisions")
	}()

	go s.serveMetrics(ctx)

	httpServer.RegisterOnShutdown(cancel)

	s.logger.Println("server is starting to listen on ", ListenAddr, "...")

	if err = httpServer.ListenAndServeTLS(sslListenCert, sslListenKey); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("could not listen on %s: %w", ListenAddr, err)
	}

	return nil
}

// serveMetrics runs the node-local plaintext /metrics listener until the context is cancelled. A
// failure to listen is logged, not fatal: the webhook authorizes requests whether or not its
// metrics are scraped, and letting the authorizer stay up matters more than the metrics.
func (s *Server) serveMetrics(ctx context.Context) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{}))

	srv := &http.Server{
		Addr:         MetricsListenAddr,
		Handler:      mux,
		ErrorLog:     s.logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.logger.Printf("metrics listener on %s stopped: %v", MetricsListenAddr, err)
	}
}
