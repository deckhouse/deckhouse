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
	nodeName := "test1"

	image := createTestProbeImage("", nil)
	pod := createPodObject(nodeName, image)

	if len(pod.Spec.ImagePullSecrets) != 0 {
		t.Errorf("pod without image env vars has none empty ImagePullSecrets. Got %v", pod.Spec.ImagePullSecrets)
	}

	imageName := pod.Spec.Containers[0].Image
	if imageName != kubernetes.DefaultAlpineImage {
		t.Errorf(
			"container pod without image env vars must be alpine(%s). Got: %v",
			kubernetes.DefaultAlpineImage,
			image,
		)
	}
}

func Test_GettingWithPassedImage(t *testing.T) {
	const expectedImage = "my.private.registry.com/alpine:latest"
	const nodeName = "test1"

	oneSecret := []string{"secret1"}
	multipleSecrets := []string{"secret1", "secret2"}

	cases := []struct {
		image *kubernetes.ProbeImage

		expectedImage   string
		expectedSecrets []string

		caseName string
	}{
		{
			image: createTestProbeImage(expectedImage, oneSecret),

			expectedImage:   expectedImage,
			expectedSecrets: oneSecret,

			caseName: "Image with one secret",
		},

		{
			image: createTestProbeImage(expectedImage, multipleSecrets),

			expectedImage:   expectedImage,
			expectedSecrets: multipleSecrets,

			caseName: "Image with multiple secrets",
		},
	}

	for _, c := range cases {
		pod := createPodObject(nodeName, c.image)
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
