// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

// Air-gap (NeedsSeed) bootstrap cache: a hostNetwork cache Pod on the first
// master (127.0.0.1:5001), filled from the bundle (over the reverse tunnel at
// 127.0.0.1:5511) by a one-shot syncer Pod, before Deckhouse. Torn down at
// finalize once the module cache DaemonSet takes over the shared hostPath.
// hostNetwork: no CNI before Deckhouse, and it binds the node loopback (reachable,
// not exposed).

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/initsecret"
	"github.com/deckhouse/deckhouse/go_lib/registry/pki"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

const (
	// Keep in sync with modules/038-registry/templates/cache/daemonset.yaml: the
	// module cache DaemonSet mounts the same dir, so its leader comes up pre-filled.
	bootstrapCacheHostPath = "/var/lib/deckhouse/registry-cache"

	bootstrapCacheLocalAddr = "127.0.0.1:5001" // cache (mirror #0 in BootstrapAirGapHostsLocal)
	bundleTunnelLocalAddr   = "127.0.0.1:5511" // bundle over the reverse tunnel (fill source)

	bootstrapCachePodName     = "registry-bootstrap-cache"
	bootstrapCacheFillPodName = "registry-bootstrap-cache-fill"
	bootstrapCacheFillSecret  = "registry-bootstrap-cache-fill"

	bootstrapCachePKISecret    = "registry-bootstrap-cache-pki"
	bootstrapCacheConfigSecret = "registry-bootstrap-cache-config"

	bootstrapCacheReadyAttempts = 60
	bootstrapCacheReadyWait     = 5 * time.Second

	fillPollAttempts = 180 // ×10s = 30 min budget for the bundle copy
	fillPollWait     = 10 * time.Second
)

// htpasswd auth (not docker-auth token): the stored blobs are auth-agnostic and
// inherited by the module cache via the shared hostPath, so the bootstrap auth need
// not match the module's. Both bootstrap users get full access (loopback, transient).
const bootstrapCacheDistributionConfig = `version: 0.1
log:
  level: info
storage:
  filesystem:
    rootdirectory: /data
  delete:
    enabled: true
  redirect:
    disable: true
http:
  addr: "127.0.0.1:5001"
  prefix: /
  secret: asecretforbootstrap
  debug:
    addr: "127.0.0.1:5002"
    prometheus:
      enabled: true
      path: /metrics
  tls:
    certificate: /pki/distribution.crt
    key: /pki/distribution.key
auth:
  htpasswd:
    realm: registry-bootstrap-cache
    path: /auth/htpasswd
`

// SyncerConfig mirrors the syncer image's config schema (a separate, un-vendored
// Go module, so duplicated here).
type SyncerConfig struct {
	Src   SyncerRegistry `json:"source"`
	Dest  SyncerRegistry `json:"destination"`
	Prune bool           `json:"prune,omitempty"`
}

type SyncerRegistry struct {
	Address string      `json:"address"`
	User    *SyncerUser `json:"user,omitempty"`
	CA      string      `json:"ca,omitempty"`
}

type SyncerUser struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

// SetupBootstrapCache brings up and fills the cache. distributionImage / syncerImage
// are fully-qualified refs (imagesBase@digest) for the cache Pod and fill Pod.
func SetupBootstrapCache(ctx context.Context, kubeCl *client.KubernetesClient, distributionImage, syncerImage string) error {
	cfg, err := readInitConfig(ctx, kubeCl)
	if err != nil {
		return fmt.Errorf("read registry-init: %w", err)
	}

	if err := bringUpBootstrapCache(ctx, kubeCl, cfg, distributionImage); err != nil {
		return fmt.Errorf("bring up bootstrap cache: %w", err)
	}

	if err := waitBootstrapCacheReady(ctx, kubeCl); err != nil {
		return fmt.Errorf("wait bootstrap cache ready: %w", err)
	}

	if err := fillBootstrapCache(ctx, kubeCl, cfg, syncerImage); err != nil {
		return fmt.Errorf("fill bootstrap cache: %w", err)
	}
	return nil
}

// readInitConfig reads the registry-init secret (CA + RO/RW users) created by
// bashible step 073.
func readInitConfig(ctx context.Context, kubeCl *client.KubernetesClient) (initsecret.Config, error) {
	secret, err := kubeCl.CoreV1().Secrets(bootstrapCacheNamespace).Get(ctx, "registry-init", metav1.GetOptions{})
	if err != nil {
		return initsecret.Config{}, fmt.Errorf("get secret registry-init: %w", err)
	}

	var cfg initsecret.Config
	if err := yaml.Unmarshal(secret.Data["config"], &cfg); err != nil {
		return initsecret.Config{}, fmt.Errorf("parse registry-init config: %w", err)
	}

	return cfg, nil
}

