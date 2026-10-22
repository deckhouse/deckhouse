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

package template_tests

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	. "github.com/deckhouse/deckhouse/testing/helm"
)

const securityPolicyExceptionLabel = "security.deckhouse.io/security-policy-exception"

// The pods of this implementation against the Pod Security Standards.
//
// Every pod either stays within the restricted standard or names a SecurityPolicyException,
// and the exception has to match the pod exactly: admission-policy-engine excepts a hostPath
// only when both the path and readOnly equal the mount's, and a host-networked pod's every
// containerPort is checked as a host port. A mismatch shows only once the policy enforces,
// as a pod that is refused — and the pods here are the ones the cluster pulls its images
// through. So what is compared below is the pod as rendered against the exception as
// rendered, item by item, in both directions: an entry that no longer matches any mount is
// a permission nobody uses.
var _ = Describe("Module :: registry :: helm template :: v2 security policy exceptions", func() {
	f := SetupHelmConfig(``)

	renderWith := func(apiVersions string, storePath string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSetFromYaml("global.discovery.apiVersions", apiVersions)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", v2Enabled)
		if storePath != "" {
			f.ValuesSet("registry.internal.v2.storePath", storePath)
		}
		f.HelmRender()
	}

	const allKinds = `
- deckhouse.io/v1alpha1/NodeStaticPodRequest
- deckhouse.io/v1alpha1/SecurityPolicyException
`

	storagePod := func() corev1.PodTemplateSpec {
		sts := f.KubernetesResource("StatefulSet", "d8-system", "registry-storage")
		Expect(sts.Exists()).To(BeTrue())

		var template corev1.PodTemplateSpec
		Expect(json.Unmarshal([]byte(sts.Field("spec.template").String()), &template)).To(Succeed())
		return template
	}

	controllerPod := func() corev1.PodTemplateSpec {
		deployment := f.KubernetesResource("Deployment", "d8-system", "registry-controller")
		Expect(deployment.Exists()).To(BeTrue())

		var template corev1.PodTemplateSpec
		Expect(json.Unmarshal([]byte(deployment.Field("spec.template").String()), &template)).To(Succeed())
		return template
	}

	enginePod := func() corev1.Pod {
		request := f.KubernetesGlobalResource("NodeStaticPodRequest", "registry-agent")
		Expect(request.Exists()).To(BeTrue())

		var pod corev1.Pod
		Expect(yaml.Unmarshal([]byte(request.Field("spec.manifest").String()), &pod)).To(Succeed())
		return pod
	}

	exception := func(namespace, name string) securityPolicyException {
		object := f.KubernetesResource("SecurityPolicyException", namespace, name)
		Expect(object.Exists()).To(BeTrue(), "no SecurityPolicyException %s/%s", namespace, name)

		var spe securityPolicyException
		Expect(json.Unmarshal([]byte(object.Field("spec").String()), &spe)).To(Succeed())
		return spe
	}

	Context("a cluster that has the kind", func() {
		BeforeEach(func() { renderWith(allKinds, "") })

		It("excepts the storage exactly as far as it needs", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			pod := storagePod()
			Expect(pod.Labels).To(HaveKeyWithValue(securityPolicyExceptionLabel, "registry-storage"))
			expectExceptionMatches(pod.Spec, exception("d8-system", "registry-storage"))
		})

		It("excepts the agent on an Engine node exactly as far as it needs", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			pod := enginePod()
			Expect(pod.Namespace).To(Equal("kube-system"), "an exception is looked up in the pod's namespace")
			Expect(pod.Labels).To(HaveKeyWithValue(securityPolicyExceptionLabel, "registry-agent-engine"))
			expectExceptionMatches(pod.Spec, exception("kube-system", "registry-agent-engine"))
		})

		It("renders the exception for the agent on a bashible node", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// Its contents are checked against the static pod the bashible step writes, in
			// bashible_tests, where that step is rendered.
			Expect(f.KubernetesResource("SecurityPolicyException", "kube-system", "registry-agent").Exists()).To(BeTrue())
		})

		It("leaves the controller within the restricted standard", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			pod := controllerPod()
			Expect(pod.Labels).NotTo(HaveKey(securityPolicyExceptionLabel))
			expectExceptionMatches(pod.Spec, securityPolicyException{})
		})
	})

	// The path decided by the hook for a master with a read-only root filesystem. The store
	// mounts it, so the exception has to name it too.
	Context("masters with a read-only root filesystem", func() {
		BeforeEach(func() { renderWith(allKinds, constant.StorePathImmutable) })

		It("excepts the store where it actually is", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			expectExceptionMatches(storagePod().Spec, exception("d8-system", "registry-storage"))
		})
	})

	// Deckhouse with admission-policy-engine disabled has no such kind, and an object of an
	// unknown kind fails the whole release.
	Context("a cluster without the kind", func() {
		BeforeEach(func() { renderWith(`["deckhouse.io/v1alpha1/NodeStaticPodRequest"]`, "") })

		It("renders the workloads without the exceptions", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("StatefulSet", "d8-system", "registry-storage").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("NodeStaticPodRequest", "registry-agent").Exists()).To(BeTrue())
			for _, name := range [][2]string{
				{"d8-system", "registry-storage"},
				{"kube-system", "registry-agent"},
				{"kube-system", "registry-agent-engine"},
			} {
				Expect(f.KubernetesResource("SecurityPolicyException", name[0], name[1]).Exists()).To(BeFalse(), name[1])
			}
		})
	})
})

