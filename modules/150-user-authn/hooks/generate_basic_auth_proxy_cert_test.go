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
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"maps"
	"math/big"
	"slices"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/robfig/cron.v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/helm"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// stubProxyCertJobRunner stands for the generator Job, which the fake cluster cannot run.
type stubProxyCertJobRunner struct {
	crt   []byte
	err   error
	calls int
}

func (s *stubProxyCertJobRunner) run(_ context.Context, _ *batchv1.Job) (string, []byte, error) {
	s.calls++
	if s.err != nil {
		return "", nil, s.err
	}
	return proxyJobName + "-stub", s.crt, nil
}

// newTestProxyCert returns a PEM certificate and key that expire at notAfter.
func newTestProxyCert(notAfter time.Time) ([]byte, []byte) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "front-proxy-client"},
		NotBefore:    notAfter.Add(-72 * time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		panic(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

func basicAuthCertSecret(crt, key []byte) string {
	return fmt.Sprintf(`
apiVersion: v1
kind: Secret
type: Opaque
data:
  client.crt: %s
  client.key: %s
metadata:
  name: basic-auth-cert
  namespace: d8-user-authn
`, base64.StdEncoding.EncodeToString(crt), base64.StdEncoding.EncodeToString(key))
}

var _ = Describe("User Authn hooks :: generate basic auth proxy ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {"publishAPI": {"enabled": true}, "providers": [{
  "type": "Crowd",
  "displayName": "Crowd",
  "crowd": {
    "baseURL": "https://crowd.example.com/crowd",
    "clientID": "plainstring",
    "clientSecret": "plainstring",
    "enableBasicAuth": true,
    "groups": [
      "administrators",
      "users"
    ]
  }
}]}}}`, "")

	expireOp := operation.MetricOperation{
		Group:  proxyCertRenewalMetricGroup,
		Action: operation.ActionExpireMetrics,
	}
	failedOp := func(state string) operation.MetricOperation {
		return operation.MetricOperation{
			Name:   proxyCertRenewalFailedMetric,
			Value:  ptr.To(1.0),
			Labels: map[string]string{"state": state},
			Action: operation.ActionGaugeSet,
			Group:  proxyCertRenewalMetricGroup,
		}
	}

	issuedCrt, _ := newTestProxyCert(time.Now().Add(72 * time.Hour))
	jobErr := errors.New(`job d8-system/proxy-cert-generate-job-stub did not complete in 30s; pod proxy-cert-generate-job-stub-x: phase Failed, container generator terminated with exit code 128: StartError: unable to init seccomp: errno 524`)

	var (
		runner        *stubProxyCertJobRunner
		restoreRunner func(kubernetes.Interface) proxyCertJobRunner
		now           time.Time
	)

	// later moves the clock of the hook past the pause after a failed issue.
	later := func() { now = now.Add(proxyCertRetryInterval + time.Minute) }

	BeforeEach(func() {
		runner = &stubProxyCertJobRunner{crt: issuedCrt}
		restoreRunner = newProxyCertJobRunner
		newProxyCertJobRunner = func(kubernetes.Interface) proxyCertJobRunner { return runner }
		now = time.Now()
		proxyCertNow = func() time.Time { return now }
		proxyCertRetries = &proxyCertRetryState{}
		f.ValuesSet("global.modulesImages", GetModulesImages())
	})

	AfterEach(func() {
		newProxyCertJobRunner = restoreRunner
		proxyCertNow = time.Now
		proxyCertRetries = &proxyCertRetryState{}
	})

	expectValues := func(crt, key []byte) {
		Expect(f.ValuesGet("userAuthn.internal.basicAuthProxyCert").String()).To(Equal(base64.StdEncoding.EncodeToString(crt)))
		if key != nil {
			Expect(f.ValuesGet("userAuthn.internal.basicAuthProxyKey").String()).To(Equal(base64.StdEncoding.EncodeToString(key)))
		}
	}

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Certificate should be generated", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(1))
			expectValues(issuedCrt, nil)
			Expect(f.ValuesGet("userAuthn.internal.basicAuthProxyKey").String()).NotTo(BeEmpty())
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(expireOp))
		})

		It("Should generate job with a valid image", func() {
			registry := f.ValuesGet("global.modulesImages.registry.base").String()
			digest := f.ValuesGet("global.modulesImages.digests.userAuthn.cfssl").String()
			job := generateJob(registry, digest, "dGVzdAo=")
			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Containers[0].Image).To(ContainSubstring("@"))
			Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal(registry + "@" + digest))
		})

		It("Should generate job that needs only the exception for root and the front-proxy CA files", func() {
			job := generateJob("registry.example.com", "sha256:test", "dGVzdAo=")
			podSpec := job.Spec.Template.Spec

			Expect(job.Labels).To(HaveKeyWithValue("name", proxyJobName))
			Expect(*job.Spec.TTLSecondsAfterFinished).To(BeNumerically(">", 0))
			Expect(job.Spec.Template.Labels).To(HaveKeyWithValue(securityPolicyExceptionLabel, proxyJobSecurityPolicyExceptionName))
			Expect(podSpec.HostPID).To(BeFalse())
			Expect(*podSpec.SecurityContext.RunAsUser).To(BeEquivalentTo(0))
			Expect(*podSpec.SecurityContext.RunAsNonRoot).To(BeFalse())
			Expect(*podSpec.AutomountServiceAccountToken).To(BeFalse())
			Expect(podSpec.SecurityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))

			// Only the front-proxy CA files, not the whole /etc/kubernetes/pki with the cluster CA key.
			frontProxyCAFiles := []string{"/etc/kubernetes/pki/front-proxy-ca.crt", "/etc/kubernetes/pki/front-proxy-ca.key"}
			hostPaths := map[string]string{}
			for _, volume := range podSpec.Volumes {
				Expect(volume.HostPath).ToNot(BeNil(), "volume %s", volume.Name)
				Expect(*volume.HostPath.Type).To(Equal(corev1.HostPathFile), "volume %s", volume.Name)
				hostPaths[volume.Name] = volume.HostPath.Path
			}
			Expect(hostPaths).To(HaveLen(2))
			Expect(slices.Collect(maps.Values(hostPaths))).To(ConsistOf(frontProxyCAFiles))

			container := podSpec.Containers[0]
			Expect(container.VolumeMounts).To(HaveLen(2))
			for _, mount := range container.VolumeMounts {
				// The generator reads the front-proxy CA from fixed paths, the same as on the host.
				Expect(mount.MountPath).To(Equal(hostPaths[mount.Name]))
				// The exception matches a hostPath only when its readOnly equals the mount's.
				Expect(mount.ReadOnly).To(BeTrue())
			}
			Expect(*container.SecurityContext.AllowPrivilegeEscalation).To(BeFalse())
			Expect(*container.SecurityContext.ReadOnlyRootFilesystem).To(BeTrue())
			Expect(container.SecurityContext.Capabilities.Drop).To(ConsistOf(corev1.Capability("ALL")))
		})
	})

	Context("Fresh cluster, the Job fails", func() {
		BeforeEach(func() {
			runner.err = jobErr
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Does not stop the module, leaves the certificate empty and reports it missing", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(1))
			// The templates leave basic-auth-proxy out without a certificate.
			Expect(f.ValuesGet("userAuthn.internal.basicAuthProxyCert").String()).To(BeEmpty())
			Expect(f.ValuesGet("userAuthn.internal.basicAuthProxyKey").String()).To(BeEmpty())
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(expireOp, failedOp("missing")))
		})

		It("Retries a missing certificate after five minutes without retrying on every module run", func() {
			now = now.Add(5*time.Minute - time.Second)
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(1))
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics[len(metrics)-1]).To(Equal(failedOp("missing")))

			now = now.Add(time.Second)
			runner.err = nil
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(2))
			expectValues(issuedCrt, nil)

			// The newly issued certificate in values is fresh before Helm updates the Secret.
			now = now.Add(5 * time.Minute)
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(2))
			metrics = f.MetricsCollector.CollectedMetrics()
			Expect(metrics[len(metrics)-1]).To(Equal(expireOp))
		})

		It("Issues the certificate on a later run and clears the failure", func() {
			runner.err = nil
			later()
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			expectValues(issuedCrt, nil)
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics[len(metrics)-1]).To(Equal(expireOp))
		})
	})

	Context("Fresh cluster with LDAP provider", func() {
		g := HookExecutionConfigInit(`{"userAuthn":{"internal": {"publishAPI": {"enabled": true}, "providers": [{
	"type": "LDAP",
	"displayName": "LDAP",
	"ldap": {
		"enableBasicAuth": true
	}
	}]}}}`, "")

		BeforeEach(func() {
			g.ValuesSet("global.modulesImages", GetModulesImages())
			g.KubeStateSet(``)
			g.BindingContexts.Set(g.GenerateBeforeHelmContext())
			g.RunHook()
		})

		It("Certificate should be generated for LDAP provider", func() {
			Expect(g).To(ExecuteSuccessfully())
			Expect(g.ValuesGet("userAuthn.internal.basicAuthProxyCert").String()).To(Equal(base64.StdEncoding.EncodeToString(issuedCrt)))
		})
	})

	Context("Multiple providers with enableBasicAuth=true", func() {
		h := HookExecutionConfigInit(`{"userAuthn":{"internal": {"publishAPI": {"enabled": true}, "providers":[
		{"type":"LDAP","displayName":"LDAP","ldap":{"enableBasicAuth":true}},
		{"type":"Crowd","displayName":"Crowd","crowd":{"enableBasicAuth":true}}
	]}}}`, "")

		BeforeEach(func() {
			h.ValuesSet("global.modulesImages", GetModulesImages())
			h.KubeStateSet(``)
			h.BindingContexts.Set(h.GenerateBeforeHelmContext())
			h.RunHook()
		})

		It("Hook should fail when multiple providers have enableBasicAuth=true", func() {
			Expect(h.GoHookError).To(HaveOccurred())
			Expect(h.GoHookError.Error()).To(ContainSubstring("multiple 'enableBasicAuth: true' options are enabled while only one is supported, review your authentication configuration"))
		})
	})

	Context("Cluster with a valid cert", func() {
		crt, key := newTestProxyCert(time.Now().Add(60 * time.Hour))

		BeforeEach(func() {
			f.KubeStateSet(basicAuthCertSecret(crt, key))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should keep previous certificate and key without a Job", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(BeZero())
			expectValues(crt, key)
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(expireOp))
		})
	})

	Context("Cluster with an expiring cert", func() {
		crt, key := newTestProxyCert(time.Now().Add(24 * time.Hour))

		BeforeEach(func() {
			f.KubeStateSet(basicAuthCertSecret(crt, key))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
		})

		It("Renews the certificate", func() {
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(1))
			expectValues(issuedCrt, nil)
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(expireOp))
		})

		It("Keeps the current certificate and reports the failure when the Job fails", func() {
			runner.err = jobErr
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			expectValues(crt, key)
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(expireOp, failedOp("expiring")))
		})

		It("Clears the failure after a later successful renewal", func() {
			runner.err = jobErr
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())

			// A module run within the pause keeps the certificate and the failure.
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(1))
			expectValues(crt, key)
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics[len(metrics)-1]).To(Equal(failedOp("expiring")))

			runner.err = nil
			later()
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			expectValues(issuedCrt, nil)
			metrics = f.MetricsCollector.CollectedMetrics()
			Expect(metrics[len(metrics)-1]).To(Equal(expireOp))
		})

		It("Keeps the fifty-minute pause while the certificate remains valid", func() {
			runner.err = jobErr
			f.RunHook()
			now = now.Add(49 * time.Minute)
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(1))
			now = now.Add(time.Minute)
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(2))
		})

		It("Shortens an existing pause when the certificate expires", func() {
			// Start the failed renewal shortly before expiry, then cross NotAfter.
			block, _ := pem.Decode(crt)
			parsed, err := x509.ParseCertificate(block.Bytes)
			Expect(err).NotTo(HaveOccurred())
			now = parsed.NotAfter.Add(-time.Minute)
			runner.err = jobErr
			f.RunHook()
			now = now.Add(5 * time.Minute)
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(2))
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics[len(metrics)-1]).To(Equal(failedOp("expired")))
		})
	})

	Context("Cluster with an expired cert, the Job fails", func() {
		crt, key := newTestProxyCert(time.Now().Add(-time.Hour))

		BeforeEach(func() {
			runner.err = jobErr
			f.KubeStateSet(basicAuthCertSecret(crt, key))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Keeps the expired certificate so the rest of the module renders, and reports the failure", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(1))
			expectValues(crt, key)
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(expireOp, failedOp("expired")))
		})

		It("Retries an expired certificate after five minutes", func() {
			now = now.Add(5*time.Minute - time.Second)
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(1))
			now = now.Add(time.Second)
			runner.err = nil
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(2))
			expectValues(issuedCrt, nil)
		})
	})

	Context("Cluster with a cert that does not parse", func() {
		BeforeEach(func() {
			f.KubeStateSet(basicAuthCertSecret([]byte("not a certificate"), []byte("not a key")))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Issues a new certificate", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(1))
			expectValues(issuedCrt, nil)
		})
	})

	Context("Basic authentication is turned off after a failed issue", func() {
		BeforeEach(func() {
			runner.err = jobErr
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(1))
		})

		It("Issues at once when the API is published again", func() {
			f.ValuesSet("userAuthn.internal.publishAPI.enabled", false)
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(1))

			f.ValuesSet("userAuthn.internal.publishAPI.enabled", true)
			runner.err = nil
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(2))
			expectValues(issuedCrt, nil)
		})

		It("Issues at once when a provider enables basic authentication again", func() {
			providers := f.ValuesGet("userAuthn.internal.providers").String()
			f.ValuesSetFromYaml("userAuthn.internal.providers", []byte(`[{"type": "Crowd", "crowd": {"enableBasicAuth": false}}]`))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(1))

			f.ValuesSetFromYaml("userAuthn.internal.providers", []byte(providers))
			runner.err = nil
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(Equal(2))
			expectValues(issuedCrt, nil)
		})
	})

	Context("Values hold a newer cert than the Secret", func() {
		oldCrt, oldKey := newTestProxyCert(time.Now().Add(24 * time.Hour))
		newCrt, newKey := newTestProxyCert(time.Now().Add(71 * time.Hour))

		BeforeEach(func() {
			runner.err = jobErr
			f.ValuesSet("userAuthn.internal.basicAuthProxyCert", base64.StdEncoding.EncodeToString(newCrt))
			f.ValuesSet("userAuthn.internal.basicAuthProxyKey", base64.StdEncoding.EncodeToString(newKey))
			f.KubeStateSet(basicAuthCertSecret(oldCrt, oldKey))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Keeps the newer certificate from the values without a Job", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(runner.calls).To(BeZero())
			expectValues(newCrt, newKey)
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(expireOp))
		})
	})
})

const testJobName = proxyJobName + "-test"

func TestProxyCertScheduledRetry(t *testing.T) {
	schedule, err := cron.Parse(proxyCertRetryCrontab)
	if err != nil {
		t.Fatal(err)
	}
	failedAt := time.Date(2026, 11, 18, 12, 53, 0, 0, time.UTC)
	restore := proxyCertNow
	t.Cleanup(func() { proxyCertNow = restore })
	for _, tt := range []struct {
		name    string
		current *proxyCert
		want    time.Duration
	}{
		{name: "missing", want: 7 * time.Minute},
		{name: "expired", current: &proxyCert{NotAfter: failedAt}, want: 7 * time.Minute},
		{name: "expiring", current: &proxyCert{NotAfter: failedAt.Add(time.Hour)}, want: 52 * time.Minute},
		{name: "expires during pause", current: &proxyCert{NotAfter: failedAt.Add(3 * time.Minute)}, want: 7 * time.Minute},
		{name: "expires after urgent pause", current: &proxyCert{NotAfter: failedAt.Add(20 * time.Minute)}, want: 22 * time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state := &proxyCertRetryState{failedAt: failedAt}
			for next := schedule.Next(failedAt); ; next = schedule.Next(next) {
				proxyCertNow = func() time.Time { return next }
				if !state.wait(tt.current) {
					if got := next.Sub(failedAt); got != tt.want {
						t.Errorf("first scheduled retry after failure at :53 = %v, want %v", got, tt.want)
					}
					break
				}
				if next.Sub(failedAt) > 2*time.Hour {
					t.Fatal("no scheduled retry within two hours")
				}
			}
		})
	}
}

// The Job must not outlive the wait of the hook by much, and a parallel run must not delete it while
// it can still run.
func TestGenerateJobDeadline(t *testing.T) {
	job := generateJob("registry.example.com", "sha256:test", "")
	if job.Spec.ActiveDeadlineSeconds == nil {
		t.Fatal("the Job has no activeDeadlineSeconds")
	}
	deadline := time.Duration(*job.Spec.ActiveDeadlineSeconds) * time.Second
	if deadline <= proxyCertJobTimeout || deadline >= 2*proxyCertJobTimeout {
		t.Errorf("activeDeadlineSeconds = %v, want more than the wait %v and less than the keep window %v",
			deadline, proxyCertJobTimeout, 2*proxyCertJobTimeout)
	}
}

func newTestProxyCertJob(t *testing.T, logs string, objects ...runtime.Object) (*proxyCertJob, *fake.Clientset) {
	t.Helper()

	client := fake.NewClientset(objects...)
	return &proxyCertJob{
		client:       client,
		pollInterval: time.Millisecond,
		timeout:      50 * time.Millisecond,
		nameSuffix:   func() string { return "test" },
		podLogs: func(_ context.Context, pod string) (io.ReadCloser, error) {
			if logs == "" {
				return nil, fmt.Errorf("no log for pod %s", pod)
			}
			return io.NopCloser(strings.NewReader(logs)), nil
		},
	}, client
}

func testJob(conditions ...batchv1.JobCondition) *batchv1.Job {
	job := generateJob("registry.example.com", "sha256:test", "dGVzdAo=")
	job.Status.Conditions = conditions
	return job
}

func testJobPod(name string, created time.Time, phase corev1.PodPhase, state corev1.ContainerState) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         proxyJobNS,
			Labels:            map[string]string{"job-name": testJobName},
			CreationTimestamp: metav1.NewTime(created),
		},
		Status: corev1.PodStatus{
			Phase:             phase,
			ContainerStatuses: []corev1.ContainerStatus{{Name: proxyJobContainer, State: state}},
		},
	}
}

func testWarningEvent(name, kind, object, reason, message string, at time.Time) *corev1.Event {
	return &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: name, Namespace: proxyJobNS},
		InvolvedObject: corev1.ObjectReference{Kind: kind, Name: object, Namespace: proxyJobNS},
		Type:           corev1.EventTypeWarning,
		Reason:         reason,
		Message:        message,
		LastTimestamp:  metav1.NewTime(at),
	}
}

var jobComplete = batchv1.JobCondition{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}

func TestProxyCertJobReadsCertFromSucceededPod(t *testing.T) {
	crt, _ := newTestProxyCert(time.Now().Add(72 * time.Hour))
	now := time.Now()
	runner, _ := newTestProxyCertJob(t, "Certificate: "+base64.StdEncoding.EncodeToString(crt),
		// The failed first attempt is listed before the pod that succeeded.
		testJobPod(testJobName+"-a", now, corev1.PodFailed, corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}}),
		testJobPod(testJobName+"-b", now.Add(time.Second), corev1.PodSucceeded, corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{}}),
	)
	var readFrom string
	podLogs := runner.podLogs
	runner.podLogs = func(ctx context.Context, pod string) (io.ReadCloser, error) {
		readFrom = pod
		return podLogs(ctx, pod)
	}

	name, got, err := runner.run(context.Background(), testJob(jobComplete))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if name != testJobName {
		t.Errorf("job name = %q, want %q", name, testJobName)
	}
	if string(got) != string(crt) {
		t.Errorf("certificate = %q, want %q", got, crt)
	}
	if readFrom != testJobName+"-b" {
		t.Errorf("log read from pod %q, want the succeeded one", readFrom)
	}
}

func TestProxyCertJobDeletesPreviousJobsBeforeCreate(t *testing.T) {
	legacy := generateJob("registry.example.com", "sha256:test", "")
	earlier := generateJob("registry.example.com", "sha256:test", "")
	earlier.Name = proxyJobName + "-older"
	unrelated := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: proxyJobNS, Labels: map[string]string{"name": "other"}}}
	pod := testJobPod(testJobName, time.Now(), corev1.PodSucceeded, corev1.ContainerState{})
	runner, client := newTestProxyCertJob(t, "Certificate: "+base64.StdEncoding.EncodeToString([]byte("crt")), legacy, earlier, unrelated, pod)

	if _, _, err := runner.run(context.Background(), testJob(jobComplete)); err != nil {
		t.Fatalf("run: %v", err)
	}

	jobs := client.BatchV1().Jobs(proxyJobNS)
	for _, name := range []string{proxyJobName, proxyJobName + "-older"} {
		if _, err := jobs.Get(context.Background(), name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
			t.Errorf("job %s: err = %v, want NotFound", name, err)
		}
	}
	for _, name := range []string{"other", testJobName} {
		if _, err := jobs.Get(context.Background(), name, metav1.GetOptions{}); err != nil {
			t.Errorf("job %s: %v, want it to exist", name, err)
		}
	}
}

func TestProxyCertJobKeepsRecentJobsOfAnotherRun(t *testing.T) {
	running := generateJob("registry.example.com", "sha256:test", "")
	running.Name = proxyJobName + "-other"
	running.CreationTimestamp = metav1.NewTime(time.Now())
	stale := generateJob("registry.example.com", "sha256:test", "")
	stale.Name = proxyJobName + "-stale"
	stale.CreationTimestamp = metav1.NewTime(time.Now().Add(-time.Hour))
	failed := generateJob("registry.example.com", "sha256:test", "")
	failed.Name = proxyJobName + "-failed"
	failed.CreationTimestamp = metav1.NewTime(time.Now())
	failed.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}
	pod := testJobPod(testJobName, time.Now(), corev1.PodSucceeded, corev1.ContainerState{})
	runner, client := newTestProxyCertJob(t, "Certificate: "+base64.StdEncoding.EncodeToString([]byte("crt")), running, stale, failed, pod)

	if _, _, err := runner.run(context.Background(), testJob(jobComplete)); err != nil {
		t.Fatalf("run: %v", err)
	}

	jobs := client.BatchV1().Jobs(proxyJobNS)
	for _, name := range []string{running.Name, failed.Name} {
		if _, err := jobs.Get(context.Background(), name, metav1.GetOptions{}); err != nil {
			t.Errorf("recent job %s: %v, want it kept", name, err)
		}
	}
	if _, err := jobs.Get(context.Background(), stale.Name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Errorf("job %s: err = %v, want NotFound", stale.Name, err)
	}
}

func TestProxyCertJobFailsAtOnceWhenItsJobIsDeleted(t *testing.T) {
	runner, client := newTestProxyCertJob(t, "")
	runner.timeout = time.Minute
	client.PrependReactor("get", "jobs", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(batchv1.Resource("jobs"), testJobName)
	})

	start := time.Now()
	_, _, err := runner.run(context.Background(), testJob())
	if err == nil || !strings.Contains(err.Error(), "was deleted before it completed") {
		t.Fatalf("err = %v, want the Job deleted", err)
	}
	if time.Since(start) > 10*time.Second {
		t.Errorf("run took %s, want it to stop without waiting for the timeout", time.Since(start))
	}
}

func TestProxyCertJobFailureReasons(t *testing.T) {
	const seccomp = `failed to create containerd task: failed to create shim task: OCI runtime create failed: runc create failed: unable to init seccomp: error loading seccomp filter into kernel: error loading seccomp filter: errno 524: unknown`
	now := time.Now()

	tests := []struct {
		name    string
		job     *batchv1.Job
		objects []runtime.Object
		want    []string
		notWant []string
	}{
		{
			name: "the pod cannot start, the Job does not complete in time",
			job:  testJob(),
			objects: []runtime.Object{
				testJobPod(testJobName+"-a", now, corev1.PodFailed, corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 128, Reason: "StartError", Message: seccomp,
				}}),
				testWarningEvent("e1", "Pod", testJobName+"-a", "Failed", "Error: "+seccomp, now),
				testWarningEvent("e2", "Pod", "someone-else", "Failed", "unrelated failure", now),
				&corev1.Event{
					ObjectMeta:     metav1.ObjectMeta{Name: "e3", Namespace: proxyJobNS},
					InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: testJobName + "-a"},
					Type:           corev1.EventTypeNormal, Reason: "Scheduled", Message: "normal event",
				},
			},
			want: []string{
				"job d8-system/proxy-cert-generate-job-test did not complete in 50ms",
				"pod proxy-cert-generate-job-test-a: phase Failed",
				"container generator terminated with exit code 128: StartError: " + seccomp,
				"events: pod/proxy-cert-generate-job-test-a Failed: Error: " + seccomp,
			},
			notWant: []string{"unrelated failure", "normal event"},
		},
		{
			name: "the Job fails after its retries",
			job: testJob(batchv1.JobCondition{
				Type: batchv1.JobFailed, Status: corev1.ConditionTrue,
				Reason: "BackoffLimitExceeded", Message: "Job has reached the specified backoff limit",
			}),
			objects: []runtime.Object{
				testJobPod(testJobName+"-a", now, corev1.PodFailed, corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1, Reason: "Error"}}),
				testWarningEvent("e1", "Job", testJobName, "BackoffLimitExceeded", "Job has reached the specified backoff limit", now),
			},
			want: []string{
				"job d8-system/proxy-cert-generate-job-test failed: BackoffLimitExceeded: Job has reached the specified backoff limit",
				"container generator terminated with exit code 1: Error",
				"events: job/proxy-cert-generate-job-test BackoffLimitExceeded",
			},
		},
		{
			name: "the image cannot be pulled",
			job:  testJob(),
			objects: []runtime.Object{
				testJobPod(testJobName+"-a", now, corev1.PodPending, corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
					Reason: "ImagePullBackOff", Message: `Back-off pulling image "registry.example.com@sha256:test"`,
				}}),
			},
			want: []string{`container generator waiting: ImagePullBackOff: Back-off pulling image "registry.example.com@sha256:test"`},
		},
		{
			name: "no pods were created",
			job:  testJob(),
			objects: []runtime.Object{
				testWarningEvent("e1", "Job", testJobName, "FailedCreate", `pods "proxy-cert-generate-job-test-" is forbidden: violates PodSecurity`, now),
			},
			want: []string{
				"no pods were created",
				`events: job/proxy-cert-generate-job-test FailedCreate: pods "proxy-cert-generate-job-test-" is forbidden: violates PodSecurity`,
			},
		},
		{
			name: "the pod is not scheduled",
			job:  testJob(),
			objects: []runtime.Object{
				func() *corev1.Pod {
					pod := testJobPod(testJobName+"-a", now, corev1.PodPending, corev1.ContainerState{})
					pod.Status.ContainerStatuses = nil
					pod.Status.Conditions = []corev1.PodCondition{{
						Type: corev1.PodScheduled, Status: corev1.ConditionFalse,
						Reason: "Unschedulable", Message: "0/3 nodes are available: 3 node(s) didn't match Pod's node affinity/selector.",
					}}
					return pod
				}(),
			},
			want: []string{"PodScheduled=False: Unschedulable: 0/3 nodes are available"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner, client := newTestProxyCertJob(t, "", tt.objects...)

			_, _, err := runner.run(context.Background(), tt.job)
			if err == nil {
				t.Fatal("run succeeded, want an error")
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q\ndoes not contain %q", err, want)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(err.Error(), notWant) {
					t.Errorf("error %q\ncontains %q", err, notWant)
				}
			}
			// The failed Job stays for inspection.
			if _, err := client.BatchV1().Jobs(proxyJobNS).Get(context.Background(), testJobName, metav1.GetOptions{}); err != nil {
				t.Errorf("failed job: %v, want it kept", err)
			}
		})
	}
}

func TestProxyCertJobCompletedWithoutCertificate(t *testing.T) {
	pod := testJobPod(testJobName, time.Now(), corev1.PodSucceeded, corev1.ContainerState{})
	runner, _ := newTestProxyCertJob(t, "something else\n", pod)

	_, _, err := runner.run(context.Background(), testJob(jobComplete))
	if err == nil || !strings.Contains(err.Error(), "no certificate in the log") {
		t.Fatalf("err = %v, want no certificate in the log", err)
	}
}

func TestProxyCertJobCompletedWithoutSucceededPod(t *testing.T) {
	pod := testJobPod(testJobName, time.Now(), corev1.PodFailed, corev1.ContainerState{})
	runner, _ := newTestProxyCertJob(t, "", pod)

	_, _, err := runner.run(context.Background(), testJob(jobComplete))
	if err == nil || !strings.Contains(err.Error(), "none of its 1 pods succeeded") {
		t.Fatalf("err = %v, want none of the pods succeeded", err)
	}
}

func TestParseProxyCert(t *testing.T) {
	crt := []byte("-----BEGIN CERTIFICATE-----\nMII\n-----END CERTIFICATE-----\n")
	encoded := base64.StdEncoding.EncodeToString(crt)

	tests := []struct {
		name    string
		logs    string
		want    string
		wantErr string
	}{
		// The generator prints the line without a trailing newline.
		{name: "only line", logs: "Certificate: " + encoded, want: string(crt)},
		{name: "after other lines", logs: "2026/11/18 starting\nCertificate: " + encoded + "\n", want: string(crt)},
		{name: "no certificate", logs: "2026/11/18 failed\n", wantErr: "no certificate in the log"},
		{name: "broken base64", logs: "Certificate: %%%", wantErr: "decode the certificate from the log"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseProxyCert(strings.NewReader(tt.logs))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDescribePodTruncatesLongMessages(t *testing.T) {
	long := strings.Repeat("é", diagnoseMaxFieldLen)
	pod := testJobPod("p", time.Now(), corev1.PodFailed, corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Error", Message: long}})

	got := describePod(pod)
	if !strings.HasSuffix(got, "...") || len(got) > diagnoseMaxFieldLen+200 {
		t.Errorf("describePod = %d bytes, want a truncated message", len(got))
	}
	if !strings.Contains(got, strings.Repeat("é", 10)) || strings.ContainsRune(got, '�') {
		t.Errorf("describePod broke a multibyte rune: %q", got[len(got)-10:])
	}
}