func bringUpBootstrapCache(ctx context.Context, kubeCl *client.KubernetesClient, cfg initsecret.Config, distributionImage string) error {
	ca, err := pki.DecodeCertKey([]byte(cfg.CA.Cert), []byte(cfg.CA.Key))
	if err != nil {
		return fmt.Errorf("decode init CA: %w", err)
	}

	dist, err := pki.GenerateCertificate("registry-distribution", ca, "127.0.0.1", "localhost", "registry.d8-system.svc")
	if err != nil {
		return fmt.Errorf("generate distribution cert: %w", err)
	}

	distCertPEM, distKeyPEM, err := pki.EncodeCertKey(dist)
	if err != nil {
		return fmt.Errorf("encode distribution cert: %w", err)
	}

	htpasswd := fmt.Sprintf("%s:%s\n%s:%s\n",
		cfg.ROUser.Name, cfg.ROUser.PasswordHash,
		cfg.RWUser.Name, cfg.RWUser.PasswordHash)

	pkiSecret := &corev1.Secret{
		ObjectMeta: bootstrapCacheObjectMeta(bootstrapCachePKISecret),
		Type:       corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"ca.crt":           []byte(cfg.CA.Cert),
			"distribution.crt": distCertPEM,
			"distribution.key": distKeyPEM,
		},
	}
	if err := applySecret(ctx, kubeCl, pkiSecret); err != nil {
		return err
	}

	configSecret := &corev1.Secret{
		ObjectMeta: bootstrapCacheObjectMeta(bootstrapCacheConfigSecret),
		Type:       corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"config.yaml": []byte(bootstrapCacheDistributionConfig),
			"htpasswd":    []byte(htpasswd),
		},
	}
	if err := applySecret(ctx, kubeCl, configSecret); err != nil {
		return err
	}

	return applyPod(ctx, kubeCl, bootstrapCachePodSpec(distributionImage))
}

func bootstrapCacheObjectMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: bootstrapCacheNamespace,
		Labels:    map[string]string{"app": bootstrapCachePodName},
	}
}

// bootstrapCachePodSpec builds the cache Pod: a bare Pod (not a Deployment) — a
// one-shot shim with no controller and a direct delete at finalize.
func bootstrapCachePodSpec(distributionImage string) *corev1.Pod {
	hostPathType := corev1.HostPathDirectoryOrCreate
	runAsUser := int64(0)
	grace := int64(5)

	return &corev1.Pod{
		ObjectMeta: bootstrapCacheObjectMeta(bootstrapCachePodName),
		Spec: corev1.PodSpec{
			HostNetwork:                   true,
			DNSPolicy:                     corev1.DNSClusterFirstWithHostNet,
			NodeSelector:                  map[string]string{"node-role.kubernetes.io/control-plane": ""},
			Tolerations:                   []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
			SecurityContext:               &corev1.PodSecurityContext{RunAsUser: &runAsUser},
			RestartPolicy:                 corev1.RestartPolicyAlways,
			TerminationGracePeriodSeconds: &grace,
			Containers: []corev1.Container{{
				Name:  "distribution",
				Image: distributionImage,
				Args:  []string{"serve", "/config/config.yaml"},
				Ports: []corev1.ContainerPort{{Name: "distribution", ContainerPort: 5001}},
				// tcpSocket, not httpGet: distribution is HTTPS+auth, a TCP check = listening.
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(5001)},
					},
					PeriodSeconds: 3,
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "data", MountPath: "/data"},
					{Name: "config", MountPath: "/config"},
					{Name: "pki", MountPath: "/pki"},
					{Name: "htpasswd", MountPath: "/auth"},
				},
			}},
			Volumes: []corev1.Volume{
				{Name: "data", VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{Path: bootstrapCacheHostPath, Type: &hostPathType},
				}},
				{Name: "config", VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: bootstrapCacheConfigSecret,
						Items:      []corev1.KeyToPath{{Key: "config.yaml", Path: "config.yaml"}},
					},
				}},
				{Name: "htpasswd", VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: bootstrapCacheConfigSecret,
						Items:      []corev1.KeyToPath{{Key: "htpasswd", Path: "htpasswd"}},
					},
				}},
				{Name: "pki", VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: bootstrapCachePKISecret},
				}},
			},
		},
	}
}

// applySecret creates the secret, replacing any pre-existing one.
func applySecret(ctx context.Context, kubeCl *client.KubernetesClient, s *corev1.Secret) error {
	_ = kubeCl.CoreV1().Secrets(s.Namespace).Delete(ctx, s.Name, metav1.DeleteOptions{})
	if _, err := kubeCl.CoreV1().Secrets(s.Namespace).Create(ctx, s, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create secret %s: %w", s.Name, err)
	}

	return nil
}

