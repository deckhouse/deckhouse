package nodegroupbundle

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestNodegroupbundle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Nodegroupbundle Suite")
}

type testStorageParams struct {
	k8sVer string
}

type testStorageFixture struct {
	storage *StorageWithK8sBundles
}

func setupTestStorage(p *testStorageParams) *testStorageFixture {

	ctx := &testTemplateContext{
		returnedVal: map[string]interface{}{
			"kubernetesVersion": p.k8sVer,
		},
	}

	storage, err := NewStorage("", ctx)
	Expect(err).To(BeNil())

	return &testStorageFixture{
		storage: storage,
	}
}

var _ = Describe("Module :: node-manager :: bashible-apiserver :: ng bundles storage", func() {
	Context("internal methods", func() {
		It("returns valid k8s bundle name", func() {
			fixtures := setupTestStorage(&testStorageParams{
				k8sVer: "1.19",
			})

			name, err := fixtures.storage.getK8sBundleName("ubuntu-lts.master")

			Expect(err).To(BeNil())
			Expect(name).To(BeEquivalentTo("ubuntu-lts.1-19"))
		})
	})

})

type testTemplateContext struct {
	returnedVal map[string]interface{}
	errVal      error
}

func (c *testTemplateContext) Get(_ string) (map[string]interface{}, error) {
	return c.returnedVal, c.errVal
}
