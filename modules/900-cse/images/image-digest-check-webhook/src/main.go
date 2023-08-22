package main

import (
	"flag"
	"os"

	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme      = runtime.NewScheme()
	setupLog    = ctrl.Log.WithName("setup")
	debug       bool
	host        string
	port        int
	metricsAddr string
	probeAddr   string
	certDir     string
	tlsCertFile string
	tlsKeyFile  string
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func initApp() {
	flag.StringVar(&host, "host", "0.0.0.0", "The address the manager endpoint binds to.")
	flag.IntVar(&port, "port", 9443, "The port the manager endpoint binds to.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8080", "The address the probe endpoint binds to.")
	flag.StringVar(&certDir, "cert-dir", "", "The directory with a certificates.")
	flag.StringVar(&tlsCertFile, "tls-cert-file-name", "tls.crt", "The the server certificate name.")
	flag.StringVar(&certDir, "tls-key-file-name", "tls.key", "The the server key name.")

	initLogger()

	flag.Parse()
}

func initLogger() {
	if os.Getenv("DEBUG") != "" && os.Getenv("DEBUG") == "yes" {
		debug = true
	}

	level := zapcore.InfoLevel
	if debug {
		level = zapcore.DebugLevel
	}

	opts := zap.Options{
		Development: debug,
		Level:       level,
	}
	opts.BindFlags(flag.CommandLine)

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
}

func main() {
	initApp()

	mgr, err := ctrl.NewManager(
		ctrl.GetConfigOrDie(),
		ctrl.Options{
			Scheme:                 scheme,
			MetricsBindAddress:     metricsAddr,
			Host:                   host,
			Port:                   port,
			HealthProbeBindAddress: probeAddr,
			WebhookServer: webhook.NewServer(
				webhook.Options{
					CertDir:  certDir,
					CertName: tlsCertFile,
					KeyName:  tlsKeyFile,
				},
			),
		},
	)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	mgr.GetWebhookServer().Register("/validate", &webhook.Admission{Handler: &ImageDigestCheckWebhook{}})

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
