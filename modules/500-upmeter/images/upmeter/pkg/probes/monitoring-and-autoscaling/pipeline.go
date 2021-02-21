package monitoring_and_autoscaling

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/flant/shell-operator/pkg/kube"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"upmeter/pkg/checks"
	"upmeter/pkg/probes/util"
)

// PodChecker defines three steps to get the information
// about existing pods, and request to them to ensure
// they work as expected
type PodChecker interface {
	// Data to track pods
	Namespace() string
	LabelSelector() string

	// Endpoint for probe request
	Endpoint() string

	// Do the actual probe to the service
	Verify(body []byte) checks.Error
}

// using insecure transport because kube-rbac-proxy generates self-signed certificates, causing cert validation error
var insecureClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

func NewPodCheckPipeline(probe *checks.Probe, timeout time.Duration, checker PodChecker) PodCheckPipeline {
	return PodCheckPipeline{
		checker: checker,
		logger:  probe.LogEntry(),

		probe:   probe,
		timeout: timeout,
		client:  insecureClient,
	}
}

// PodCheckPipeline checks that at least one pod is ready and the service is available.
// The pipeline send request specified by checker and verifies it by the checker
type PodCheckPipeline struct {
	checker PodChecker
	logger  *log.Entry

	probe   *checks.Probe
	timeout time.Duration
	client  *http.Client
}

func (pp PodCheckPipeline) ReportError(err checks.Error) {
	pp.logger.Errorf(err.Error())
	if err.Result() == checks.StatusNoData {
		return
	}
	pp.probe.ResultCh <- pp.probe.Result(err.Result())
}

func (pp PodCheckPipeline) ReportSuccess() {
	pp.probe.ResultCh <- pp.probe.Result(checks.StatusSuccess)
}

// Go launches the probe checks and reports the result
func (pp PodCheckPipeline) Go() {
	err := pp.ensureReadyPod()
	if err != nil {
		pp.ReportError(err)
		return
	}

	err = pp.runProbe()
	if err != nil {
		pp.ReportError(err)
		return
	}

	pp.ReportSuccess()
}

func (pp PodCheckPipeline) runProbe() checks.Error {
	var (
		err  checks.Error
		req  *http.Request
		body []byte
	)

	util.DoWithTimer(pp.timeout,
		func() {
			req, err = newRequest(pp.checker.Endpoint(), pp.probe.ServiceAccountToken)
			if err != nil {
				return
			}
			body, err = doRequest(pp.client, req)
			if err != nil {
				return
			}
			err = pp.checker.Verify(body)
		},
		func() {
			err = checks.ErrUnknownResult("probe request timed out")
		},
	)

	return err
}

func (pp PodCheckPipeline) ensureReadyPod() checks.Error {
	var (
		namespace     = pp.checker.Namespace()
		labelSelector = pp.checker.LabelSelector()
	)

	var err checks.Error
	util.DoWithTimer(pp.timeout,
		func() {
			_, err = getReadyPod(pp.probe.KubernetesClient, namespace, labelSelector)
		},
		func() {
			err = checks.ErrUnknownResult("getting pods timed out %s", podFilter(namespace, labelSelector))
		},
	)
	return err
}

func getReadyPod(client kube.KubernetesClient, namespace, labelSelector string) (*v1.Pod, checks.Error) {
	podList, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, checks.ErrUnknownResult("cannot get pods in API %s: %v", podFilter(namespace, labelSelector), err)
	}

	for _, pod := range podList.Items {
		if isPodReady(&pod) {
			return &pod, nil
		}

	}

	return nil, checks.ErrFail("no ready pods found %s", podFilter(namespace, labelSelector))
}

func isPodReady(pod *v1.Pod) bool {
	if pod.Status.Phase != v1.PodRunning {
		return false
	}

	for _, cnd := range pod.Status.Conditions {
		if cnd.Status != v1.ConditionTrue {
			return false
		}
	}

	return true
}

func podFilter(namespace, labelSelector string) string {
	return fmt.Sprintf("%s,%s", namespace, labelSelector)
}
