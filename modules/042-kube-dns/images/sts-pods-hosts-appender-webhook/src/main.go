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
	"fmt"
	"net/http"
	"os"

	"github.com/sirupsen/logrus"
	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	kwhlogrus "github.com/slok/kubewebhook/v2/pkg/log/logrus"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhmutating "github.com/slok/kubewebhook/v2/pkg/webhook/mutating"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type config struct {
	certFile   string
	keyFile    string
	cpuRequest string
	memRequest string
}

//goland:noinspection SpellCheckingInspection
func httpHandlerHealthz(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Ok.")
}

func initFlags() config {
	cfg := config{
		cpuRequest: "10m",
		memRequest: "16Mi",
	}

	fl := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fl.StringVar(&cfg.certFile, "tls-cert-file", "", "TLS certificate file")
	fl.StringVar(&cfg.keyFile, "tls-key-file", "", "TLS key file")
	fl.Parse(os.Args[1:])
	return cfg
}

func (c *config) addInitContainerToPod(_ context.Context, _ *kwhmodel.AdmissionReview, obj metav1.Object) (*kwhmutating.MutatorResult, error) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		// If not a pod just continue the mutation chain(if there is one) and don't do nothing.
		return &kwhmutating.MutatorResult{}, nil
	}

	if pod.Spec.Subdomain == "" {
		// do nothing if there isn't spec.subdomain
		return &kwhmutating.MutatorResult{
			MutatedObject: pod,
		}, nil
	}

	volume := corev1.Volume{
		Name: "etc-hosts",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}

	volumeMountInit := corev1.VolumeMount{
		Name:      "etc-hosts",
		MountPath: "/mnt/",
		ReadOnly:  false,
	}

	volumeMount := corev1.VolumeMount{
		Name:      "etc-hosts",
		MountPath: "/etc/hosts",
		SubPath:   "hosts",
		ReadOnly:  true,
	}

	var podHostname string
	if pod.Spec.Hostname != "" {
		podHostname = pod.Spec.Hostname
	} else {
		podHostname = pod.Name
	}

	runAsUser := int64(65534)
	runAsGroup := int64(65534)
	runAsNonRoot := true
	readOnlyRootFileSystem := true
	allowPrivilegeEscalation := false
	seccompType := corev1.SeccompProfileTypeRuntimeDefault
	initContainer := corev1.Container{
		Name:         "render-etc-hosts-with-cluster-domain-aliases",
		Image:        os.Getenv("INIT_CONTAINER_IMAGE"),
		VolumeMounts: []corev1.VolumeMount{volumeMountInit},
		Command:      []string{"/render-etc-hosts-with-cluster-domain-aliases"},
		Env: []corev1.EnvVar{
			{Name: "POD_HOSTNAME", Value: podHostname},
			{Name: "POD_NAMESPACE", Value: pod.Namespace},
			{Name: "POD_SUBDOMAIN", Value: pod.Spec.Subdomain},
			{Name: "CLUSTER_DOMAIN", Value: os.Getenv("CLUSTER_DOMAIN")},
			{Name: "CLUSTER_DOMAIN_ALIASES", Value: os.Getenv("CLUSTER_DOMAIN_ALIASES")},
			{Name: "POD_IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "status.podIP"}}},
		},
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"all"},
			},
			RunAsUser:                &runAsUser,
			RunAsGroup:               &runAsGroup,
			RunAsNonRoot:             &runAsNonRoot,
			ReadOnlyRootFilesystem:   &readOnlyRootFileSystem,
			AllowPrivilegeEscalation: &allowPrivilegeEscalation,
			SeccompProfile:           &corev1.SeccompProfile{Type: seccompType},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(c.cpuRequest),
				corev1.ResourceMemory: resource.MustParse(c.memRequest),
			},
		},
	}

	pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
	pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: "deckhouse-registry-kube-dns"})

	for i := range pod.Spec.Containers {
		pod.Spec.Containers[i].VolumeMounts = append(pod.Spec.Containers[i].VolumeMounts, volumeMount)
	}

	for i := range pod.Spec.InitContainers {
		pod.Spec.InitContainers[i].VolumeMounts = append(pod.Spec.InitContainers[i].VolumeMounts, volumeMount)
	}

	// add to the very beginning of initContainers
	pod.Spec.InitContainers = append([]corev1.Container{initContainer}, pod.Spec.InitContainers...)

	return &kwhmutating.MutatorResult{
		MutatedObject: pod,
	}, nil
}

func main() {
	logrusLogEntry := logrus.NewEntry(logrus.New())
	logrusLogEntry.Logger.SetLevel(logrus.DebugLevel)
	logger := kwhlogrus.NewLogrus(logrusLogEntry)

	cfg := initFlags()

	mt := kwhmutating.MutatorFunc(cfg.addInitContainerToPod)

	mcfg := kwhmutating.WebhookConfig{
		ID:      "addHostAliasesToPod",
		Obj:     &corev1.Pod{},
		Mutator: mt,
		Logger:  logger,
	}
	wh, err := kwhmutating.NewWebhook(mcfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating webhook: %s", err)
		os.Exit(1)
	}

	// Get the handler for our webhook.
	whHandler, err := kwhhttp.HandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: logger})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating webhook handler: %s", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.Handle("/", whHandler)
	mux.HandleFunc("/healthz", httpHandlerHealthz)

	logger.Infof("Listening on :4443")
	err = http.ListenAndServeTLS(":4443", cfg.certFile, cfg.keyFile, mux)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error serving webhook: %s", err)
		os.Exit(1)
	}
}
