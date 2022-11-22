/*
Copyright 2022 Flant JSC

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

package main

import (
	"flag"
	d8config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	module_manager "github.com/deckhouse/deckhouse/go_lib/deckhouse-config/module-manager"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"
	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	kwhlogrus "github.com/slok/kubewebhook/v2/pkg/log/logrus"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type config struct {
	certFile   string
	keyFile    string
	listenAddr string
	debug      bool
}

func initFlags() *config {
	cfg := &config{}

	fl := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fl.StringVar(&cfg.certFile, "tls-cert-file", "", "TLS certificate file")
	fl.StringVar(&cfg.keyFile, "tls-key-file", "", "TLS key file")
	fl.StringVar(&cfg.listenAddr, "listen-address", ":8080", "listen address")
	fl.BoolVar(&cfg.debug, "debug", false, "debug logging")

	_ = fl.Parse(os.Args[1:])
	if cfg.certFile == "" && cfg.keyFile == "" {
		log.Fatal(`"tls-cert-file" and/or "tls-key-file" args not provided`)
	}
	return cfg
}

func main() {
	cfg := initFlags()

	log.SetFormatter(&log.JSONFormatter{})
	logLevel := log.InfoLevel
	if cfg.debug {
		logLevel = log.DebugLevel
	}
	log.SetLevel(logLevel)
	kl := kwhlogrus.NewLogrus(log.NewEntry(log.StandardLogger()))

	cmValidator := NewConfigMapValidator(os.Getenv("CONFIG_MAP_NAMES"), os.Getenv("ALLOWED_USERS"))
	log.Infof("Allow modifying ConfigMaps %v by %v users only.", cmValidator.eligibleNames, cmValidator.allowedUsers)

	cmWebhook, err := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "configMapValidator",
		Obj:       &v1.ConfigMap{},
		Validator: cmValidator,
		Logger:    kl,
	})
	if err != nil {
		log.Fatalf("create ConfigMap validator: %s", err)
	}

	// Prepare deckhouse-config service.
	globalHooksDir := os.Getenv("GLOBAL_HOOKS_DIR")
	modulesDir := os.Getenv("MODULES_DIR")
	log.Infof("Use OpenAPI schemas from '%s' and '%s'.", globalHooksDir, modulesDir)

	mm, err := module_manager.InitBasic(globalHooksDir, modulesDir)
	if err != nil {
		log.Fatalf("Init ModuleManager failed: %s", err)
	}
	d8config.InitService(mm)

	// Create validating webhook for ModuleConfig objects.
	configValidator := NewModuleConfigValidator(globalHooksDir, modulesDir)
	configWebhook, err := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "configValidator",
		Obj:       &unstructured.Unstructured{},
		Validator: configValidator,
		Logger:    kl,
	})
	if err != nil {
		log.Fatalf("create ModuleConfig validator: %s", err)
	}

	mux := http.NewServeMux()

	mux.Handle("/validate-cm", kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: cmWebhook, Logger: kl}))
	mux.Handle("/validate", kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: configWebhook, Logger: kl}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })

	log.Infof("Listening on %s", cfg.listenAddr)

	err = http.ListenAndServeTLS(cfg.listenAddr, cfg.certFile, cfg.keyFile, mux)
	if err != nil {
		log.Fatalf("error serving webhook: %s", err)
	}
}
