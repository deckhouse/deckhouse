/*
Copyright 2021 Flant JSC

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
	"context"
	"flag"
	"net/http"
	"os"

	"github.com/sirupsen/logrus"
	kwhlogrus "github.com/slok/kubewebhook/v2/pkg/log/logrus"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	whhttp "github.com/slok/kubewebhook/v2/pkg/http"
	"github.com/slok/kubewebhook/v2/pkg/webhook/mutating"
)

func mutateIngress(_ context.Context, _ *kwhmodel.AdmissionReview, obj metav1.Object) (mr *mutating.MutatorResult, err error) {
	annotations := obj.GetAnnotations()

	for annotation, value := range annotations {
		// Migration is based on
		// https://cert-manager.io/docs/installation/upgrading/upgrading-0.10-0.11/#additional-annotation-changes
		switch annotation {
		case "certmanager.k8s.io/acme-http01-edit-in-place":
			addIfNotExists(annotations, "acme.cert-manager.io/http01-edit-in-place", value)

		case "certmanager.k8s.io/acme-http01-ingress-class":
			addIfNotExists(annotations, "acme.cert-manager.io/http01-ingress-class", value)

		case "certmanager.k8s.io/issuer":
			addIfNotExists(annotations, "cert-manager.io/issuer", value)

		case "certmanager.k8s.io/cluster-issuer":
			addIfNotExists(annotations, "cert-manager.io/cluster-issuer", value)

		case "certmanager.k8s.io/alt-names":
			addIfNotExists(annotations, "cert-manager.io/alt-names", value)

		case "certmanager.k8s.io/ip-sans":
			addIfNotExists(annotations, "cert-manager.io/ip-sans", value)

		case "certmanager.k8s.io/common-name":
			addIfNotExists(annotations, "cert-manager.io/common-name", value)

		case "certmanager.k8s.io/issuer-name":
			addIfNotExists(annotations, "cert-manager.io/issuer-name", value)

		case "certmanager.k8s.io/issuer-kind":
			addIfNotExists(annotations, "cert-manager.io/issuer-kind", value)
		}
	}

	obj.SetAnnotations(annotations)
	return &mutating.MutatorResult{MutatedObject: obj}, nil
}

func addIfNotExists(obj map[string]string, key, value string) {
	if _, ok := obj[key]; !ok {
		obj[key] = value
	}
}

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

	klog.InitFlags(fl)

	_ = fl.Parse(os.Args[1:])
	if cfg.certFile == "" && cfg.keyFile == "" {
		klog.Fatal(`"tls-cert-file" and/or "tls-key-file" args not provided`)
	}
	return cfg
}

func main() {
	cfg := initFlags()
	logrusLogEntry := logrus.NewEntry(logrus.New())
	logLevel := logrus.WarnLevel
	if cfg.debug {
		logLevel = logrus.DebugLevel
	}
	logrusLogEntry.Logger.SetLevel(logLevel)
	kl := kwhlogrus.NewLogrus(logrusLogEntry)

	wh, err := mutating.NewWebhook(
		mutating.WebhookConfig{
			ID:      "ingressAnnotate",
			Obj:     &unstructured.Unstructured{},
			Mutator: mutating.MutatorFunc(mutateIngress),
			Logger:  kl,
		})
	if err != nil {
		klog.Fatalf("error creating webhook: %s", err)
	}

	mux := http.NewServeMux()

	mux.Handle("/mutate", whhttp.MustHandlerFor(whhttp.HandlerConfig{Webhook: wh, Logger: kl}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })

	klog.Info("Listening on :8080")

	err = http.ListenAndServeTLS(cfg.listenAddr, cfg.certFile, cfg.keyFile, mux)
	if err != nil {
		klog.Fatalf("error serving webhook: %s", err)
	}
}
