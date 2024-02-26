/*
Copyright 2023 Flant JSC

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
	"fmt"
	"time"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	"github.com/cilium/cilium/pkg/defaults"
	v2_validation "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2/validator"
	k8sClient "github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/k8s/client/clientset/versioned/scheme"
	"github.com/cilium/cilium/pkg/option"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	validateK8sPoliciesTimeout = 5 * time.Minute
	ciliumGroup                = "cilium.io"
)

var (
	cnpMetricDesc = prometheus.NewDesc("cilium_bad_clusterwidepolicy", "Show when cluster policy invalid", nil, nil)
)

type Exporter struct {
	client     *k8sClient.Clientset
	kubeConfig *k8sClient.Config

	metrics []prometheus.Metric
}

func NewExporter() *Exporter {
	defaultConfig := k8sClient.Config{
		EnableK8s:             true,
		K8sAPIServer:          "",
		K8sKubeConfigPath:     "",
		K8sClientQPS:          defaults.K8sClientQPSLimit,
		K8sClientBurst:        defaults.K8sClientBurst,
		K8sHeartbeatTimeout:   30 * time.Second,
		EnableK8sAPIDiscovery: defaults.K8sEnableAPIDiscovery,
	}
	cl, err := k8sClient.NewStandaloneClientset(defaultConfig)
	if err != nil {
		klog.Fatalf("Create kubernetes client failed: %+v\n", err)
	}

	return &Exporter{
		client:     &cl,
		kubeConfig: &defaultConfig,
		metrics:    make([]prometheus.Metric, 0),
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- cnpMetricDesc
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	for _, m := range e.metrics {
		ch <- m
	}
}

func (e *Exporter) startScheduled(t time.Duration) {
	ticker = time.NewTicker(t)

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			err := validateCNPs(*e.client)
			result := 0
			if err != nil {
				result = 1
			}
			klog.Warningf("validate ploicy failed: %+v\n", err)
			allMetrics := make([]prometheus.Metric, 0, 1)
			v1 := prometheus.MustNewConstMetric(cnpMetricDesc, prometheus.GaugeValue, float64(result))
			allMetrics = append(allMetrics, v1)
			e.metrics = allMetrics
		}
	}
}

func validateCNPs(clientset k8sClient.Clientset) error {
	if !clientset.IsEnabled() {
		return fmt.Errorf("kubernetes client not configured. Please provide configuration via --%s or --%s",
			option.K8sAPIServer, option.K8sKubeConfigPath)
	}

	npValidator, err := v2_validation.NewNPValidator()
	if err != nil {
		return err
	}

	ctx, initCancel := context.WithTimeout(context.Background(), validateK8sPoliciesTimeout)
	defer initCancel()
	cnpErr := validateNPResources(ctx, clientset, npValidator.ValidateCNP, "ciliumnetworkpolicies", "CiliumNetworkPolicy")

	ctx, initCancel2 := context.WithTimeout(context.Background(), validateK8sPoliciesTimeout)
	defer initCancel2()
	ccnpErr := validateNPResources(ctx, clientset, npValidator.ValidateCCNP, "ciliumclusterwidenetworkpolicies", "CiliumClusterwideNetworkPolicy")

	if cnpErr != nil {
		return cnpErr
	}
	if ccnpErr != nil {
		return ccnpErr
	}
	klog.Info("All CCNPs and CNPs are valid")
	return nil
}

func validateNPResources(
	ctx context.Context,
	clientset k8sClient.Clientset,
	validator func(cnp *unstructured.Unstructured) error,
	name,
	shortName string,
) error {
	// Check if the crd is installed at all.
	_, err := clientset.ApiextensionsV1().CustomResourceDefinitions().Get(
		ctx,
		name+"."+ciliumGroup,
		metav1.GetOptions{},
	)
	switch {
	case err == nil:
	case k8sErrors.IsNotFound(err):
		return nil
	default:
		return err
	}

	var (
		policyErr error
		cnps      unstructured.UnstructuredList
		cnpName   string
	)
	for {
		opts := metav1.ListOptions{
			Limit:    25,
			Continue: cnps.GetContinue(),
		}
		err = clientset.
			CiliumV2().
			RESTClient().
			Get().
			VersionedParams(&opts, scheme.ParameterCodec).
			Resource(name).
			Do(ctx).
			Into(&cnps)
		if err != nil {
			return err
		}

		for _, cnp := range cnps.Items {
			if cnp.GetNamespace() != "" {
				cnpName = fmt.Sprintf("%s/%s", cnp.GetNamespace(), cnp.GetName())
			} else {
				cnpName = cnp.GetName()
			}
			if err := validator(&cnp); err != nil {
				klog.Errorf("Unexpected validation error for policy %s - %s: %s", shortName, cnpName, err)
				policyErr = fmt.Errorf("found invalid %s", shortName)
			} else {
				klog.Info("Validation OK!", shortName, cnpName)
			}
		}
		if cnps.GetContinue() == "" {
			break
		}
	}
	return policyErr
}
