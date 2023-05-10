package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var _ = Describe("Kubernetes objects", func() {
	var (
		clientset     *kubernetes.Clientset
		dynamicClient dynamic.Interface
	)

	BeforeEach(func() {
		// get default config for Kubernetes
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

		// create client for Kubernetes
		config, err := clientConfig.ClientConfig()
		Expect(err).NotTo(HaveOccurred())
		dynamicClient, err = dynamic.NewForConfig(config)
		Expect(err).NotTo(HaveOccurred())
		clientset, err = kubernetes.NewForConfig(config)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when applying YAML manifests", func() {
		It("should create ConfigMap", func() {

			configMapYAML := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-configmap
  namespace: default
data:
  key: value
`

			// Apply YAML manifests in cluster
			Expect(ApplyYAML(dynamicClient, configMapYAML)).To(Succeed())

			// Check that YAML-manifests were created
			_, err := clientset.CoreV1().ConfigMaps("default").Get(context.Background(), "my-configmap", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			// Delete created YAML manifest
			err = clientset.CoreV1().ConfigMaps("default").Delete(context.Background(), "my-configmap", metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func ApplyYAML(client dynamic.Interface, yamlStream string) error {
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewBufferString(yamlStream), 4096)

	for {
		obj := &unstructured.Unstructured{}
		if err := decoder.Decode(obj); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error decoding YAML: %v", err)
		}

		// Get GVK of object
		gvr := GetGroupVersionResource(obj)

		// Get Dynamic Resource Interface for this GVK
		resource := client.Resource(gvr)

		// Apply object using "apply" mechanism
		_, err := resource.
			Namespace(obj.GetNamespace()).
			Apply(context.TODO(), obj.GetName(), obj, metav1.ApplyOptions{
				Force:        true,
				FieldManager: "d8-integration-test",
			})
		if err != nil {
			return fmt.Errorf("failed to apply object: %v", err)
		}

	}
	return nil
}

func GetGroupVersionResource(obj *unstructured.Unstructured) schema.GroupVersionResource {
	gvk := obj.GroupVersionKind()
	apiVersion, kind := gvk.ToAPIVersionAndKind()
	version := apiVersion
	if strings.Contains(version, "/") {
		version = strings.Split(apiVersion, "/")[1]
	}
	resource := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  version,
		Resource: strings.ToLower(kind) + "s",
	}
	return resource
}