// applyPod creates the Pod, replacing any pre-existing one.
func applyPod(ctx context.Context, kubeCl *client.KubernetesClient, p *corev1.Pod) error {
	_ = kubeCl.CoreV1().Pods(p.Namespace).Delete(ctx, p.Name, metav1.DeleteOptions{})
	if _, err := kubeCl.CoreV1().Pods(p.Namespace).Create(ctx, p, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create pod %s: %w", p.Name, err)
	}

	return nil
}

func waitBootstrapCacheReady(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Waiting for registry-bootstrap-cache to become Ready", bootstrapCacheReadyAttempts, bootstrapCacheReadyWait).
		RunContext(ctx, func() error {
			p, err := kubeCl.CoreV1().Pods(bootstrapCacheNamespace).Get(ctx, bootstrapCachePodName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("get pod %s: %w", bootstrapCachePodName, err)
			}
			for _, c := range p.Status.Conditions {
				if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
					return nil
				}
			}
			return fmt.Errorf("registry-bootstrap-cache not ready (phase=%s)", p.Status.Phase)
		})
}

// fillBootstrapCache copies the bundle into the cache via a one-shot hostNetwork
// syncer Pod (bundle 127.0.0.1:5511 -> cache 127.0.0.1:5001), then removes it.
func fillBootstrapCache(ctx context.Context, kubeCl *client.KubernetesClient, cfg initsecret.Config, syncerImage string) error {
	syncerCfg := SyncerConfig{
		Src: SyncerRegistry{Address: bundleTunnelLocalAddr},
		Dest: SyncerRegistry{
			Address: bootstrapCacheLocalAddr,
			CA:      cfg.CA.Cert,
			User:    &SyncerUser{Name: cfg.RWUser.Name, Password: cfg.RWUser.Password},
		},
		Prune: false,
	}
	cfgBytes, err := yaml.Marshal(syncerCfg)
	if err != nil {
		return fmt.Errorf("marshal syncer config: %w", err)
	}

	fillSecret := &corev1.Secret{
		ObjectMeta: bootstrapCacheObjectMeta(bootstrapCacheFillSecret),
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"config.yaml": cfgBytes},
	}
	if err := applySecret(ctx, kubeCl, fillSecret); err != nil {
		return err
	}
	if err := applyPod(ctx, kubeCl, fillPodSpec(syncerImage)); err != nil {
		return err
	}

	if err := waitFillPodSucceeded(ctx, kubeCl); err != nil {
		return err
	}

	// Best-effort: finalize's DeleteBootstrapCache also covers these.
	_ = kubeCl.CoreV1().Pods(bootstrapCacheNamespace).Delete(ctx, bootstrapCacheFillPodName, metav1.DeleteOptions{})
	_ = kubeCl.CoreV1().Secrets(bootstrapCacheNamespace).Delete(ctx, bootstrapCacheFillSecret, metav1.DeleteOptions{})
	return nil
}

// fillPodSpec builds the one-shot syncer Pod (image entrypoint is /syncer; arg is
// the config path; exits 0 on success).
func fillPodSpec(syncerImage string) *corev1.Pod {
	runAsUser := int64(0)

	return &corev1.Pod{
		ObjectMeta: bootstrapCacheObjectMeta(bootstrapCacheFillPodName),
		Spec: corev1.PodSpec{
			HostNetwork:     true,
			DNSPolicy:       corev1.DNSClusterFirstWithHostNet,
			NodeSelector:    map[string]string{"node-role.kubernetes.io/control-plane": ""},
			Tolerations:     []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
			SecurityContext: &corev1.PodSecurityContext{RunAsUser: &runAsUser},
			RestartPolicy:   corev1.RestartPolicyOnFailure,
			Containers: []corev1.Container{{
				Name:  "syncer",
				Image: syncerImage,
				Args:  []string{"/config/config.yaml"},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "config", MountPath: "/config"},
				},
			}},
			Volumes: []corev1.Volume{
				{Name: "config", VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: bootstrapCacheFillSecret,
						Items:      []corev1.KeyToPath{{Key: "config.yaml", Path: "config.yaml"}},
					},
				}},
			},
		},
	}
}

// waitFillPodSucceeded polls the fill Pod until Succeeded. RestartPolicy OnFailure
// retries a transient syncer error in place; a persistent one exhausts the budget.
func waitFillPodSucceeded(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Waiting for registry-bootstrap-cache fill to complete", fillPollAttempts, fillPollWait).
		RunContext(ctx, func() error {
			p, err := kubeCl.CoreV1().Pods(bootstrapCacheNamespace).Get(ctx, bootstrapCacheFillPodName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("get pod %s: %w", bootstrapCacheFillPodName, err)
			}
			if p.Status.Phase == corev1.PodSucceeded {
				return nil
			}
			return fmt.Errorf("registry-bootstrap-cache fill not done (phase=%s)", p.Status.Phase)
		})
}