// securityPolicyException is the part of the kind these pods use.
type securityPolicyException struct {
	SecurityContext struct {
		RunAsNonRoot *struct {
			AllowedValue bool `json:"allowedValue"`
		} `json:"runAsNonRoot"`
		RunAsUser *struct {
			AllowedValues []int64 `json:"allowedValues"`
		} `json:"runAsUser"`
	} `json:"securityContext"`
	Network struct {
		HostNetwork *struct {
			AllowedValue bool `json:"allowedValue"`
		} `json:"hostNetwork"`
		HostPorts []struct {
			Port     int32  `json:"port"`
			Protocol string `json:"protocol"`
		} `json:"hostPorts"`
	} `json:"network"`
	Volumes struct {
		Types *struct {
			AllowedValues []string `json:"allowedValues"`
		} `json:"types"`
		HostPath *struct {
			AllowedValues []struct {
				Path     string `json:"path"`
				ReadOnly bool   `json:"readOnly"`
			} `json:"allowedValues"`
		} `json:"hostPath"`
	} `json:"volumes"`
}

// expectExceptionMatches holds a pod to the restricted standard, less what the exception
// allows, and the exception to what the pod uses. An empty exception is a pod that must need
// none.
func expectExceptionMatches(spec corev1.PodSpec, spe securityPolicyException) {
	// What restricted asks of every container and the exceptions here never relax.
	for _, container := range spec.Containers {
		sc := container.SecurityContext
		Expect(sc).NotTo(BeNil(), "container %s states no security context", container.Name)
		Expect(sc.AllowPrivilegeEscalation).NotTo(BeNil(), "container %s", container.Name)
		Expect(*sc.AllowPrivilegeEscalation).To(BeFalse(), "container %s", container.Name)
		Expect(sc.Capabilities).NotTo(BeNil(), "container %s", container.Name)
		Expect(sc.Capabilities.Drop).To(ContainElement(corev1.Capability("ALL")), "container %s", container.Name)
		Expect(sc.Capabilities.Add).To(BeEmpty(), "container %s", container.Name)
		Expect(sc.Privileged == nil || !*sc.Privileged).To(BeTrue(), "container %s", container.Name)

		seccomp := sc.SeccompProfile
		if seccomp == nil && spec.SecurityContext != nil {
			seccomp = spec.SecurityContext.SeccompProfile
		}
		Expect(seccomp).NotTo(BeNil(), "container %s has no seccomp profile", container.Name)
		Expect(seccomp.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault), "container %s", container.Name)
	}

	// The user: root only where the exception says so.
	var usesRoot bool
	for _, container := range spec.Containers {
		var user *int64
		var nonRoot *bool
		if spec.SecurityContext != nil {
			user, nonRoot = spec.SecurityContext.RunAsUser, spec.SecurityContext.RunAsNonRoot
		}
		if sc := container.SecurityContext; sc != nil {
			if sc.RunAsUser != nil {
				user = sc.RunAsUser
			}
			if sc.RunAsNonRoot != nil {
				nonRoot = sc.RunAsNonRoot
			}
		}
		if user != nil && *user == 0 {
			usesRoot = true
		} else {
			Expect(nonRoot).NotTo(BeNil(), "container %s does not say it runs as non-root", container.Name)
			Expect(*nonRoot).To(BeTrue(), "container %s", container.Name)
		}
	}
	if usesRoot {
		Expect(spe.SecurityContext.RunAsUser).NotTo(BeNil(), "the pod runs as root with no exception for it")
		Expect(spe.SecurityContext.RunAsUser.AllowedValues).To(Equal([]int64{0}))
		Expect(spe.SecurityContext.RunAsNonRoot).NotTo(BeNil())
		Expect(spe.SecurityContext.RunAsNonRoot.AllowedValue).To(BeFalse())
	} else {
		Expect(spe.SecurityContext.RunAsUser).To(BeNil(), "an exception for root the pod does not use")
		Expect(spe.SecurityContext.RunAsNonRoot).To(BeNil(), "an exception for root the pod does not use")
	}

	// The network. On the host network every containerPort binds on the node.
	type port struct {
		number   int32
		protocol string
	}
	opened := map[port]bool{}
	for _, container := range spec.Containers {
		for _, p := range container.Ports {
			protocol := string(p.Protocol)
			if protocol == "" {
				protocol = "TCP"
			}
			if p.HostPort != 0 {
				opened[port{p.HostPort, protocol}] = true
			}
			if spec.HostNetwork {
				opened[port{p.ContainerPort, protocol}] = true
			}
		}
	}
	excepted := map[port]bool{}
	for _, p := range spe.Network.HostPorts {
		excepted[port{p.Port, p.Protocol}] = true
	}
	Expect(excepted).To(Equal(opened), "the exception's hostPorts are not the ports the pod opens on the node")

	if spec.HostNetwork {
		Expect(spe.Network.HostNetwork).NotTo(BeNil(), "the pod uses the host network with no exception for it")
		Expect(spe.Network.HostNetwork.AllowedValue).To(BeTrue())
	} else {
		Expect(spe.Network.HostNetwork).To(BeNil(), "an exception for the host network the pod does not use")
	}
	Expect(spec.HostPID).To(BeFalse())
	Expect(spec.HostIPC).To(BeFalse())

	// The volumes. readOnly is the mount's, as admission-policy-engine reads it: absent is false.
	type hostPath struct {
		path     string
		readOnly bool
	}
	mounted := map[hostPath]bool{}
	for _, volume := range spec.Volumes {
		if volume.HostPath == nil {
			continue
		}
		readOnly := map[bool]bool{}
		for _, container := range append(append([]corev1.Container{}, spec.InitContainers...), spec.Containers...) {
			for _, mount := range container.VolumeMounts {
				if mount.Name == volume.Name {
					readOnly[mount.ReadOnly] = true
				}
			}
		}
		// Two mounts of one volume that disagree cannot both be matched by an exception.
		Expect(len(readOnly)).To(BeNumerically("<=", 1), "volume %s is mounted both read-only and writable", volume.Name)
		mounted[hostPath{volume.HostPath.Path, readOnly[true]}] = true
	}
	exceptedPaths := map[hostPath]bool{}
	if spe.Volumes.HostPath != nil {
		for _, p := range spe.Volumes.HostPath.AllowedValues {
			exceptedPaths[hostPath{p.Path, p.ReadOnly}] = true
		}
	}
	Expect(exceptedPaths).To(Equal(mounted), "the exception's host paths are not the ones the pod mounts")

	if len(mounted) > 0 {
		Expect(spe.Volumes.Types).NotTo(BeNil(), "the pod mounts host paths with no exception for the volume type")
		Expect(spe.Volumes.Types.AllowedValues).To(Equal([]string{"hostPath"}))
	} else {
		Expect(spe.Volumes.Types).To(BeNil(), "an exception for a volume type the pod does not use")
	}
}
