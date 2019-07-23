package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"

	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	whhttp "github.com/slok/kubewebhook/pkg/http"
	"github.com/slok/kubewebhook/pkg/log"
	mutatingwh "github.com/slok/kubewebhook/pkg/webhook/mutating"

	"github.com/thanhpk/randstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	cfg        *config
	logger     = &log.Std{Debug: true}
	kubeClient *kubernetes.Clientset
)

const (
	oldIngressAnnotationPrefix = "ingress.kubernetes.io/"
	newIngressAnnotationPrefix = "nginx.ingress.kubernetes.io/"
)

func createOrUpdateIngress(ingress *extensionsv1beta1.Ingress) error {
	_, err := kubeClient.ExtensionsV1beta1().Ingresses(ingress.ObjectMeta.Namespace).Create(ingress)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			_, err := kubeClient.ExtensionsV1beta1().Ingresses(ingress.ObjectMeta.Namespace).Update(ingress)
			logger.Warningf("Ingress already exists, updating existing %s", ingress.Name)
			if err != nil {
				return fmt.Errorf("failed to create -rwr Ingress object: %v", err)
			}
		} else {
			return fmt.Errorf("failed to update -rwr Ingress object: %v", err)
		}
	}

	return nil
}

func rewriteTargetMigrationRequired(ingress *extensionsv1beta1.Ingress) bool {
	if ingress.ObjectMeta.Annotations == nil {
		return false
	}

	if _, ok := ingress.ObjectMeta.Annotations["ingress.flant.com/skip-rewrite-target-migration"]; ok {
		return false
	}

	if _, ok := ingress.Annotations["nginx.ingress.kubernetes.io/rewrite-target"]; !ok {
		return false
	}

	if strings.Contains(ingress.ObjectMeta.Annotations["nginx.ingress.kubernetes.io/rewrite-target"], "$") {
		return false
	}

	for _, ingressRule := range ingress.Spec.Rules {
		for _, path := range ingressRule.HTTP.Paths {
			if strings.Contains(path.Path, "(") && strings.Contains(path.Path, ")") {
				if _, ok := ingress.ObjectMeta.Annotations["nginx.ingress.kubernetes.io/use-regex"]; !ok {
					return false
				}
			}
		}
	}

	return true
}

func migrateAnnotations(ingress *extensionsv1beta1.Ingress) {
	// do not migrate annotations if there are none
	if ingress.Annotations == nil {
		return
	}

	var hasOldAnnotations bool
	for k := range ingress.Annotations {
		if strings.HasPrefix(k, oldIngressAnnotationPrefix) {
			hasOldAnnotations = true
			break
		}
	}

	if hasOldAnnotations {
		for k := range ingress.Annotations {
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
}

func rewriteTargetMigration(ingress *extensionsv1beta1.Ingress) error {
	if !cfg.enableRwr {
		return nil
	}

	if ingress.ObjectMeta.GenerateName != "" {
		ingress.Name = ingress.ObjectMeta.GenerateName + strings.ToLower(randstr.String(5))
		ingress.ObjectMeta.GenerateName = ""
	}

	rwrIngress := ingress.DeepCopy()
	rwrIngress.ObjectMeta = metav1.ObjectMeta{
		Name:            rwrIngress.ObjectMeta.Name + "-rwr",
		Namespace:       rwrIngress.ObjectMeta.Namespace,
		Labels:          rwrIngress.ObjectMeta.Labels,
		Annotations:     rwrIngress.ObjectMeta.Annotations,
		OwnerReferences: rwrIngress.ObjectMeta.OwnerReferences,
	}
	rwrIngress.Status = extensionsv1beta1.IngressStatus{}

	if rwrIngress.Annotations != nil {
		// remove cert-manager annotation and change ingress.class
		delete(rwrIngress.Annotations, "kubernetes.io/tls-acme")
		if _, ok := rwrIngress.Annotations["kubernetes.io/ingress.class"]; !ok {
			rwrIngress.Annotations["kubernetes.io/ingress.class"] = "nginx-rwr"
		} else {
			rwrIngress.Annotations["kubernetes.io/ingress.class"] = rwrIngress.Annotations["kubernetes.io/ingress.class"] + "-rwr"
		}
	}

	if !rewriteTargetMigrationRequired(ingress) {
		return createOrUpdateIngress(rwrIngress)
	}

	for rulePos, ingressRule := range rwrIngress.Spec.Rules {
		for pathPos, path := range ingressRule.HTTP.Paths {
			if rwrIngress.Spec.Rules[rulePos].HTTP.Paths[pathPos].Path == "" || rwrIngress.Spec.Rules[rulePos].HTTP.Paths[pathPos].Path == "/" {
				rwrIngress.Spec.Rules[rulePos].HTTP.Paths[pathPos].Path = "/()(.*)"
			} else {
				rwrIngress.Spec.Rules[rulePos].HTTP.Paths[pathPos].Path = strings.TrimSuffix(path.Path, "/") + "(/|$)(.*)"
			}
		}
	}
	rwrIngress.Annotations["nginx.ingress.kubernetes.io/rewrite-target"] = strings.TrimSuffix(rwrIngress.Annotations["nginx.ingress.kubernetes.io/rewrite-target"], "/") + "/$2"

	return createOrUpdateIngress(rwrIngress)
}

func ingressMutator(_ context.Context, obj metav1.Object) (bool, error) {
	ingress, ok := obj.(*extensionsv1beta1.Ingress)
	if !ok {
		return false, fmt.Errorf("not an Ingress object")
	}

	// completely skip "-rwr" Ingresses that are created by us
	if strings.HasSuffix(ingress.ObjectMeta.Name, "-rwr") {
		return false, nil
	}

	// Mutation step #1: migrate annotation prefixes
	migrateAnnotations(ingress)

	// Mutation step #2: rewrite-target migration: https://github.com/deckhouse/deckhouse/issues/641
	err := rewriteTargetMigration(ingress)
	if err != nil {
		return false, err
	}

	return false, nil
}

type config struct {
	certFile   string
	keyFile    string
	listenAddr string
	enableRwr  bool
}

func initFlags() *config {
	cfg := &config{}

	fl := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fl.StringVar(&cfg.certFile, "tls-cert-file", "", "TLS certificate file")
	fl.StringVar(&cfg.keyFile, "tls-key-file", "", "TLS key file")
	fl.StringVar(&cfg.listenAddr, "listen-address", ":8080", "listen address")
	fl.BoolVar(&cfg.enableRwr, "enable-rwr", false, "enable -rwr Ingresses creation")

	_ = fl.Parse(os.Args[1:])
	if cfg.certFile == "" && cfg.keyFile == "" {
		logger.Errorf("\"tls-cert-file\" and/or \"tls-key-file\" args not provided")
		os.Exit(1)
	}
	return cfg
}

func main() {
	cfg = initFlags()

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	kubeClient, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		panic(err.Error())
	}

	mt := mutatingwh.MutatorFunc(ingressMutator)

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
