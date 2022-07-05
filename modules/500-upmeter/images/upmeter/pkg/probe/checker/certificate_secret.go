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

package checker

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/run"
)

// CertificateSecretLifecycle is a checker constructor and configurator
type CertificateSecretLifecycle struct {
	Access    kubernetes.Access
	Namespace string
	AgentID   string

	CreationTimeout time.Duration
	DeletionTimeout time.Duration

	ControlPlaneAccessTimeout time.Duration
}

func (c CertificateSecretLifecycle) Checker() check.Checker {
	return &CertificateSecretLifecycleChecker{
		Access:    c.Access,
		Namespace: c.Namespace,
		AgentID:   c.AgentID,

		CreationTimeout: c.CreationTimeout,
		DeletionTimeout: c.DeletionTimeout,

		ControlPlaneAccessTimeout: c.ControlPlaneAccessTimeout,
	}
}

type CertificateSecretLifecycleChecker struct {
	Access    kubernetes.Access
	Namespace string
	AgentID   string

	CreationTimeout time.Duration
	DeletionTimeout time.Duration

	ControlPlaneAccessTimeout time.Duration
}

func (c *CertificateSecretLifecycleChecker) Check() check.Error {
	return c.new(c.name()).Check()
}

func (c *CertificateSecretLifecycleChecker) name() string {
	// should be new for each run in order not to get stuck with created object
	return run.StaticIdentifier("upmeter-cm-probe")
}

func (c *CertificateSecretLifecycleChecker) new(name string) check.Checker {
	pingControlPlaneOrUnknown := newControlPlaneChecker(c.Access, c.ControlPlaneAccessTimeout)

	createCert := &certificateCreator{
		access:    c.Access,
		name:      name,
		agentID:   c.AgentID,
		namespace: c.Namespace,
	}

	deleteCert := &certificateDeleter{
		access:    c.Access,
		name:      name,
		namespace: c.Namespace,
	}

	createCertOrUnknown := doOrUnknown(c.CreationTimeout, createCert)

	getSecretOrFail := withTimeout(
		&secretExistenceChecker{
			access:    c.Access,
			name:      name,
			namespace: c.Namespace,
		},
		c.ControlPlaneAccessTimeout,
	)

	getNoSecretOrFail := withTimeout(
		&secretNonexistenceChecker{
			access:    c.Access,
			name:      name,
			namespace: c.Namespace,
		},
		c.ControlPlaneAccessTimeout,
	)

	return sequence(
		pingControlPlaneOrUnknown,
		createCertOrUnknown,
		withFinalizer(
			getSecretOrFail,
			deleteCert,
		),
		getNoSecretOrFail,
	)
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
		Create(obj, metav1.CreateOptions{})

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
		Delete(c.name, &metav1.DeleteOptions{})
}

type secretExistenceChecker struct {
	access    kubernetes.Access
	name      string
	namespace string
}

func (c *secretExistenceChecker) Check() check.Error {
	_, err := c.access.Kubernetes().CoreV1().Secrets(c.namespace).Get(c.name, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	if apierrors.IsNotFound(err) {
		return check.ErrFail("secret %s/%s is not there: %w", c.namespace, c.name, err)
	}

	return check.ErrUnknown("getting %s/%s: %w", c.namespace, c.name, err)
}

type secretNonexistenceChecker struct {
	access    kubernetes.Access
	name      string
	namespace string
}

func (c *secretNonexistenceChecker) Check() check.Error {
	_, err := c.access.Kubernetes().CoreV1().Secrets(c.namespace).Get(c.name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return check.ErrUnknown("getting %s/%s: %w", c.namespace, c.name, err)
	}
	return check.ErrFail("secret %s/%s is still there", c.namespace, c.name)
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
