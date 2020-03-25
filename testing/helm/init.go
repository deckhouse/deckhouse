package helm

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/flant/shell-operator/pkg/utils/manifest/releaseutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/testing/library"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
	"github.com/deckhouse/deckhouse/testing/library/sandbox_runner"
	"github.com/deckhouse/deckhouse/testing/library/values_store"
)

var (
	globalTmpDir string
)

type HelmConfig struct {
	modulePath  string
	objectStore object_store.ObjectStore
	values      *values_store.ValuesStore
	Session     *gexec.Session
}

func (hec HelmConfig) ValuesGet(path string) library.KubeResult {
	return hec.values.Get(path)
}

func (hec *HelmConfig) ValuesSet(path string, value interface{}) {
	hec.values.SetByPath(path, value)
}

func (hec *HelmConfig) ValuesSetFromYaml(path, value string) {
	hec.values.SetByPathFromYaml(path, []byte(value))
}

func (hec *HelmConfig) KubernetesGlobalResource(kind, name string) object_store.KubeObject {
	return hec.objectStore.KubernetesGlobalResource(kind, name)
}

func (hec *HelmConfig) KubernetesResource(kind, namespace, name string) object_store.KubeObject {
	return hec.objectStore.KubernetesResource(kind, namespace, name)
}

func SetupHelmConfig(values string) *HelmConfig {
	_, path, _, ok := runtime.Caller(1)
	if !ok {
		panic("can't execute runtime.Caller")
	}

	modulePath := filepath.Dir(filepath.Dir(path))

	initialValues, err := library.InitValues(modulePath, []byte(values))
	if err != nil {
		panic(err)
	}

	config := new(HelmConfig)
	config.modulePath = modulePath

	initialValuesJson, err := json.Marshal(initialValues)
	if err != nil {
		panic(err)
	}

	BeforeEach(func() {
		config.values = values_store.NewStoreFromRawJson(initialValuesJson)

		// set some common values
		config.values.SetByPath("global.discovery.clusterVersion", "1.17.0")
		config.values.SetByPath("global.modulesImages.registry", "registry.example.com")
	})

	globalTmpDir, err = ioutil.TempDir("", "")
	if err != nil {
		panic(err)
	}

	return config
}

func (hec *HelmConfig) HelmRender() {
	hec.objectStore = make(object_store.ObjectStore)

	helmCmd := exec.Command("helm", "template")
	tempDir, err := ioutil.TempDir(globalTmpDir, "")
	Expect(err).ToNot(HaveOccurred())

	helmCmd.Dir = tempDir
	helmCmd.Args = append(helmCmd.Args, ".")

	yamlValuesBytes := hec.values.GetAsYaml()

	hec.Session = sandbox_runner.Run(helmCmd,
		sandbox_runner.WithSourceDirectory(hec.modulePath, tempDir),
		sandbox_runner.WithFile(filepath.Join(tempDir, "values.yaml"), yamlValuesBytes),
	)

	for _, doc := range releaseutil.SplitManifests(string(hec.Session.Out.Contents())) {
		var t interface{}
		err = yaml.Unmarshal([]byte(doc), &t)
		if t == nil {
			continue
		}

		Expect(err).To(Not(HaveOccurred()))
		Expect(t).To(BeAssignableToTypeOf(map[string]interface{}{}))

		var unstructuredObj unstructured.Unstructured
		unstructuredObj.SetUnstructuredContent(t.(map[string]interface{}))

		hec.objectStore.PutObject(unstructuredObj.Object, object_store.NewMetaIndex(
			unstructuredObj.GetKind(),
			unstructuredObj.GetNamespace(),
			unstructuredObj.GetName(),
		))
	}
}

var _ = AfterSuite(func() {
	By("Removing temporary directories")

	Expect(os.RemoveAll(globalTmpDir)).Should(Succeed())
})
