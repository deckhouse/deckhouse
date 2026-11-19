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
	"io"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	proxyJobNS = "d8-system"
	// The prefix of the Job names and the value of their "name" label.
	proxyJobName      = "proxy-cert-generate-job"
	proxyJobContainer = "generator"

	// The generator issues a certificate for 72 hours, the hook renews it when less than 48 hours are left.
	proxyCertRenewBefore  = 48 * time.Hour
	proxyCertRetryCrontab = "*/5 * * * *"

	proxyCertRenewalFailedMetric = "d8_user_authn_basic_auth_proxy_cert_renewal_failed"
	proxyCertRenewalMetricGroup  = "d8_user_authn_basic_auth_proxy_cert_renewal"

	// The generator signs the CSR with the front-proxy CA. Only these two files are mounted,
	// the rest of /etc/kubernetes/pki holds the cluster CA and other signing keys.
	frontProxyCACertPath = "/etc/kubernetes/pki/front-proxy-ca.crt"
	frontProxyCAKeyPath  = "/etc/kubernetes/pki/front-proxy-ca.key"

	securityPolicyExceptionLabel = "security.deckhouse.io/security-policy-exception"
	// Must match the SecurityPolicyException in templates/basic-auth-proxy/security-policy-exception.yaml.
	proxyJobSecurityPolicyExceptionName = "user-authn-proxy-cert-generate-job"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/user-authn/generate_basic_auth_proxy_cert",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Schedule: []go_hook.ScheduleConfig{
		{
			// Check urgent failures frequently; the retry state limits Job attempts.
			Name:    "generate_basic_auth_proxy_cert",
			Crontab: proxyCertRetryCrontab,
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-user-authn"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"basic-auth-cert"},
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
	OIDC struct {
		EnableBasicAuth bool `json:"enableBasicAuth"`
	} `json:"oidc"`
	LDAP struct {
		EnableBasicAuth bool `json:"enableBasicAuth"`
	} `json:"ldap"`
}

