package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"deckhouse-registry-webhook/internal/registryclient"
	"deckhouse-registry-webhook/internal/webhook"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var (
	webhookAddr  = flag.String("webhook-addr", ":8443", "Webhook address and port")
	healthAddr   = flag.String("health-addr", ":8001", "Health address and port")
	imageToCheck = flag.String("image-to-check", "the-name-of-a-nonexistent-image", "Nonexistent image name to check")
	tlsCertFile  = flag.String("tls-cert-file", "/tls/tls.crt", "Path to the TLS certificate file")
	tlsKeyFile   = flag.String("tls-key-file", "/tls/tls.key", "Path to the TLS key file")
	logLevelStr  = flag.String("log-level", "info", "Log level")
)

var (
	BuildDatetime = "none"
	AppName       = "docker-secret-validating-webhook"
)

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// catch signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		s := <-sigChan
		close(sigChan)
		log.Infof("catch signal: %s", s)
		cancel()
	}()

	// Parse command-line flags
	flag.Parse()
	logLevel, err := log.ParseLevel(*logLevelStr)
	if err != nil {
		logLevel = log.InfoLevel
	}

	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(logLevel)
	log.Infof("%s build time %s", AppName, BuildDatetime)

	// health endpoint
	health := mux.NewRouter()
	health.PathPrefix("/healthz").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	healthSrv := http.Server{
		Addr:    *healthAddr,
		Handler: health,
	}
	log.Infof("starting healthz on %s", *healthAddr)
	go func() { _ = healthSrv.ListenAndServe() }()

	registryClient := registryclient.NewRegistryClient()
	// run webnhook
	wh := webhook.NewValidatingWebhook(*webhookAddr, *imageToCheck, *tlsCertFile, *tlsKeyFile, registryClient)
	err = wh.Run(ctx)
	if err != nil {
		log.Errorf("error serving webhook: %v", err)
	}
}
