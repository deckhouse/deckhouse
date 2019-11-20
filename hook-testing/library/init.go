package library

import (
	"encoding/json"
	jsonpatch "gopkg.in/evanphx/json-patch.v4"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	hook "github.com/flant/shell-operator/pkg/hook"
)

var	HookConfig = &HookExecutionConfig{}

type HookExecutionConfig struct {
	tmpDir string
	HookPath           string
	Values             string
	ConfigValues       string
	BindingContextsRaw string
}

func SetupHookExecutionConfig(values, configValues, bindingContextsRaw string) {
	_, filepath, _, ok := runtime.Caller(1)
	if !ok {
		panic("can't execute runtime.Caller")
	}
	HookConfig.HookPath = strings.TrimSuffix(filepath, "_test.go")

	var err error
	HookConfig.tmpDir, err = ioutil.TempDir("", "")
	if err != nil {
		panic(err)
	}

	BeforeEach(func() {
		HookConfig.Values = values
		HookConfig.ConfigValues = configValues
		HookConfig.BindingContextsRaw = bindingContextsRaw
	})
}

func (hec *HookExecutionConfig) ValuesGet(path string) gjson.Result {
	return gjson.Get(hec.Values, path)
}

func (hec *HookExecutionConfig) ValuesSet(path string, value interface{}) {
	newValues, err := sjson.Set(hec.Values, path, value)
	Expect(err).ToNot(HaveOccurred())

	hec.Values = newValues
}

func (hec *HookExecutionConfig) ValuesDelete(path string) {
	newValues, err := sjson.Delete(hec.Values, path)
	Expect(err).ToNot(HaveOccurred())

	hec.Values = newValues
}

func (hec *HookExecutionConfig) ConfigValuesGet(path string) gjson.Result {
	return gjson.Get(hec.ConfigValues, path)
}

func (hec *HookExecutionConfig) ConfigValuesSet(path string, value interface{}) {
	newValues, err := sjson.Set(hec.ConfigValues, path, value)
	Expect(err).ToNot(HaveOccurred())

	hec.ConfigValues = newValues
}

func (hec *HookExecutionConfig) ConfigValuesDelete(path string) {
	newValues, err := sjson.Delete(hec.ConfigValues, path)
	Expect(err).ToNot(HaveOccurred())

	hec.ConfigValues = newValues
}

func (hec *HookExecutionConfig) BindingContextsGet(path string) gjson.Result {
	return gjson.Get(hec.BindingContextsRaw, path)
}

func (hec *HookExecutionConfig) BindingContextsSet(path string, value interface{}) {
	newContexts, err := sjson.Set(hec.BindingContextsRaw, path, value)
	Expect(err).ToNot(HaveOccurred())

	hec.BindingContextsRaw = newContexts
}

func (hec *HookExecutionConfig) BindingContextsDelete(path string) {
	newContexts, err := sjson.Delete(hec.BindingContextsRaw, path)
	Expect(err).ToNot(HaveOccurred())

	hec.BindingContextsRaw = newContexts
}

type HookExecutionResult struct {
	PatchedValues       string
	PatchedConfigValues string

	Session *gexec.Session
}

func (her *HookExecutionResult) PatchedValuesGet(path string) gjson.Result {
	return gjson.Get(her.PatchedValues, path)
}

func (her *HookExecutionResult) PatchedConfigValuesGet(path string) gjson.Result {
	return gjson.Get(her.PatchedConfigValues, path)
}