func generateProxyAuthCert(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	// A run that finds nothing to report clears the signal of an earlier failed renewal.
	input.MetricsCollector.Expire(proxyCertRenewalMetricGroup)

	// check proxy rollout conditions
	if !input.Values.Get("userAuthn.internal.publishAPI.enabled").Bool() {
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

	var config *provider

	for _, prov := range providers {
		if prov.Typ == "Crowd" && prov.Crowd.EnableBasicAuth {
			if config != nil {
				return errors.New("multiple 'enableBasicAuth: true' options are enabled while only one is supported, review your authentication configuration")
			}
			config = &prov
		}
		if prov.Typ == "OIDC" && prov.OIDC.EnableBasicAuth {
			if config != nil {
				return errors.New("multiple 'enableBasicAuth: true' options are enabled while only one is supported, review your authentication configuration")
			}
			config = &prov
		}
		if prov.Typ == "LDAP" && prov.LDAP.EnableBasicAuth {
			if config != nil {
				return errors.New("multiple 'enableBasicAuth: true' options are enabled while only one is supported, review your authentication configuration")
			}
			config = &prov
		}
	}

	if config == nil {
		return nil
	}

	current, expiring, err := currentProxyCert(input)
	if err != nil {
		return err
	}
	if current != nil && !expiring {
		setProxyCertValues(input, current.Crt, current.Key)
		return nil
	}

	// Avoid waiting for a Job on every module run after a failure. Missing or expired certificates
	// use a shorter pause because forwarded API requests need a valid certificate.
	if proxyCertRetries.wait(current) {
		reportProxyCertFailure(input, current)
		return nil
	}

	crt, key, err := issueProxyCert(ctx, input, dc)
	if err == nil {
		proxyCertRetries.succeeded()
		setProxyCertValues(input, crt, key)
		return nil
	}
	proxyCertRetries.failed()

	if current == nil {
		input.Logger.Error("cannot issue the basic-auth-proxy client certificate, basic-auth-proxy is not deployed until it is issued",
			"error", err)
	} else {
		input.Logger.Error("cannot renew the basic-auth-proxy client certificate, keeping the current one",
			"not_after", current.NotAfter.UTC().Format(time.RFC3339), "error", err)
	}
	reportProxyCertFailure(input, current)

	return nil
}

// reportProxyCertFailure keeps the module rendering after a failed issue. Without a certificate the
// templates leave basic-auth-proxy out; with one, the current certificate stays in use. The metric
// reports the failure until an issue succeeds.
func reportProxyCertFailure(input *go_hook.HookInput, current *proxyCert) {
	if current == nil {
		reportProxyCertRenewalFailed(input, proxyCertStateMissing)
		return
	}

	state := proxyCertStateExpiring
	if !proxyCertNow().Before(current.NotAfter) {
		state = proxyCertStateExpired
	}
	reportProxyCertRenewalFailed(input, state)
	setProxyCertValues(input, current.Crt, current.Key)
}

// After the pause, the next scheduled or module run retries. A scheduled run before the pause ends
// skips the attempt; the five-minute schedule bounds the additional delay while Deckhouse runs.
const (
	proxyCertRetryInterval       = 50 * time.Minute
	proxyCertUrgentRetryInterval = 5 * time.Minute
)

// proxyCertNow is a variable for tests.
var proxyCertNow = time.Now

// proxyCertRetries remembers the last failed issue. It lives in memory: after a restart of Deckhouse
// the first run tries at once.
var proxyCertRetries = &proxyCertRetryState{}

type proxyCertRetryState struct {
	mu       sync.Mutex
	failedAt time.Time
}

func (s *proxyCertRetryState) wait(current *proxyCert) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := proxyCertNow()
	interval := proxyCertRetryInterval
	if current == nil || !now.Before(current.NotAfter) {
		interval = proxyCertUrgentRetryInterval
	}
	return !s.failedAt.IsZero() && now.Sub(s.failedAt) < interval
}

func (s *proxyCertRetryState) failed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failedAt = proxyCertNow()
}

func (s *proxyCertRetryState) succeeded() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failedAt = time.Time{}
}

// The states of the certificate the failure metric reports.
const (
	proxyCertStateMissing  = "missing"
	proxyCertStateExpiring = "expiring"
	proxyCertStateExpired  = "expired"
)

func reportProxyCertRenewalFailed(input *go_hook.HookInput, state string) {
	input.MetricsCollector.Set(proxyCertRenewalFailedMetric, 1, map[string]string{"state": state},
		metrics.WithGroup(proxyCertRenewalMetricGroup))
}

// currentProxyCert returns the certificate in use and whether it has to be renewed. It takes the
// later of the certificate in the basic-auth-cert Secret and the one in the values: a run that has
// just issued a certificate puts it into the values before Helm writes it to the Secret. It returns
// nil when neither holds a usable certificate: no Secret, empty fields or a PEM that does not parse.
func currentProxyCert(input *go_hook.HookInput) (*proxyCert, bool, error) {
	var fromSecret *proxyCert

	snap := input.Snapshots.Get("secret")
	if len(snap) > 0 {
		var sec secret
		if err := snap[0].UnmarshalTo(&sec); err != nil {
			return nil, false, fmt.Errorf("failed to unmarshal 'secret' snapshot: %w", err)
		}
		fromSecret = parseProxyCertPair(input, "Secret", sec.Crt, sec.Key)
	}

	crt, crtErr := base64.StdEncoding.DecodeString(input.Values.Get("userAuthn.internal.basicAuthProxyCert").String())
	key, keyErr := base64.StdEncoding.DecodeString(input.Values.Get("userAuthn.internal.basicAuthProxyKey").String())
	var fromValues *proxyCert
	if crtErr == nil && keyErr == nil {
		fromValues = parseProxyCertPair(input, "values", crt, key)
	}

	current := fromSecret
	if fromValues != nil && (current == nil || fromValues.NotAfter.After(current.NotAfter)) {
		current = fromValues
	}
	if current == nil {
		return nil, false, nil
	}

	return current, current.NotAfter.Sub(proxyCertNow()) < proxyCertRenewBefore, nil
}

