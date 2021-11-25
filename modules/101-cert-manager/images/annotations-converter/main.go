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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	whhttp "github.com/slok/kubewebhook/pkg/http"
	"github.com/slok/kubewebhook/pkg/webhook/mutating"
)

func mutateIngress(_ context.Context, obj metav1.Object) (stop bool, err error) {
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
	return true, nil
}

func addIfNotExists(obj map[string]string, key, value string) {
	if _, ok := obj[key]; !ok {
		obj[key] = value
	}
}

type klogLogger struct{}

func (*klogLogger) Infof(format string, args ...interface{}) {
	klog.Infof(format, args...)
}

func (*klogLogger) Errorf(format string, args ...interface{}) {
	klog.Errorf(format, args...)
}

func (*klogLogger) Warningf(format string, args ...interface{}) {
	klog.Warningf(format, args...)
}

func (*klogLogger) Debugf(format string, args ...interface{}) {
	klog.Warningf(format, args...)
}

type config struct {
	certFile   string
	keyFile    string
	listenAddr string
}

func initFlags() *config {
	cfg := &config{}

	fl := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fl.StringVar(&cfg.certFile, "tls-cert-file", "", "TLS certificate file")
	fl.StringVar(&cfg.keyFile, "tls-key-file", "", "TLS key file")
	fl.StringVar(&cfg.listenAddr, "listen-address", ":8080", "listen address")

	klog.InitFlags(fl)

	_ = fl.Parse(os.Args[1:])
	if cfg.certFile == "" && cfg.keyFile == "" {
		klog.Fatal(`"tls-cert-file" and/or "tls-key-file" args not provided`)
	}
	return cfg
}

func main() {
	cfg := initFlags()

	wh, err := mutating.NewWebhook(
		mutating.WebhookConfig{
			Name: "ingressAnnotate",
			Obj:  &unstructured.Unstructured{},
		},
		mutating.MutatorFunc(mutateIngress),
		nil, nil, &klogLogger{})
	if err != nil {
		klog.Fatalf("error creating webhook: %s", err)
	}

	mux := http.NewServeMux()

	mux.Handle("/mutate", whhttp.MustHandlerFor(wh))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })

	klog.Info("Listening on :8080")

	err = http.ListenAndServeTLS(cfg.listenAddr, cfg.certFile, cfg.keyFile, mux)
	if err != nil {
		klog.Fatalf("error serving webhook: %s", err)
	}
}
