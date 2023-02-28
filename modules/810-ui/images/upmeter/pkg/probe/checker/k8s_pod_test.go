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
	"testing"

	"d8.io/upmeter/pkg/kubernetes"
)

func createTestProbeImage(name string, secrets []string) *kubernetes.ProbeImage {
	return kubernetes.NewProbeImage(&kubernetes.ProbeImageConfig{
		Name:        name,
		PullSecrets: secrets,
	})
}

func Test_GettingWithDefaultImage(t *testing.T) {
	var (
		podName          = "pod-1"
		nodeName         = "test1"
		initialImageName = "anyof:3.14"
	)

	image := createTestProbeImage(initialImageName, nil)
	pod := createPodObject(podName, nodeName, "123", image)

	if len(pod.Spec.ImagePullSecrets) != 0 {
		t.Errorf("expected empty pull secrets, got %v", pod.Spec.ImagePullSecrets)
	}

	imageName := pod.Spec.Containers[0].Image
	if imageName != initialImageName {
		t.Errorf("expected image to fallback to %q, got %q", initialImageName, image)
	}
}

func Test_GettingWithPassedImage(t *testing.T) {
	var (
		expectedImage   = "my.private.registry.com/alpine:latest"
		nodeName        = "test1"
		podName         = "pod-1"
		oneSecret       = []string{"secret1"}
		multipleSecrets = []string{"secret1", "secret2"}
	)

	cases := []struct {
		image           *kubernetes.ProbeImage
		expectedImage   string
		expectedSecrets []string
		caseName        string
	}{
		{
			image:           createTestProbeImage(expectedImage, oneSecret),
			expectedImage:   expectedImage,
			expectedSecrets: oneSecret,
			caseName:        "Image with one secret",
		},
		{
			image:           createTestProbeImage(expectedImage, multipleSecrets),
			expectedImage:   expectedImage,
			expectedSecrets: multipleSecrets,
			caseName:        "Image with multiple secrets",
		},
	}

	for _, c := range cases {
		pod := createPodObject(podName, nodeName, "123", c.image)

		image := pod.Spec.Containers[0].Image
		if image != expectedImage {
			t.Errorf("%s: incorrect pod image. Got %v", c.caseName, image)
		}

		pullSecrets := pod.Spec.ImagePullSecrets
		if len(pullSecrets) != len(c.expectedSecrets) {
			t.Errorf("%s: image pull secrets has not equal len's. Got %v", c.caseName, len(pullSecrets))
		}

		for i, expectedSecret := range c.expectedSecrets {
			name := pullSecrets[i].Name
			if name != expectedSecret {
				t.Errorf("%s: image pull secret not equal with expected. Got %v", c.caseName, name)
			}
		}
	}
}
