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

package hooks

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloudflare/cfssl/csr"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

const (
	proxyJobNS   = "d8-system"
	proxyJobName = "crowd-proxy-cert-generate-job"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/user-authn/generate_crowd_basic_auth_proxy_cert",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "generate_crowd_basic_auth_proxy_cert",
			Crontab: "42 4 * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "crowd-secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-user-authn"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"crowd-basic-auth-cert"},
			},
			FilterFunc: filterSecret,
		},
	},
}, dependency.WithExternalDependencies(generateProxyAuthCert))

func filterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec corev1.Secret

	err := sdk.FromUnstructured(obj, &sec)
	if err != nil {
		return nil, err
	}

	return secret{Crt: sec.Data["client.crt"], Key: sec.Data["client.key"]}, nil
}

type secret struct {
	Crt []byte
	Key []byte
}

type provider struct {
	Typ   string `json:"type"`
	Crowd struct {
		EnableBasicAuth bool `json:"enableBasicAuth"`
	} `json:"crowd"`
}

func generateProxyAuthCert(input *go_hook.HookInput, dc dependency.Container) error {
	// check proxy rollout conditions
	if !input.Values.Get("userAuthn.publishAPI.enabled").Bool() {
		return nil
	}

	if !input.Values.Exists("userAuthn.internal.providers") {
		return nil
	}

	providersJSON := input.Values.Get("userAuthn.internal.providers").String()

	var providers []provider

	err := json.Unmarshal([]byte(providersJSON), &providers)
	if err != nil {
		return err
	}

	var crowdConfig *provider

	for _, prov := range providers {
		if prov.Typ == "Crowd" && prov.Crowd.EnableBasicAuth {
			if crowdConfig != nil {
				return errors.New("only one enableBasicAuth must be enabled for Crowd")
			}
			crowdConfig = &prov
		}
	}

	if crowdConfig == nil {
		return nil
	}

	// check certificate renewal necessity
	snap := input.Snapshots["crowd-secret"]
	if len(snap) > 0 {
		secret := snap[0].(secret)

		// if cert if valid more then two days - skip renewal
		expiring, err := certificate.IsCertificateExpiringSoon(secret.Crt, 2*24*time.Hour)
		if err != nil {
			return err
		}

		if !expiring {
			input.Values.Set("userAuthn.internal.crowdProxyCert", base64.StdEncoding.EncodeToString(secret.Crt))
			input.Values.Set("userAuthn.internal.crowdProxyKey", base64.StdEncoding.EncodeToString(secret.Key))
			return nil
		}
	}

	// create CSR
	gcsr, pkey, err := certificate.GenerateCSR(input.LogEntry, "front-proxy-client", certificate.WithCSRKeyRequest(&csr.KeyRequest{A: "rsa", S: 2048}))
	if err != nil {
		return err
	}

	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	registry := input.Values.Get("global.modulesImages.registry.base").String()
	digest := input.Values.Get("global.modulesImages.digests.userAuthn.selfSignedGenerator").String()
	job := generateJob(registry, digest, base64.StdEncoding.EncodeToString(gcsr))

	foreground := v1.DeletePropagationForeground
	_ = kubeClient.BatchV1().Jobs(proxyJobNS).Delete(context.Background(), proxyJobName, v1.DeleteOptions{PropagationPolicy: &foreground})
	createdJob, err := kubeClient.BatchV1().Jobs(proxyJobNS).Create(context.Background(), job, v1.CreateOptions{})
	if err != nil {
		return err
	}

	// Postpone job deletion after hook execution.
	input.PatchCollector.Delete("batch/v1", "Job", proxyJobNS, proxyJobName)

	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") == "true" {
		createdJob.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
		_, _ = kubeClient.BatchV1().Jobs(proxyJobNS).UpdateStatus(context.Background(), createdJob, v1.UpdateOptions{})
	}

	if _, err = waitForJob(kubeClient); err != nil {
		return err
	}

	// get logs from completed pod
	pods, err := kubeClient.CoreV1().Pods(proxyJobNS).List(context.Background(), v1.ListOptions{LabelSelector: "job-name=crowd-proxy-cert-generate-job"})
	if err != nil {
		return err
	}
	if len(pods.Items) == 0 {
		return errors.New("no job pods found")
	}

	logReq := kubeClient.CoreV1().Pods(proxyJobNS).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{})
	stream, err := logReq.Stream(context.Background())
	if err != nil {
		return err
	}
	defer stream.Close()

	var certb64 string

	sc := bufio.NewScanner(stream)
	for sc.Scan() {
		if !strings.HasPrefix(sc.Text(), "Certificate: ") {
			// for integration tests - FakePod always returns "fake logs"
			if sc.Text() == "fake logs" {
				certb64 = testingCert
				break
			}
			continue
		}
		certb64 = strings.TrimPrefix(sc.Text(), "Certificate: ")
		break
	}

	if certb64 == "" {
		return errors.New("cert not generated")
	}

	input.Values.Set("userAuthn.internal.crowdProxyCert", certb64)
	input.Values.Set("userAuthn.internal.crowdProxyKey", base64.StdEncoding.EncodeToString(pkey))

	return nil
}

