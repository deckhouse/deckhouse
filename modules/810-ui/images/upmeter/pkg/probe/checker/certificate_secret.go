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

package checker

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

// CertificateSecretLifecycle is a checker constructor and configurator
type CertificateSecretLifecycle struct {
	Access    kubernetes.Access
	Preflight Doer

	Namespace string
	AgentID   string
	Name      string

	CreationTimeout         time.Duration
	DeletionTimeout         time.Duration
	SecretTransitionTimeout time.Duration
}

func (c CertificateSecretLifecycle) Checker() check.Checker {
	certGetter := &certificateGetter{access: c.Access, namespace: c.Namespace, name: c.Name}

	certCreator := doWithTimeout(
		&certificateCreator{access: c.Access, namespace: c.Namespace, name: c.Name, agentID: c.AgentID},
		c.CreationTimeout,
		fmt.Errorf("creation timeout reached"),
	)

	certDeleter := doWithTimeout(
		&certificateDeleter{access: c.Access, namespace: c.Namespace, name: c.Name},
		c.DeletionTimeout,
		fmt.Errorf("deletion timeout reached"),
	)

	certSecretGetter := &secretGetter{access: c.Access, namespace: c.Namespace, name: c.Name}
	certSecretDeleter := &secretDeleter{access: c.Access, namespace: c.Namespace, name: c.Name}

	// Not to rarely
	pollInterval := c.SecretTransitionTimeout / 10
	if pollInterval > 5*time.Second {
		pollInterval = 5 * time.Second
	}

	checker := &KubeControllerObjectLifecycle{
		preflight: c.Preflight,

		parentGetter:  certGetter,
		parentCreator: certCreator,
		parentDeleter: certDeleter,

		childGetter:          certSecretGetter,
		childDeleter:         certSecretDeleter,
		childPollingInterval: pollInterval,
		childPollingTimeout:  c.SecretTransitionTimeout,
	}

	return checker
}

type certificateCreator struct {
	access  kubernetes.Access
	agentID string

	name      string
	namespace string
}

func (c *certificateCreator) Do(ctx context.Context) error {
	decUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	obj := &unstructured.Unstructured{}
	manifest := certificateManifest(c.agentID, c.name, c.namespace)

	if _, _, err := decUnstructured.Decode([]byte(manifest), nil, obj); err != nil {
		return err
	}

	_, err := c.access.Kubernetes().Dynamic().
		Resource(certificateGVR).
		Namespace(c.namespace).
		Create(ctx, obj, metav1.CreateOptions{})

	return err
}

type certificateDeleter struct {
	access    kubernetes.Access
	name      string
	namespace string
}

func (c *certificateDeleter) Do(ctx context.Context) error {
	return c.access.Kubernetes().Dynamic().
		Resource(certificateGVR).
		Namespace(c.namespace).
		Delete(ctx, c.name, metav1.DeleteOptions{})
}

type certificateGetter struct {
	access    kubernetes.Access
	name      string
	namespace string
}

func (c *certificateGetter) Do(ctx context.Context) error {
	_, err := c.access.Kubernetes().Dynamic().
		Resource(certificateGVR).
		Namespace(c.namespace).
		Get(ctx, c.name, metav1.GetOptions{})
	return err
}

type secretGetter struct {
	access    kubernetes.Access
	name      string
	namespace string
}

func (c *secretGetter) Do(ctx context.Context) error {
	_, err := c.access.Kubernetes().CoreV1().Secrets(c.namespace).Get(ctx, c.name, metav1.GetOptions{})
	return err
}

type secretDeleter struct {
	access    kubernetes.Access
	name      string
	namespace string
}

func (c *secretDeleter) Do(ctx context.Context) error {
	err := c.access.Kubernetes().CoreV1().Secrets(c.namespace).Delete(ctx, c.name, metav1.DeleteOptions{})
	return err
}

var certificateGVR = schema.GroupVersionResource{
	Group:    "cert-manager.io",
	Version:  "v1",
	Resource: "certificates",
}

func certificateManifest(agentID, name, namespace string) string {
	tpl := `
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %q
    upmeter-group: control-plane
    upmeter-probe: cert-manager
  name: %q
  namespace: %q
spec:
  certificateOwnerRef: true
  dnsNames:
  - nothing-%s.example.com
  issuerRef:
    kind: ClusterIssuer
    name: selfsigned
  secretName: %q
  secretTemplate:
    labels:
      heritage: upmeter
      upmeter-agent: %q
      upmeter-group: control-plane
      upmeter-probe: cert-manager
`

	return fmt.Sprintf(tpl,
		agentID,   // certificate label
		name,      // certificate name
		namespace, // certificate namespace
		agentID,   // dnsName part
		name,      // secret name
		agentID,   // secret label
	)
}