func parseProxyCertPair(input *go_hook.HookInput, source string, crt, key []byte) *proxyCert {
	if len(crt) == 0 || len(key) == 0 {
		return nil
	}

	parsed, err := helpers.ParseCertificatePEM(crt)
	if err != nil {
		input.Logger.Warn("cannot parse the basic-auth-proxy client certificate", "source", source, "error", err)
		return nil
	}

	return &proxyCert{Crt: crt, Key: key, NotAfter: parsed.NotAfter}
}

type proxyCert struct {
	Crt      []byte
	Key      []byte
	NotAfter time.Time
}

func setProxyCertValues(input *go_hook.HookInput, crt, key []byte) {
	input.Values.Set("userAuthn.internal.basicAuthProxyCert", base64.StdEncoding.EncodeToString(crt))
	input.Values.Set("userAuthn.internal.basicAuthProxyKey", base64.StdEncoding.EncodeToString(key))
}

// issueProxyCert signs a new client certificate with the front-proxy CA. Only the control-plane
// nodes have the CA key, so a Job on one of them signs the CSR and prints the certificate.
func issueProxyCert(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) ([]byte, []byte, error) {
	gcsr, pkey, err := certificate.GenerateCSR(input.Logger, "front-proxy-client", certificate.WithCSRKeyRequest(&csr.KeyRequest{A: "rsa", S: 2048}))
	if err != nil {
		return nil, nil, fmt.Errorf("generate CSR: %w", err)
	}

	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return nil, nil, fmt.Errorf("get Kubernetes client: %w", err)
	}

	registry := input.Values.Get("global.modulesImages.registry.base").String()
	digest := input.Values.Get("global.modulesImages.digests.userAuthn.selfSignedGenerator").String()

	jobName, crt, err := newProxyCertJobRunner(kubeClient).run(ctx, generateJob(registry, digest, base64.StdEncoding.EncodeToString(gcsr)))
	if err != nil {
		return nil, nil, err
	}

	// A failed Job stays for inspection; the next run deletes it before it creates a new one.
	input.PatchCollector.Delete("batch/v1", "Job", proxyJobNS, jobName)

	return crt, pkey, nil
}

// proxyCertJobRunner runs the generator Job and reads the certificate from the log of its pod.
type proxyCertJobRunner interface {
	run(ctx context.Context, job *batchv1.Job) (jobName string, crt []byte, err error)
}

// newProxyCertJobRunner is a variable for tests, which cannot run a Job with the fake client.
var newProxyCertJobRunner = func(client kubernetes.Interface) proxyCertJobRunner {
	return &proxyCertJob{
		client:       client,
		pollInterval: 2 * time.Second,
		timeout:      30 * time.Second,
		nameSuffix:   func() string { return utilrand.String(5) },
		podLogs: func(ctx context.Context, pod string) (io.ReadCloser, error) {
			return client.CoreV1().Pods(proxyJobNS).GetLogs(pod, &corev1.PodLogOptions{Container: proxyJobContainer}).Stream(ctx)
		},
	}
}

type proxyCertJob struct {
	client       kubernetes.Interface
	pollInterval time.Duration
	timeout      time.Duration
	nameSuffix   func() string
	podLogs      func(ctx context.Context, pod string) (io.ReadCloser, error)
}