func waitForJob(kubeClient k8s.Client) (*batchv1.Job, error) {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(2 * time.Second)

	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") == "true" {
		ticker = time.NewTicker(1 * time.Nanosecond)
	}

	defer ticker.Stop()

	// Maybe we can replace it with watch.Until but it's not fully realized in a fake client.
	// We need to more something to make it works
	for {
		select {
		case <-ticker.C:
			job, err := kubeClient.BatchV1().Jobs(proxyJobNS).Get(context.Background(), proxyJobName, v1.GetOptions{})
			if err != nil {
				continue
			}
			for _, cond := range job.Status.Conditions {
				if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
					return job, nil
				}
			}

		case <-timeout:
			return nil, errors.New("job timeout")
		}
	}
}

func generateJob(registry, digest, csrb64 string) *batchv1.Job {
	return &batchv1.Job{
		TypeMeta: v1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      proxyJobName,
			Namespace: proxyJobNS,
			Labels: map[string]string{
				"name":     "crowd-proxy-cert-generate-job",
				"heritage": "deckhouse",
				"module":   "user-authn",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: pointer.Int32(1),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{{Name: "deckhouse-registry"}},
					Containers: []corev1.Container{
						{
							Name:  "generator",
							Image: fmt.Sprintf("%s@%s", registry, digest),
							Args:  []string{"generate-crowd-proxy-certs"},
							Env: []corev1.EnvVar{
								{
									Name:  "CSR",
									Value: csrb64,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "etc",
									ReadOnly:  true,
									MountPath: "/etc",
								},
								{
									Name:      "var",
									ReadOnly:  true,
									MountPath: "/var",
								},
								{
									Name:      "mnt",
									ReadOnly:  true,
									MountPath: "/mnt",
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{
						{
							Name: "etc",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/etc",
								},
							},
						},
						{
							Name: "mnt",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/mnt",
								},
							},
						},
						{
							Name: "var",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var",
								},
							},
						},
					},
					HostPID: true,
					NodeSelector: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
					},
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
				},
			},
		},
	}
}

const (
	testingCert = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUIwRENDQVRHZ0F3SUJBZ0lCQVRBS0JnZ3Foa2pPUFFRREJEQVNNUkF3RGdZRFZRUUtFd2RCWTIxbElFTnYKTUI0WERUQTVNVEV4TURJek1EQXdNRm9YRFRFd01EVXdPVEl6TURBd01Gb3dFakVRTUE0R0ExVUVDaE1IUVdOdApaU0JEYnpDQm16QVFCZ2NxaGtqT1BRSUJCZ1VyZ1FRQUl3T0JoZ0FFQVpaWWN3dlJTdVp0SkxtS25PTUM2NUh3Ck1OQ01YZU5EbWUybFBDUnJta2FQazBJUEpVU1Rta0JndmRxeDR1cG1NVTZ0VGtZSlUyTjNsZnVxTUU4RGh0dHQKQUM1eHNxeWcrMEdCaXZhZTM0NXlzVzNnMFMyWlJzTjA5M0IvampxTUpMbjNROEwyUDAydHJZaEd0Wm8yeG0wMgp3UWVWL0J2SjhKWUlla3FDU1h2Q1Q2cnhvelV3TXpBT0JnTlZIUThCQWY4RUJBTUNCYUF3RXdZRFZSMGxCQXd3CkNnWUlLd1lCQlFVSEF3RXdEQVlEVlIwVEFRSC9CQUl3QURBS0JnZ3Foa2pPUFFRREJBT0JqQUF3Z1lnQ1FnRzcKTlRtQ0JMQXl1bXhQY2NtY2dodmYxK2NiOXh6TVYwd3RBUVNlL2RxaURUWk81QmZ3blVGWjBUY0NZSXNCak1uUwoxbnNNamFnckQvUDdJc2UrVmZaQkhRSkNBZmdkT0JJMzRuQXcrampGWTR6SUMxOVoraHFzMjZaZ1UvREcwQVJ1CnNNVXFWMjJBb1dWUEQ0S2tEUklZR0lxK3h0WGJyTnR3TW9rQ0lNTExobWZFK3ZPVAotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg=="
)
