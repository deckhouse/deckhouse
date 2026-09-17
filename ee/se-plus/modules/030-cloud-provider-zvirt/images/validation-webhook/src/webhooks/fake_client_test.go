/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package webhooks

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvaladmission "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/admission"

	zicv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/instanceclass/v1"
	zmeta "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/meta"
	zval "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/validation"
)

const masterInstanceClassName = "master-fc613b4dfd67"

var (
	nodeGroupGVK    = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroup"}
	moduleConfigGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig"}
)

// A fake client is enough here: these tests are about the adapter — what the validator puts into
// the state and what it turns the result into — not about the API server.
func newWebhookAdmissionStateBuilderFactory(t *testing.T, objects ...runtime.Object) *zval.AdmissionStateBuilderFactory {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}

	client := clientfake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()

	return zval.NewAdmissionStateBuilderFactory(client, cpvaladmission.StateBuilderConfig{
		ModuleName:       zmeta.ModuleName,
		NamespaceName:    zmeta.Namespace,
		InstanceClassGVK: zicv1.GroupVersionKind,
	})
}

func validClusterObjects() []runtime.Object {
	return []runtime.Object{
		credentialSecret("admin@internal", "password"),
		nodeGroupObject("master", cpapi.NodeTypeCloudPermanent),
		instanceClassObject(masterInstanceClassName, true),
	}
}

func nodeGroupObject(name string, nodeType cpapi.NodeType) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(nodeGroupGVK)
	obj.SetName(name)

	spec := map[string]any{"nodeType": string(nodeType)}
	if name == "master" {
		spec["cloudInstances"] = map[string]any{
			"classReference": map[string]any{
				"kind": zicv1.ZvirtInstanceClassKind,
				"name": masterInstanceClassName,
			},
		}
	}
	obj.Object["spec"] = spec

	return obj
}

func staticNodeGroupObject(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(nodeGroupGVK)
	obj.SetName(name)
	obj.Object["spec"] = map[string]any{"nodeType": "Static"}

	return obj
}

// malformedNodeGroupObject has a spec of the wrong shape, so decoding it into a NodeGroup fails.
func malformedNodeGroupObject() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(nodeGroupGVK)
	obj.SetName("malformed")
	obj.Object["spec"] = "this should have been an object"

	return obj
}

// foreignClassNodeGroupObject is a well-formed NodeGroup belonging to another cloud provider.
func foreignClassNodeGroupObject() *unstructured.Unstructured {
	obj := staticNodeGroupObject("foreign")
	obj.Object["spec"] = map[string]any{
		"nodeType": "CloudEphemeral",
		"cloudInstances": map[string]any{
			"classReference": map[string]any{"kind": "YandexInstanceClass", "name": "worker"},
		},
	}

	return obj
}

func instanceClassObject(name string, withEtcdDisk bool) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(zicv1.GroupVersionKind)
	obj.SetName(name)

	spec := map[string]any{
		"numCPUs":        int64(4),
		"memory":         int64(8192),
		"template":       "debian-bookworm",
		"vnicProfileID":  "0000-1111",
		"rootDiskSizeGb": int64(50),
	}
	if withEtcdDisk {
		spec["etcdDiskSizeGb"] = int64(10)
	}
	obj.Object["spec"] = spec

	return obj
}

// moduleConfigObject builds the module's ModuleConfig with the given connection settings. An empty
// caBundle is left out entirely, which is how a configuration that trusts the system store looks.
func moduleConfigObject(name, server, caBundle string, insecure bool) *unstructured.Unstructured {
	providerParameters := map[string]any{
		"server":    server,
		"clusterID": "c4bf82a5-b803-40c3-9f6c-b9398378f424",
	}
	if caBundle != "" {
		providerParameters["caBundle"] = caBundle
	}
	if insecure {
		providerParameters["insecure"] = true
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(moduleConfigGVK)
	obj.SetName(name)
	obj.Object["spec"] = map[string]any{
		"enabled": true,
		"version": int64(2),
		"settings": map[string]any{
			"provider": map[string]any{"parameters": providerParameters},
			"nodes": map[string]any{
				"parameters": map[string]any{"sshPublicKey": "ssh-rsa AAAA", "layout": "Standard"},
			},
		},
	}

	return obj
}

// testCABundle generates a real CA certificate: the rule parses it and insists it is a CA.
func testCABundle(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "zvirt-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	return base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func credentialSecret(identity, secret string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cpapi.CredentialSecretName,
			Namespace: zmeta.Namespace,
		},
		Type: cpapi.CredentialsSecretType,
		StringData: map[string]string{
			cpapi.CredentialSecretAuthSchemeKey: string(cpapi.AuthSchemeUserPassword),
			cpapi.CredentialSecretIdentityKey:   identity,
			cpapi.CredentialSecretSecretKey:     secret,
		},
	}
}