func (r *proxyCertJob) run(ctx context.Context, job *batchv1.Job) (string, []byte, error) {
	if err := r.deletePreviousJobs(ctx); err != nil {
		return "", nil, err
	}

	// A new name for every Job: a Job deleted in the foreground keeps its name until its pods are
	// gone, and a Job created with the same name meanwhile fails with AlreadyExists.
	job.Name = proxyJobName + "-" + r.nameSuffix()
	created, err := r.client.BatchV1().Jobs(proxyJobNS).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return "", nil, fmt.Errorf("create Job %s/%s: %w", proxyJobNS, job.Name, err)
	}

	if err := r.waitForCompletion(ctx, created.Name); err != nil {
		return "", nil, fmt.Errorf("%w; %s", err, r.diagnose(ctx, created.Name))
	}

	crt, err := r.readCert(ctx, created.Name)
	if err != nil {
		return "", nil, err
	}

	return created.Name, crt, nil
}

// deletePreviousJobs deletes the Jobs of earlier runs: a failed one is kept for inspection, and
// releases before the per-run names left one named proxy-cert-generate-job.
func (r *proxyCertJob) deletePreviousJobs(ctx context.Context) error {
	jobs, err := r.client.BatchV1().Jobs(proxyJobNS).List(ctx, metav1.ListOptions{LabelSelector: "name=" + proxyJobName})
	if err != nil {
		return fmt.Errorf("list Jobs %s in %s: %w", proxyJobName, proxyJobNS, err)
	}

	for _, job := range jobs.Items {
		// A parallel run may still be waiting, reading its certificate or collecting diagnostics,
		// even if the Job has already finished.
		if proxyCertNow().Sub(job.CreationTimestamp.Time) < 2*r.timeout {
			continue
		}
		err := r.client.BatchV1().Jobs(proxyJobNS).Delete(ctx, job.Name, metav1.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationBackground)})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete Job %s/%s: %w", proxyJobNS, job.Name, err)
		}
	}

	return nil
}

// waitForCompletion waits until the Job completes, fails or runs out of time.
func (r *proxyCertJob) waitForCompletion(ctx context.Context, name string) error {
	waitCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var lastErr error
	err := wait.PollUntilContextCancel(waitCtx, r.pollInterval, true, func(ctx context.Context) (bool, error) {
		job, err := r.client.BatchV1().Jobs(proxyJobNS).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, fmt.Errorf("job %s/%s was deleted before it completed", proxyJobNS, name)
		}
		if err != nil {
			lastErr = err
			return false, nil
		}
		for _, cond := range job.Status.Conditions {
			if cond.Status != corev1.ConditionTrue {
				continue
			}
			switch cond.Type {
			case batchv1.JobComplete:
				return true, nil
			case batchv1.JobFailed:
				return false, fmt.Errorf("job %s/%s failed: %s: %s", proxyJobNS, name, cond.Reason, cond.Message)
			}
		}
		return false, nil
	})

	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
		err = fmt.Errorf("job %s/%s did not complete in %s", proxyJobNS, name, r.timeout)
		if lastErr != nil {
			err = fmt.Errorf("%w, last error: %w", err, lastErr)
		}
	}

	return err
}

// readCert reads the certificate from the log of the pod that succeeded. With a backoff limit, a
// failed pod of the same Job can be listed first.
func (r *proxyCertJob) readCert(ctx context.Context, jobName string) ([]byte, error) {
	pods, err := r.client.CoreV1().Pods(proxyJobNS).List(ctx, metav1.ListOptions{LabelSelector: "job-name=" + jobName})
	if err != nil {
		return nil, fmt.Errorf("list pods of Job %s/%s: %w", proxyJobNS, jobName, err)
	}

	var podName string
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodSucceeded {
			podName = pod.Name
			break
		}
	}
	if podName == "" {
		return nil, fmt.Errorf("job %s/%s completed, but none of its %d pods succeeded", proxyJobNS, jobName, len(pods.Items))
	}

	stream, err := r.podLogs(ctx, podName)
	if err != nil {
		return nil, fmt.Errorf("read log of pod %s/%s: %w", proxyJobNS, podName, err)
	}
	defer stream.Close()

	crt, err := parseProxyCert(stream)
	if err != nil {
		return nil, fmt.Errorf("pod %s/%s: %w", proxyJobNS, podName, err)
	}

	return crt, nil
}

