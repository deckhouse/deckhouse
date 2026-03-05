/*
Copyright 2026 Flant JSC

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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	serializeryaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	"d8.io/upmeter/pkg/kubernetes"
)

func namespaceExists(ctx context.Context, access kubernetes.Access, namespace string) (bool, error) {
	_, err := access.Kubernetes().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func waitNamespaceAbsent(ctx context.Context, access kubernetes.Access, namespace string, timeout, interval time.Duration) error {
	return waitForCondition(timeout, interval, func() (bool, error) {
		_, err := access.Kubernetes().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	})
}

// queryEndpoint sends an authenticated GET request to the endpoint via insecure client.
func queryEndpoint(access kubernetes.Access, endpoint string, timeout time.Duration) ([]byte, error) {
	return queryEndpointWithHeaders(access, endpoint, timeout, nil)
}

// queryEndpointWithHeaders sends an authenticated GET request with custom headers.
func queryEndpointWithHeaders(
	access kubernetes.Access,
	endpoint string,
	timeout time.Duration,
	headers map[string]string,
) ([]byte, error) {
	req, err := newGetRequestWithHeaders(endpoint, access.ServiceAccountToken(), access.UserAgent(), headers)
	if err != nil {
		return nil, err
	}

	client := newInsecureClient(3 * timeout)
	body, reqErr := doRequest(client, req)
	if reqErr != nil {
		return nil, reqErr
	}

	return body, nil
}

// decodeManifestToUnstructured parses a YAML manifest into an Unstructured object.
func decodeManifestToUnstructured(manifest string) (*unstructured.Unstructured, error) {
	dec := serializeryaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}

	if _, _, err := dec.Decode([]byte(manifest), nil, obj); err != nil {
		return nil, err
	}

	return obj, nil
}

func fallbackDuration(actual, fallback time.Duration) time.Duration {
	if actual <= 0 {
		return fallback
	}
	return actual
}

func fallbackString(actual, fallback string) string {
	if actual == "" {
		return fallback
	}
	return actual
}
