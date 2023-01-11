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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: deckhouse :: hooks :: images_copier ::", func() {
	f := HookExecutionConfigInit(`
global:
  deckhouseVersion: "12345"
  modulesImages:
    registry:
      dockercfg: eyJhdXRocyI6IHsicmVnaXN0cnkuZGVja2hvdXNlLmlvIjogeyJhdXRoIjogImRUcHdZWE56Q2c9PSJ9fX0=
      base: registry.deckhouse.io/deckhouse/fe
      address: registry.deckhouse.io
    tags:
      module_1:
        image_1: "image-1-latest"
        image_2: "image-2-v1.0"
      module_2:
        image_3: "image-3-latest"
      deckhouse:
        imagesCopier: "image-copier-latest"
deckhouse:
  internal:
    currentReleaseImageName: "registry.deckhouse.io/deckhouse/fe:rock-solid"
`, `{}`)

	const copierSecret = `
apiVersion: v1
kind: Secret
metadata:
  name: images-copier-config
  namespace: d8-system
  annotations:
    first: val
data:
  # {"username":"abc","password":"xxxxxxxxx","insecure":true,"image":"my.repo.io/deckhouse:rock-solid-1.24.17"}
  dest-repo.json: eyJ1c2VybmFtZSI6ImFiYyIsInBhc3N3b3JkIjoieHh4eHh4eHh4IiwiaW5zZWN1cmUiOnRydWUsImltYWdlIjoibXkucmVwby5pby9kZWNraG91c2U6cm9jay1zb2xpZC0xLjI0LjE3In0=
`

	const copierFullSecret = `
apiVersion: v1
kind: Secret
metadata:
  annotations:
    first: val
  creationTimestamp: null
  name: images-copier-config
  namespace: d8-system
data:
  # {"username":"abc","password":"xxxxxxxxx","insecure":true,"image":"my.repo.io/deckhouse:rock-solid-1.24.17"}
  dest-repo.json: eyJ1c2VybmFtZSI6ImFiYyIsInBhc3N3b3JkIjoieHh4eHh4eHh4IiwiaW5zZWN1cmUiOnRydWUsImltYWdlIjoibXkucmVwby5pby9kZWNraG91c2U6cm9jay1zb2xpZC0xLjI0LjE3In0=

  # {"deckhouse":{"imagesCopier":"image-copier-latest"},"module_1":{"image_1":"image-1-latest","image_2":"image-2-v1.0"},"module_2":{"image_3":"image-3-latest"}}
  d8-images.json: eyJkZWNraG91c2UiOnsiaW1hZ2VzQ29waWVyIjoiaW1hZ2UtY29waWVyLWxhdGVzdCJ9LCJtb2R1bGVfMSI6eyJpbWFnZV8xIjoiaW1hZ2UtMS1sYXRlc3QiLCJpbWFnZV8yIjoiaW1hZ2UtMi12MS4wIn0sIm1vZHVsZV8yIjp7ImltYWdlXzMiOiJpbWFnZS0zLWxhdGVzdCJ9fQ==

  # {"username":"u","password":"pass","insecure":false,"image":"registry.deckhouse.io/deckhouse/fe:rock-solid"}
  d8-repo.json: eyJ1c2VybmFtZSI6InUiLCJwYXNzd29yZCI6InBhc3MiLCJpbnNlY3VyZSI6ZmFsc2UsImltYWdlIjoicmVnaXN0cnkuZGVja2hvdXNlLmlvL2RlY2tob3VzZS9mZTpyb2NrLXNvbGlkIn0=
`
	const copierJob = `
apiVersion: batch/v1
kind: Job
metadata:
  annotations:
    first: val
  creationTimestamp: "2021-03-18T14:00:08Z"
  labels:
    app: d8-images-copier
    heritage: deckhouse
  name: copy-images
  namespace: d8-system
spec:
  backoffLimit: 0
  template:
    metadata:
      annotations:
        first: val
      creationTimestamp: "2021-03-18T14:00:08Z"
      labels:
        app: d8-images-copier
        heritage: deckhouse
      namespace: d8-system
    spec:
      containers:
      - command:
        - copy-images
        - -c
        - /config/
        image: registry.deckhouse.io/deckhouse/fe:image-copier-latest
        imagePullPolicy: Always
        name: image-copier
        resources: {}
        volumeMounts:
        - mountPath: /config/
          name: config
          readOnly: true
      imagePullSecrets:
      - name: deckhouse-registry
      restartPolicy: Never
      volumes:
      - name: config
        secret:
          defaultMode: 256
          secretName: images-copier-config
status: {}
`
	assertFullConfigSecret := func(f *HookExecutionConfig) {
		secret := f.KubernetesResource("Secret", "d8-system", copierConfSecretName)
		Expect(secret.Exists()).To(BeTrue())
		var expectedSecret map[string]interface{}

		err := yaml.UnmarshalStrict([]byte(copierFullSecret), &expectedSecret)
		Expect(err).ToNot(HaveOccurred())

		Expect(secret["data"]).To(Equal(expectedSecret["data"]))
	}

	assertCopierJobIsValid := func(f *HookExecutionConfig) {
		job := f.KubernetesResource("Job", "d8-system", copierJobName)
		Expect(job.Exists()).To(BeTrue())

		secret := f.KubernetesResource("Secret", "d8-system", copierConfSecretName)
		Expect(secret.Exists()).To(BeTrue())

		// annotation same as in secret
		secretAnnotations := secret.Field(`metadata.annotations`).Value()
		jobsAnnotations := job.Field(`metadata.annotations`).Value()
		Expect(secretAnnotations).To(Equal(jobsAnnotations))

		// set correct container image
		copierImage := job.Field(`spec.template.spec.containers.0.image`).String()
		Expect(copierImage).To(Equal("registry.deckhouse.io/deckhouse/fe:image-copier-latest"))

		// mount image copier secret to pod
		volume := job.Field(`spec.template.spec.volumes.0`)
		volumeSecretName := volume.Get("secret.secretName").String()
		Expect(volumeSecretName).To(Equal(copierConfSecretName))

		// mount image copier volume to container
		volumeMountName := volume.Get("name").String()
		containerVolumeMountName := job.Field(`spec.template.spec.containers.0.volumeMounts.0.name`).String()
		Expect(volumeMountName).To(Equal(containerVolumeMountName))
	}

	Context("Empty cluster", func() {
		It("Hook run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Copier secret config added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(copierSecret))
				f.RunHook()
			})

			It("Should add to secret deckhouse images, release channel, registry auth, and does not change dest registry conf", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertFullConfigSecret(f)
			})

			It("creates images copier job", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertCopierJobIsValid(f)
			})

		})
	})

	genJobState := func(newStatus map[string]interface{}) string {
		var job map[string]interface{}
		err := yaml.Unmarshal([]byte(copierJob), &job)
		Expect(err).ToNot(HaveOccurred())

		job["status"] = newStatus
		resJob, _ := yaml.Marshal(job)

		return string(resJob)
	}

	genFullSecretState := func(newSecretAnnotations map[string]string) string {
		secret := copierFullSecret

		if newSecretAnnotations == nil {
			return secret
		}

		s := map[string]interface{}{}
		err := yaml.Unmarshal([]byte(secret), &s)
		Expect(err).ToNot(HaveOccurred())

		u := unstructured.Unstructured{Object: s}
		u.SetAnnotations(newSecretAnnotations)

		sBytes, err := yaml.Marshal(u.Object)
		Expect(err).ToNot(HaveOccurred())

		return string(sBytes)
	}

	genJobPodState := func(name string) string {
		obj := &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: copierNs,
				Labels:    copierLabels(),
			},
		}

		str, err := yaml.Marshal(obj)
		if err != nil {
			panic(err)
		}

		return string(str)
	}

	Context("Job with full secret config", func() {
		BeforeEach(func() {
			JoinKubeResourcesAndSet(f,
				genJobState(map[string]interface{}{}),
				genFullSecretState(nil),
			)

			f.RunHook()
		})

		It("does not changing anything", func() {
			Expect(f).To(ExecuteSuccessfully())

			assertFullConfigSecret(f)
			assertCopierJobIsValid(f)
		})

		Context("Pod created", func() {
			const podName = "pod-1"
			BeforeEach(func() {
				JoinKubeResourcesAndSet(f,
					genJobState(map[string]interface{}{}),
					genFullSecretState(nil),
					genJobPodState(podName),
				)

				f.RunHook()
			})

			It("does not changing anything", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertFullConfigSecret(f)
				assertCopierJobIsValid(f)
			})

			Context("Job running", func() {
				BeforeEach(func() {
					JoinKubeResourcesAndSet(f,
						genJobState(map[string]interface{}{
							"active": 1,
						}),

						genFullSecretState(nil),
						genJobPodState(podName),
					)

					f.RunHook()
				})

				It("does not changing anything", func() {
					Expect(f).To(ExecuteSuccessfully())

					assertFullConfigSecret(f)
					assertCopierJobIsValid(f)
				})
			})

			Context("Job finished successfully", func() {
				BeforeEach(func() {
					JoinKubeResourcesAndSet(f,
						genJobState(map[string]interface{}{
							"succeeded": 1,
						}),

						genFullSecretState(nil),
						genJobPodState(podName),
					)

					f.RunHook()
				})

				It("should remove secret and job and job pods", func() {
					Expect(f).To(ExecuteSuccessfully())

					job := f.KubernetesResource("Job", "d8-system", copierJobName)
					Expect(job.Exists()).To(BeFalse())

					secret := f.KubernetesResource("Secret", "d8-system", copierConfSecretName)
					Expect(secret.Exists()).To(BeFalse())

					pod := f.KubernetesResource("Pod", "d8-system", podName)
					Expect(pod.Exists()).To(BeFalse())
				})
			})

			Context("Job failed", func() {
				BeforeEach(func() {
					JoinKubeResourcesAndSet(f,
						genJobState(map[string]interface{}{
							"failed": 1,
						}),

						genFullSecretState(nil),
						genJobPodState(podName),
					)

					f.RunHook()
				})

				It("does not changing anything", func() {
					Expect(f).To(ExecuteSuccessfully())

					assertFullConfigSecret(f)
					assertCopierJobIsValid(f)

					pod := f.KubernetesResource("Pod", "d8-system", podName)
					Expect(pod.Exists()).To(BeTrue())
				})

				Context("change annotation in secret", func() {
					var curCreationTimestamp string

					BeforeEach(func() {
						job := f.KubernetesResource("Job", "d8-system", copierJobName)
						curCreationTimestamp = job.Field("metadata.creationTimestamp").String()

						JoinKubeResourcesAndSet(f,
							genJobState(map[string]interface{}{
								"failed": 1,
							}),

							genFullSecretState(map[string]string{
								"need-restart": "1",
							}),

							genJobPodState(podName),
						)

						f.RunHook()
					})

					It("should restart job", func() {
						assertFullConfigSecret(f)
						assertCopierJobIsValid(f)

						job := f.KubernetesResource("Job", "d8-system", copierJobName)
						Expect(job.Field("metadata.creationTimestamp")).ToNot(Equal(curCreationTimestamp))

						pod := f.KubernetesResource("Pod", "d8-system", podName)
						Expect(pod.Exists()).To(BeFalse())
					})
				})
			})
		})

		Context("remove secret", func() {
			const podName = "pod-2"
			DescribeTable("should remove job and pods", func(newState map[string]interface{}) {
				JoinKubeResourcesAndSet(f,
					genJobState(newState),
					genJobPodState(podName),
				)

				f.RunHook()

				Expect(f).To(ExecuteSuccessfully())

				job := f.KubernetesResource("Job", "d8-system", copierJobName)
				Expect(job.Exists()).To(BeFalse())

				pod := f.KubernetesResource("Pod", "d8-system", podName)
				Expect(pod.Exists()).To(BeFalse())
			},
				Entry("running", map[string]interface{}{
					"active": 1,
				}),

				Entry("failed", map[string]interface{}{
					"failed": 1,
				}),

				// this case example. Deckhouse was restarted before cleanup execution
				// after restart we need cleanup job
				Entry("succeeded", map[string]interface{}{
					"succeeded": 1,
				}),
			)
		})
	})

	Context("job only in cluster", func() {
		DescribeTable("should remove job", func(newState map[string]interface{}) {
			JoinKubeResourcesAndSet(f,
				genJobState(newState),
			)

			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			job := f.KubernetesResource("Job", "d8-system", copierJobName)
			Expect(job.Exists()).To(BeFalse())
		},
			Entry("running", map[string]interface{}{
				"active": 1,
			}),

			Entry("failed", map[string]interface{}{
				"failed": 1,
			}),

			// this case example. Deckhouse was restarted before cleanup execution
			// after restart we need cleanup job
			Entry("succeeded", map[string]interface{}{
				"succeeded": 1,
			}),
		)
	})
})