// parseProxyCert finds the line "Certificate: <base64 PEM>" the generator prints.
func parseProxyCert(logs io.Reader) ([]byte, error) {
	sc := bufio.NewScanner(logs)
	for sc.Scan() {
		certb64, ok := strings.CutPrefix(sc.Text(), "Certificate: ")
		if !ok {
			continue
		}
		crt, err := base64.StdEncoding.DecodeString(certb64)
		if err != nil {
			return nil, fmt.Errorf("decode the certificate from the log: %w", err)
		}
		return crt, nil
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read the log: %w", err)
	}

	return nil, errors.New("no certificate in the log")
}

const (
	diagnoseTimeout     = 10 * time.Second
	diagnoseMaxPods     = 3
	diagnoseMaxEvents   = 5
	diagnoseMaxFieldLen = 512
)

// diagnose explains why the Job did not complete: the state of its pods and their containers and
// the warning events of the Job and the pods. Errors on the way go into the text, the caller
// already has an error to return.
func (r *proxyCertJob) diagnose(ctx context.Context, jobName string) string {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), diagnoseTimeout)
	defer cancel()

	var parts []string
	objects := map[string]bool{jobName: true}

	pods, err := r.client.CoreV1().Pods(proxyJobNS).List(ctx, metav1.ListOptions{LabelSelector: "job-name=" + jobName})
	switch {
	case err != nil:
		parts = append(parts, fmt.Sprintf("cannot list pods: %v", err))
	case len(pods.Items) == 0:
		parts = append(parts, "no pods were created")
	default:
		items := pods.Items
		// The newest pods first: they show the last attempt.
		sort.Slice(items, func(i, j int) bool {
			return items[j].CreationTimestamp.Before(&items[i].CreationTimestamp)
		})
		for i := range items {
			objects[items[i].Name] = true
			if i < diagnoseMaxPods {
				parts = append(parts, describePod(&items[i]))
			}
		}
	}

	if events := r.warningEvents(ctx, objects); events != "" {
		parts = append(parts, events)
	}

	return strings.Join(parts, "; ")
}

func describePod(pod *corev1.Pod) string {
	var b strings.Builder
	fmt.Fprintf(&b, "pod %s: phase %s", pod.Name, pod.Status.Phase)
	if pod.Status.Reason != "" || pod.Status.Message != "" {
		fmt.Fprintf(&b, ", %s", reasonMessage(pod.Status.Reason, pod.Status.Message))
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Status == corev1.ConditionFalse && cond.Message != "" {
			fmt.Fprintf(&b, ", %s=False: %s", cond.Type, reasonMessage(cond.Reason, cond.Message))
		}
	}

	statuses := append(slices.Clone(pod.Status.InitContainerStatuses), pod.Status.ContainerStatuses...)
	for _, cs := range statuses {
		if state := describeContainerState(cs.State); state != "" {
			fmt.Fprintf(&b, ", container %s %s", cs.Name, state)
		}
		if cs.LastTerminationState.Terminated != nil {
			fmt.Fprintf(&b, ", container %s last %s", cs.Name, describeContainerState(cs.LastTerminationState))
		}
	}

	return b.String()
}

func describeContainerState(state corev1.ContainerState) string {
	switch {
	case state.Waiting != nil:
		return "waiting: " + reasonMessage(state.Waiting.Reason, state.Waiting.Message)
	case state.Terminated != nil:
		return fmt.Sprintf("terminated with exit code %d: %s", state.Terminated.ExitCode,
			reasonMessage(state.Terminated.Reason, state.Terminated.Message))
	default:
		return ""
	}
}

func reasonMessage(reason, message string) string {
	message = truncate(strings.TrimSpace(message))
	switch {
	case message == "":
		return reason
	case reason == "":
		return message
	default:
		return reason + ": " + message
	}
}

