package hooks_configuration

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestHooksConfiguration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Hooks configuration tests", func() {
	hooks, err := GetAllHooks()
	Context("hooks discovery", func() {
		It("", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(hooks).ToNot(HaveLen(0))
		})
	})

	hooksCH := make(chan Hook, len(hooks))
	Context("run", func() {
		for _, hook := range hooks {
			hooksCH <- hook
			It(hook.Path, func() {
				ithook := <-hooksCH

				By("Hook file should be executable", func() {
					Expect(ithook.Executable).To(BeTrue())
				})

				err := ithook.ExecuteGetConfig()
				By(ithook.Path+" --config must not fail", func() {
					Expect(err).ToNot(HaveOccurred())

				})

				By("keepFullObjectsInMemory is mandatory for kubernetes entries", func() {
					if ithook.HookConfig.Get("kubernetes").Exists() {
						kubernetesEntries := ithook.HookConfig.Get("kubernetes").Array()
						for _, value := range kubernetesEntries {
							Expect(value.Get("keepFullObjectsInMemory").Exists()).To(BeTrue())
						}
					}
				})
			})
		}
	})
})
