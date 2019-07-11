package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"strings"

	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	whhttp "github.com/slok/kubewebhook/pkg/http"
	"github.com/slok/kubewebhook/pkg/log"
	mutatingwh "github.com/slok/kubewebhook/pkg/webhook/mutating"
)

var logger = &log.Std{Debug: true}

const (
	oldIngressAnnotationPrefix = "ingress.kubernetes.io/"
	newIngressAnnotationPrefix = "nginx.ingress.kubernetes.io/"
)

func annotateIngressMutator(_ context.Context, obj metav1.Object) (bool, error) {
	ingress, ok := obj.(*extensionsv1beta1.Ingress)
	if !ok {
		return false, errors.New("not an Ingress object")
	}

	if ingress.Annotations == nil {
		return false, nil
	}

	var hasOldAnnotations bool
	for k, _ := range ingress.Annotations {
		if strings.HasPrefix(k, oldIngressAnnotationPrefix) {
			hasOldAnnotations = true
			break
		}
	}

	if hasOldAnnotations {
		for k, _ := range ingress.Annotations {
			if strings.HasPrefix(k, newIngressAnnotationPrefix) {
				delete(ingress.Annotations, k)
			}
		}

		for k, v := range ingress.Annotations {
			if strings.HasPrefix(k, oldIngressAnnotationPrefix) {
				ingress.Annotations["nginx."+k] = v
			}
		}
	}

	return false, nil
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

	_ = fl.Parse(os.Args[1:])
	if cfg.certFile == "" && cfg.keyFile == "" {
		logger.Errorf("\"tls-cert-file\" and/or \"tls-key-file\" args not provided")
		os.Exit(1)
	}
	return cfg
}

func main() {

	cfg := initFlags()

	mt := mutatingwh.MutatorFunc(annotateIngressMutator)

	mcfg := mutatingwh.WebhookConfig{
		Name: "ingressAnnotate",
		Obj:  &extensionsv1beta1.Ingress{},
	}
	wh, err := mutatingwh.NewWebhook(mcfg, mt, nil, nil, logger)
	if err != nil {
		logger.Errorf("error creating webhook: %s", err)
		os.Exit(1)
	}

	whHandler, err := whhttp.HandlerFor(wh)
	if err != nil {
		logger.Errorf("error creating webhook handler: %s", err)
		os.Exit(1)
	}
	logger.Infof("Listening on :8080")
	err = http.ListenAndServeTLS(cfg.listenAddr, cfg.certFile, cfg.keyFile, whHandler)
	if err != nil {
		logger.Errorf("error serving webhook: %s", err)
		os.Exit(1)
	}
}