// warningEvents returns the latest warning events of the given objects, oldest first.
func (r *proxyCertJob) warningEvents(ctx context.Context, objects map[string]bool) string {
	list, err := r.client.CoreV1().Events(proxyJobNS).List(ctx, metav1.ListOptions{FieldSelector: "type=" + corev1.EventTypeWarning})
	if err != nil {
		return fmt.Sprintf("cannot list events: %v", err)
	}

	var events []corev1.Event
	for _, ev := range list.Items {
		// The fake client ignores field selectors, so the type is checked here as well.
		if ev.Type == corev1.EventTypeWarning && objects[ev.InvolvedObject.Name] &&
			(ev.InvolvedObject.Kind == "Pod" || ev.InvolvedObject.Kind == "Job") {
			events = append(events, ev)
		}
	}
	if len(events) == 0 {
		return ""
	}

	sort.SliceStable(events, func(i, j int) bool { return eventTime(events[i]).Before(eventTime(events[j])) })
	if len(events) > diagnoseMaxEvents {
		events = events[len(events)-diagnoseMaxEvents:]
	}

	lines := make([]string, 0, len(events))
	for _, ev := range events {
		lines = append(lines, fmt.Sprintf("%s/%s %s", strings.ToLower(ev.InvolvedObject.Kind), ev.InvolvedObject.Name,
			reasonMessage(ev.Reason, ev.Message)))
	}

	return "events: " + strings.Join(lines, ", ")
}

func eventTime(ev corev1.Event) time.Time {
	switch {
	case !ev.LastTimestamp.IsZero():
		return ev.LastTimestamp.Time
	case !ev.EventTime.IsZero():
		return ev.EventTime.Time
	default:
		return ev.CreationTimestamp.Time
	}
}

func truncate(s string) string {
	if len(s) <= diagnoseMaxFieldLen {
		return s
	}
	// Cutting at a byte offset can split a multibyte rune, drop its remains.
	return strings.ToValidUTF8(s[:diagnoseMaxFieldLen], "") + "..."
}

func generateJob(registry, digest, csrb64 string) *batchv1.Job {
	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      proxyJobName,
			Namespace: proxyJobNS,
			Labels: map[string]string{
				"name":     proxyJobName,
				"heritage": "deckhouse",
				"module":   "user-authn",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To(int32(1)),
			// A failed Job stays for inspection until the next run deletes it. This removes it
			// when there is no next run, for example after basic authentication is turned off.
			TTLSecondsAfterFinished: ptr.To(int32(24 * 60 * 60)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						securityPolicyExceptionLabel: proxyJobSecurityPolicyExceptionName,
					},
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						// The front-proxy CA key on the host is readable by root only.
						RunAsUser:    ptr.To(int64(0)),
						RunAsGroup:   ptr.To(int64(0)),
						RunAsNonRoot: ptr.To(false),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					// The generator only signs the CSR and never calls the API server.
					AutomountServiceAccountToken: ptr.To(false),
					ImagePullSecrets:             []corev1.LocalObjectReference{{Name: "deckhouse-registry"}},
					Containers: []corev1.Container{
						{
							Name:  proxyJobContainer,
							Image: fmt.Sprintf("%s@%s", registry, digest),
							Args:  []string{"generate-proxy-certs"},
							Env: []corev1.EnvVar{
								{
									Name:  "CSR",
									Value: csrb64,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								ReadOnlyRootFilesystem:   ptr.To(true),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "front-proxy-ca-crt",
									ReadOnly:  true,
									MountPath: frontProxyCACertPath,
								},
								{
									Name:      "front-proxy-ca-key",
									ReadOnly:  true,
									MountPath: frontProxyCAKeyPath,
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{
						{
							Name: "front-proxy-ca-crt",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: frontProxyCACertPath,
									Type: ptr.To(corev1.HostPathFile),
								},
							},
						},
						{
							Name: "front-proxy-ca-key",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: frontProxyCAKeyPath,
									Type: ptr.To(corev1.HostPathFile),
								},
							},
						},
					},
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