func Hook(text string, body func(hookResult *HookExecutionResult), timeout ...float64) bool {
	var (
		err error

		tmpDir          string
		bindingContexts []hook.BindingContextV1

		ValuesFile                *os.File
		ConfigValuesFile          *os.File
		ValuesJsonPatchFile       *os.File
		ConfigValuesJsonPatchFile *os.File
		BindingContextFile        *os.File

		hookEnvs []string
	)

	itBody := func() {
		// TODO: Encompass into Describe, Context, It blocks
		hookRes := &HookExecutionResult{}

		hookCmd := &exec.Cmd{
			Path: HookConfig.HookPath,
			Args: []string{HookConfig.HookPath, "--config"},
			Env:  hookEnvs,
		}

		hookRes.Session, err = gexec.Start(hookCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ShouldNot(HaveOccurred())

		hookRes.Session.Wait()
		Expect(hookRes.Session.ExitCode()).To(Equal(0))

		out := hookRes.Session.Out.Contents()
		By("Parsing config " + string(out))
		var parsedConfig json.RawMessage
		Expect(json.Unmarshal(out, &parsedConfig)).To(Succeed())

		var parsedValues json.RawMessage
		err = json.Unmarshal([]byte(HookConfig.Values), &parsedValues)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(parsedValues).ToNot(BeNil())

		err = json.Unmarshal([]byte(HookConfig.ConfigValues), &parsedValues)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(parsedValues).ToNot(BeNil())

		if HookConfig.BindingContextsRaw != "" {
			err := json.Unmarshal([]byte(HookConfig.BindingContextsRaw), &bindingContexts)
			Expect(err).ShouldNot(HaveOccurred())
		}

		tmpDir, err = ioutil.TempDir(HookConfig.tmpDir, "")
		Expect(err).ShouldNot(HaveOccurred())

		ValuesFile, err = ioutil.TempFile(tmpDir, "")
		Expect(err).ShouldNot(HaveOccurred())
		hookEnvs = append(hookEnvs, "VALUES_PATH="+ValuesFile.Name())

		ConfigValuesFile, err = ioutil.TempFile(tmpDir, "")
		Expect(err).ShouldNot(HaveOccurred())
		hookEnvs = append(hookEnvs, "CONFIG_VALUES_PATH="+ConfigValuesFile.Name())

		ValuesJsonPatchFile, err = ioutil.TempFile(tmpDir, "")
		Expect(err).ShouldNot(HaveOccurred())
		hookEnvs = append(hookEnvs, "VALUES_JSON_PATCH_PATH="+ValuesJsonPatchFile.Name())

		ConfigValuesJsonPatchFile, err = ioutil.TempFile(tmpDir, "")
		Expect(err).ShouldNot(HaveOccurred())
		hookEnvs = append(hookEnvs, "CONFIG_VALUES_JSON_PATCH_PATH="+ConfigValuesJsonPatchFile.Name())

		BindingContextFile, err = ioutil.TempFile(tmpDir, "")
		Expect(err).ShouldNot(HaveOccurred())
		hookEnvs = append(hookEnvs, "BINDING_CONTEXT_PATH="+BindingContextFile.Name())

		_, err = ValuesFile.WriteString(HookConfig.Values)
		Expect(err).ShouldNot(HaveOccurred())

		_, err = ConfigValuesFile.WriteString(HookConfig.ConfigValues)
		Expect(err).ShouldNot(HaveOccurred())

		_, err = BindingContextFile.WriteString(HookConfig.BindingContextsRaw)
		Expect(err).ShouldNot(HaveOccurred())

		hookCmd = &exec.Cmd{
			Path: HookConfig.HookPath,
			Args: []string{HookConfig.HookPath},
			Env:  hookEnvs,
		}

		hookRes.Session, err = gexec.Start(hookCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ShouldNot(HaveOccurred())

		hookRes.Session.Wait()

		valuesJsonPatchBytes, err := ioutil.ReadAll(ValuesJsonPatchFile)
		Expect(err).ShouldNot(HaveOccurred())
		configValuesJsonPatchBytes, err := ioutil.ReadAll(ConfigValuesJsonPatchFile)
		Expect(err).ShouldNot(HaveOccurred())

		if len(valuesJsonPatchBytes) != 0 {
			patch, err := jsonpatch.DecodePatch(valuesJsonPatchBytes)
			Expect(err).ShouldNot(HaveOccurred())

			patchedValuesBytes, err := patch.Apply([]byte(HookConfig.Values))
			Expect(err).ShouldNot(HaveOccurred())
			hookRes.PatchedValues = string(patchedValuesBytes)
		}

		if len(configValuesJsonPatchBytes) != 0 {
			patch, err := jsonpatch.DecodePatch(configValuesJsonPatchBytes)
			Expect(err).ShouldNot(HaveOccurred())

			patchedConfigValuesBytes, err := patch.Apply([]byte(HookConfig.ConfigValues))
			Expect(err).ShouldNot(HaveOccurred())
			hookRes.PatchedConfigValues = string(patchedConfigValuesBytes)
		}

		body(hookRes)
	}
	return It(text, itBody, timeout...)
}


var _ = AfterSuite(func() {
	By("Removing temporary directories")

	Expect(os.RemoveAll(HookConfig.tmpDir)).Should(Succeed())
})
