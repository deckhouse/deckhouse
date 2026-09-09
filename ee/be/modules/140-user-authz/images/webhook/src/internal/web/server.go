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
	// network, so this port must be free on the node (see the DaemonSet).
	MetricsListenAddr = "127.0.0.1:4208"

	metricsNamespace = "user_authz_webhook"
)

func buildTLSConfig() (*tls.Config, error) {
	clientCertPool := x509.NewCertPool()

	{ // kube-apiserver requests
		clientCertBytes, err := os.ReadFile(authClientCA)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %v", authClientCA, err)
		}
		clientCertPool.AppendCertsFromPEM(clientCertBytes)
	}
	{ // kubelet liveness probe requests
		clientCertBytes, err := os.ReadFile(sslListenCert)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %v", sslListenCert, err)
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
	rbacEvaluator := hook.NewRBACEvaluator(logger, informerFactory)

	// The index of rule bindings is fed from the same ClusterRoleBinding informer: it tells the
	// handler which rules bind a subject, so a rule the webhook has not observed yet still restricts.
	ruleBindings := binding.NewIndex()
	if _, err := informerFactory.Rbac().V1().ClusterRoleBindings().Informer().AddEventHandler(ruleBindings.EventHandler()); err != nil {
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

	h, err := hook.NewHandler(logger, c, nsInformer.Lister(), nsInformer.Informer().HasSynced, rbacEvaluator, rulesSource, ruleBindings)
	if err != nil {
		return nil, err
	}
	return &Server{
		logger:          logger,
		cache:           c,
		handler:         h,
		informerFactory: informerFactory,
		informersSynced: append([]kcache.InformerSynced{nsInformer.Informer().HasSynced}, rbacEvaluator.Synced()...),
		rules:           rulesSource,
		registry:        registry,
	}, nil
}

func (s *Server) prepareHTTPServer() (*http.Server, error) {
	router := http.NewServeMux()

	router.Handle("/", s.handler)
	router.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		err := s.cache.Check()
		if err == nil {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "Ok. rules: %s", s.rules.State())
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
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

// Run starts webhook server and its configuration renewal. It will exit only if the webserver stops listening.
func (s *Server) Run() error {
	httpServer, err := s.prepareHTTPServer()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.informerFactory.Start(ctx.Done())

	if ok := kcache.WaitForCacheSync(ctx.Done(), s.informersSynced...); !ok {
		return fmt.Errorf("failed to sync informer caches")
	}

	go s.rules.Run(ctx)
	go s.serveMetrics(ctx)

	httpServer.RegisterOnShutdown(cancel)

	s.logger.Println("server is starting to listen on ", ListenAddr, "...")

	if err = httpServer.ListenAndServeTLS(sslListenCert, sslListenKey); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("could not listen on %s: %v", ListenAddr, err)
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
