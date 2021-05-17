package hooks

import (
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: deckhouse_version ", func() {
	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)

	Context("Unknown deckhouse version", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.deckhouseVersion").String()).To(Equal(`unknown`))
		})
	})

	Context("With number version", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			err := writeVersionTMPFile("21.01")
			Expect(err).To(BeNil())
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.deckhouseVersion").String()).To(Equal(`21.01`))
		})
	})

	Context("With string version", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			err := writeVersionTMPFile("21.01-hotfix1")
			Expect(err).To(BeNil())
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.deckhouseVersion").String()).To(Equal(`21.01-hotfix1`))
		})
	})
})

func writeVersionTMPFile(content string) error {
	tmpfile, err := ioutil.TempFile("", "deckhouse-version-*")
	if err != nil {
		return err
	}
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		return err
	}

	return os.Setenv("D8_VERSION_TMP_FILE", tmpfile.Name())
}
