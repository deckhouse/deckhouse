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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/flant/addon-operator/pkg/hook/types"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/pkg/module_manager/models/hooks"
	"github.com/flant/addon-operator/pkg/module_manager/models/hooks/kind"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/sdk"
	klient "github.com/flant/kube-client/client"
	"github.com/flant/kube-client/fake"
	. "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	utils "github.com/flant/shell-operator/pkg/utils/file"
	hookcontext "github.com/flant/shell-operator/test/hook/context"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/deckhouse/deckhouse/testing/library"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
	"github.com/deckhouse/deckhouse/testing/library/sandbox_runner"
	"github.com/deckhouse/deckhouse/testing/library/values_store"
	"github.com/deckhouse/deckhouse/testing/library/values_validation"
)

var (
	globalTmpDir string
	moduleName   string
)

func (hec *HookExecutionConfig) KubernetesGlobalResource(kind, name string) object_store.KubeObject {
	res := hec.kubernetesResource(kind, "", name)
	// we should be sure, that we dont have .metadata.namespace in global resources
	// thats why we delete it if exists
	metadata, ok := res["metadata"].(map[string]interface{})
	if !ok {
		return res
	}
	delete(metadata, "namespace")
	res["metadata"] = metadata
	return res
}

func (hec *HookExecutionConfig) KubernetesResource(kind, namespace, name string) object_store.KubeObject {
	return hec.kubernetesResource(kind, namespace, name)
}

// TODO extract this GVR finder into github.com/flant/kube-client.
func (hec *HookExecutionConfig) kubernetesResource(kindOrName, namespace, name string) object_store.KubeObject {
	possibleGVR := make([]schema.GroupVersionResource, 0)
	var requestedGroup string
	if x := strings.Split(kindOrName, "."); len(x) > 1 {
		requestedGroup = strings.Join(x[1:], ".")
	}

	for _, group := range hec.fakeCluster.Discovery.Resources {
		for _, apiResource := range group.APIResources {
			if (requestedGroup == "" && strings.EqualFold(apiResource.Kind, kindOrName)) ||
				(requestedGroup == "" && strings.EqualFold(apiResource.Name, kindOrName)) ||
				(requestedGroup != "" && strings.EqualFold(apiResource.Group, requestedGroup)) {
				// ignore parse error, because FakeClusterResources should be valid
				gv, _ := schema.ParseGroupVersion(group.GroupVersion)
				gvr := schema.GroupVersionResource{
					Resource: apiResource.Name,
					Group:    gv.Group,
					Version:  gv.Version,
				}
				possibleGVR = append(possibleGVR, gvr)
				break
			}
		}
	}

	// avoid situation of different groups: v1/v1beta1/etc
	for _, gvr := range possibleGVR {
		b, err := hec.fakeCluster.Client.Dynamic().Resource(gvr).Namespace(namespace).Get(context.TODO(), name, v1.GetOptions{})
		if err == nil {
			return b.UnstructuredContent()
		}
	}

	return object_store.KubeObject{}
}

type ShellOperatorHookConfig struct {
	ConfigVersion interface{} `json:"configVersion,omitempty"`
	Kubernetes    interface{} `json:"kubernetes,omitempty"`
	Schedule      interface{} `json:"schedule,omitempty"`
}

type CustomCRD struct {
	Group      string
	Version    string
	Kind       string
	Namespaced bool
}

type TestMetricsCollector interface {
	CollectedMetrics() []operation.MetricOperation
}

type HookExecutionConfig struct {
	tmpDir                   string // FIXME
	HookPath                 string
	GoHook                   *kind.GoHook
	values                   *values_store.ValuesStore
	configValues             *values_store.ValuesStore
	hookConfig               string // <hook> --config output
	KubeExtraCRDs            []CustomCRD
	IsKubeStateInited        bool
	BindingContexts          BindingContextsSlice
	BindingContextController *hookcontext.BindingContextController
	extraHookEnvs            []string
	ValuesValidator          *values_validation.ValuesValidator
	GoHookError              error
	GoHookBindingActions     []go_hook.BindingAction

	MetricsCollector TestMetricsCollector
	PatchCollector   *object_patch.PatchCollector

	Session      *gexec.Session
	LogrusOutput *gbytes.Buffer

	fakeClusterVersion k8s.FakeClusterVersion
	fakeCluster        *fake.Cluster
}

func (hec *HookExecutionConfig) KubeClient() *klient.Client {
	return hec.fakeCluster.Client
}

func (hec *HookExecutionConfig) RegisterCRD(group, version, kind string, namespaced bool) {
	newCRD := CustomCRD{Group: group, Version: version, Kind: kind, Namespaced: namespaced}
	hec.KubeExtraCRDs = append(hec.KubeExtraCRDs, newCRD)
}

func (hec *HookExecutionConfig) ValuesGet(path string) library.KubeResult {
	return hec.values.Get(path)
}

func (hec *HookExecutionConfig) ConfigValuesGet(path string) library.KubeResult {
	return hec.configValues.Get(path)
}

func (hec *HookExecutionConfig) ValuesSet(path string, value interface{}) {
	hec.values.SetByPath(path, value)
}

func (hec *HookExecutionConfig) ConfigValuesSet(path string, value interface{}) {
	hec.configValues.SetByPath(path, value)
}

func (hec *HookExecutionConfig) ValuesDelete(path string) {
	hec.values.DeleteByPath(path)
}

func (hec *HookExecutionConfig) ConfigValuesDelete(path string) {
	hec.configValues.DeleteByPath(path)
}

func (hec *HookExecutionConfig) ValuesSetFromYaml(path string, value []byte) {
	hec.values.SetByPathFromYAML(path, value)
}

func (hec *HookExecutionConfig) ConfigValuesSetFromYaml(path string, value []byte) {
	hec.configValues.SetByPathFromYAML(path, value)
}

func (hec *HookExecutionConfig) AddHookEnv(env string) {
	hec.extraHookEnvs = append(hec.extraHookEnvs, env)
}

func HookExecutionConfigInit(initValues, initConfigValues string, k8sVersion ...k8s.FakeClusterVersion) *HookExecutionConfig {
	var err error
	hookEnvs := []string{"ADDON_OPERATOR_NAMESPACE=tests", "DECKHOUSE_POD=tests"}

	hec := new(HookExecutionConfig)
	fakeClusterVersion := k8s.DefaultFakeClusterVersion
	if len(k8sVersion) > 0 {
		fakeClusterVersion = k8sVersion[0]
	}
	hec.fakeClusterVersion = fakeClusterVersion

	_, f, _, ok := runtime.Caller(1)
	if !ok {
		panic("can't execute runtime.Caller")
	}
	hec.HookPath = strings.TrimSuffix(f, "_test.go")

	// Use a working directory to retrieve moduleName and modulePath to load OpenAPI schemas.
	wd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("get working directory: %v", err))
	}

	var modulePath string
	if !strings.Contains(wd, "global-hooks") {
		modulePath = wd
		maxDepth := 20
		for {
			modulePathCandidate := filepath.Dir(modulePath)
			if filepath.Base(modulePathCandidate) == "modules" {
				break
			}
			modulePath = modulePathCandidate

			maxDepth--
			if maxDepth == 0 {
				panic("cannot find module name")
			}
		}

		var err error
		moduleName, err = library.GetModuleNameByPath(modulePath)
		if err != nil {
			panic(fmt.Errorf("get module name from working directory: %v", err))
		}
	}

	// Catch logrus messages for LoadOpenAPISchemas.
	buf := &bytes.Buffer{}
	logrus.SetOutput(buf)
	// TODO Is there a solution for ginkgo to have a shared validator for all tests in module?
	hec.ValuesValidator, err = values_validation.NewValuesValidator(moduleName, modulePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, buf.String())
		panic(fmt.Errorf("load module OpenAPI schemas for hook: %v", err))
	}
	// Set logrus output to GinkgoWriter to print only messages for failed specs.
	logrus.SetOutput(GinkgoWriter)

	// Search golang hook by name.
	goHookPath := hec.HookPath + ".go"
	hasGoHook, err := utils.FileExists(goHookPath)
	if err == nil && hasGoHook {
		goHookName := filepath.Base(goHookPath)
		for _, h := range sdk.Registry().Hooks() {
			if strings.Contains(goHookPath, h.GetPath()) {
				hec.GoHook = h
				break
			}
		}
		if hec.GoHook == nil {
			panic(fmt.Errorf("go hook '%s' exists but is not registered as '%s'", goHookPath, goHookName))
		}
		hec.HookPath = ""
	}

	hec.KubeExtraCRDs = []CustomCRD{}

	BeforeEach(func() {
		defaultConfigValues := addonutils.Values{
			addonutils.GlobalValuesKey:                   map[string]interface{}{},
			addonutils.ModuleNameToValuesKey(moduleName): map[string]interface{}{},
		}
		configValues, err := addonutils.NewValuesFromBytes([]byte(initConfigValues))
		if err != nil {
			panic(err)
		}
		mergedConfigValuesYaml, err := addonutils.MergeValues(defaultConfigValues, configValues).YamlBytes()
		if err != nil {
			panic(err)
		}
		values, err := addonutils.NewValuesFromBytes([]byte(initValues))
		if err != nil {
			panic(err)
		}
		mergedValuesYaml, err := addonutils.MergeValues(defaultConfigValues, values).YamlBytes()
		if err != nil {
			panic(err)
		}
		hec.configValues, err = values_store.NewStoreFromRawYaml(mergedConfigValuesYaml)
		if err != nil {
			panic(err)
		}
		hec.values, err = values_store.NewStoreFromRawYaml(mergedValuesYaml)
		if err != nil {
			panic(err)
		}
		hec.IsKubeStateInited = false
		hec.BindingContexts.Set()
	})

	// Run --config for shell hook
	if hec.GoHook == nil {
		hookEnvs = append(hookEnvs, "D8_IS_TESTS_ENVIRONMENT=yes")

		stdout := bytes.Buffer{}
		stderr := bytes.Buffer{}
		cmd := &exec.Cmd{
			Path:   hec.HookPath,
			Args:   []string{hec.HookPath, "--config"},
			Env:    append(os.Environ(), hookEnvs...),
			Stdout: &stdout,
			Stderr: &stderr,
		}

		hec.tmpDir, err = os.MkdirTemp(globalTmpDir, "")
		if err != nil {
			panic(err)
		}

		if err := cmd.Run(); err != nil {
			panic(fmt.Errorf("%s\nstdout:\n%s\n\nstderr:\n%s", err, stdout.String(), stderr.String()))
		}

		var config ShellOperatorHookConfig
		err = yaml.Unmarshal(stdout.Bytes(), &config)
		if err != nil {
			panic(err)
		}

		result, err := json.Marshal(config)
		if err != nil {
			panic(err)
		}
		hec.hookConfig = string(result)
	}

	return hec
}

func (hec *HookExecutionConfig) KubeStateSetAndWaitForBindingContexts(newKubeState string, _ int) hookcontext.GeneratedBindingContexts {
	// The method is deprecated
	return hec.KubeStateSet(newKubeState)
}

func (hec *HookExecutionConfig) KubeStateSet(newKubeState string) hookcontext.GeneratedBindingContexts {
	var contexts hookcontext.GeneratedBindingContexts
	var err error
	if !hec.IsKubeStateInited {
		hec.BindingContextController = hookcontext.NewBindingContextController(hec.hookConfig, hec.fakeClusterVersion)
		if err != nil {
			panic(err)
		}
		hec.fakeCluster = hec.BindingContextController.FakeCluster()
		hec.fakeCluster.Client.WithServer("fake-test")
		dependency.TestDC.K8sClient = hec.fakeCluster.Client

		if hec.GoHook != nil {
			// TODO: check if global here
			m := hooks.NewModuleHook(hec.GoHook)
			err := m.InitializeHookConfig()
			if err != nil {
				panic(err)
			}
			hec.GoHook.BackportHookConfig(&m.GetHookConfig().HookConfig)
			shHook := hec.GoHook.GetBasicHook()

			hec.BindingContextController.WithHook(&shHook)
		}

		if len(hec.KubeExtraCRDs) > 0 {
			for _, crd := range hec.KubeExtraCRDs {
				hec.BindingContextController.RegisterCRD(crd.Group, crd.Version, crd.Kind, crd.Namespaced)
			}
		}

		contexts, err = hec.BindingContextController.Run(newKubeState)
		if err != nil {
			panic(err)
		}
		hec.IsKubeStateInited = true
	} else {
		contexts, err = hec.BindingContextController.ChangeState(newKubeState)
		if err != nil {
			panic(err)
		}
	}

	return contexts
}

// GenerateOnStartupContext returns binding context for OnStartup.
func (hec *HookExecutionConfig) GenerateOnStartupContext() hookcontext.GeneratedBindingContexts {
	return SimpleBindingGeneratedBindingContext(OnStartup)
}

// GenerateScheduleContext returns binding context for Schedule with needed snapshots.
func (hec *HookExecutionConfig) GenerateScheduleContext(crontab string) hookcontext.GeneratedBindingContexts {
	if hec.BindingContextController == nil {
		return SimpleBindingGeneratedBindingContext(Schedule)
	}
	contexts, err := hec.BindingContextController.RunSchedule(crontab)
	if err != nil {
		panic(err)
	}
	return contexts
}

func (hec *HookExecutionConfig) generateAllSnapshotsContext(binding BindingType) hookcontext.GeneratedBindingContexts {
	if hec.BindingContextController == nil {
		return SimpleBindingGeneratedBindingContext(binding)
	}

	contexts, err := hec.BindingContextController.RunBindingWithAllSnapshots(binding)
	if err != nil {
		panic(err)
	}
	return contexts
}

// GenerateBeforeHelmContext returns binding context for beforeHelm binding with all available snapshots.
func (hec *HookExecutionConfig) GenerateBeforeHelmContext() hookcontext.GeneratedBindingContexts {
	return hec.generateAllSnapshotsContext(BeforeHelm)
}

// GenerateAfterHelmContext returns binding context for afterHelm binding with all available snapshots.
func (hec *HookExecutionConfig) GenerateAfterHelmContext() hookcontext.GeneratedBindingContexts {
	return hec.generateAllSnapshotsContext(AfterHelm)
}

// GenerateAfterDeleteHelmContext returns binding context for afterDeleteHelm binding with all available snapshots.
func (hec *HookExecutionConfig) GenerateAfterDeleteHelmContext() hookcontext.GeneratedBindingContexts {
	return hec.generateAllSnapshotsContext(AfterDeleteHelm)
}

// GenerateBeforeAllContext returns binding context for beforeAll binding with all available snapshots.
func (hec *HookExecutionConfig) GenerateBeforeAllContext() hookcontext.GeneratedBindingContexts {
	return hec.generateAllSnapshotsContext(BeforeAll)
}

// GenerateAfterAllContext returns binding context for afterAll binding with all available snapshots.
func (hec *HookExecutionConfig) GenerateAfterAllContext() hookcontext.GeneratedBindingContexts {
	return hec.generateAllSnapshotsContext(AfterAll)
}

func (hec *HookExecutionConfig) RunHook() {
	if hec.GoHook != nil {
		hec.RunGoHook()
		return
	}

	var (
		err error

		tmpDir string

		ValuesFile                *os.File
		ConfigValuesFile          *os.File
		ValuesJSONPatchFile       *os.File
		ConfigValuesJSONPatchFile *os.File
		BindingContextFile        *os.File
		KubernetesPatchSetFile    *os.File
		MetricsFile               *os.File

		hookEnvs []string
	)

	hookEnvs = append(hookEnvs, "ADDON_OPERATOR_NAMESPACE=tests", "DECKHOUSE_POD=tests", "D8_IS_TESTS_ENVIRONMENT=yes", "PATH="+os.Getenv("PATH"))
	hookEnvs = append(hookEnvs, hec.extraHookEnvs...)

	hookCmd := &exec.Cmd{
		Path: hec.HookPath,
		Args: []string{hec.HookPath, "--config"},
		Env:  append(os.Environ(), hookEnvs...),
	}

	hec.Session, err = gexec.Start(hookCmd, nil, GinkgoWriter)
	Expect(err).ShouldNot(HaveOccurred())

	hec.Session.Wait(10)
	Expect(hec.Session.ExitCode()).To(Equal(0))

	out := hec.Session.Out.Contents()
	var parsedConfig json.RawMessage
	Expect(yaml.Unmarshal(out, &parsedConfig)).To(Succeed())

	Expect(hec.values.JSONRepr).ToNot(BeEmpty())
	Expect(hec.configValues.JSONRepr).ToNot(BeEmpty())

	By("Validating initial values")
	Expect(hec.ValuesValidator.ValidateJSONValues(moduleName, hec.values.JSONRepr, false)).To(Succeed())
	By("Validating initial config values")
	Expect(hec.ValuesValidator.ValidateJSONValues(moduleName, hec.configValues.JSONRepr, true)).To(Succeed())

	tmpDir, err = TempDirWithPerms(globalTmpDir, "", 0o777)
	Expect(err).ShouldNot(HaveOccurred())

	ValuesFile, err = TempFileWithPerms(tmpDir, "", 0o777)
	Expect(err).ShouldNot(HaveOccurred())
	hookEnvs = append(hookEnvs, "VALUES_PATH="+ValuesFile.Name())

	ConfigValuesFile, err = TempFileWithPerms(tmpDir, "", 0o777)
	Expect(err).ShouldNot(HaveOccurred())
	hookEnvs = append(hookEnvs, "CONFIG_VALUES_PATH="+ConfigValuesFile.Name())

	ValuesJSONPatchFile, err = TempFileWithPerms(tmpDir, "", 0o777)
	Expect(err).ShouldNot(HaveOccurred())
	hookEnvs = append(hookEnvs, "VALUES_JSON_PATCH_PATH="+ValuesJSONPatchFile.Name())

	ConfigValuesJSONPatchFile, err = TempFileWithPerms(tmpDir, "", 0o777)
	Expect(err).ShouldNot(HaveOccurred())
	hookEnvs = append(hookEnvs, "CONFIG_VALUES_JSON_PATCH_PATH="+ConfigValuesJSONPatchFile.Name())

	BindingContextFile, err = TempFileWithPerms(tmpDir, "", 0o777)
	Expect(err).ShouldNot(HaveOccurred())
	hookEnvs = append(hookEnvs, "BINDING_CONTEXT_PATH="+BindingContextFile.Name())

	KubernetesPatchSetFile, err = TempFileWithPerms(tmpDir, "", 0o777)
	Expect(err).ShouldNot(HaveOccurred())
	hookEnvs = append(hookEnvs, "D8_TEST_KUBERNETES_PATCH_SET_FILE="+KubernetesPatchSetFile.Name())

	MetricsFile, err = TempFileWithPerms(tmpDir, "", 0o777)
	Expect(err).ShouldNot(HaveOccurred())
	hookEnvs = append(hookEnvs, "METRICS_PATH="+MetricsFile.Name())

	hookCmd = &exec.Cmd{
		Path: hec.HookPath,
		Args: []string{hec.HookPath},
		Dir:  "/deckhouse",
		Env:  hookEnvs,
	}

	options := []sandbox_runner.SandboxOption{
		sandbox_runner.WithFile(ValuesFile.Name(), hec.values.JSONRepr),
		sandbox_runner.WithFile(ConfigValuesFile.Name(), hec.configValues.JSONRepr),
		sandbox_runner.WithFile(BindingContextFile.Name(), []byte(hec.BindingContexts.JSON)),
	}

	hec.Session = sandbox_runner.Run(hookCmd, options...)
	if hec.Session.ExitCode() != 0 {
		By("Shell hook execution failed", func() {
			fmt.Fprint(GinkgoWriter, hookColoredOutput("stdout", hec.Session.Out.Contents()))
			fmt.Fprint(GinkgoWriter, hookColoredOutput("stderr", hec.Session.Err.Contents()))
		})
	}

	valuesJSONPatchBytes, err := io.ReadAll(ValuesJSONPatchFile)
	Expect(err).ShouldNot(HaveOccurred())
	configValuesJSONPatchBytes, err := io.ReadAll(ConfigValuesJSONPatchFile)
	Expect(err).ShouldNot(HaveOccurred())
	kubernetesPatchBytes, err := io.ReadAll(KubernetesPatchSetFile)
	Expect(err).ShouldNot(HaveOccurred())

	// TODO: take a closer look and refactor into a function
	if len(valuesJSONPatchBytes) != 0 {
		patch, err := addonutils.JsonPatchFromBytes(valuesJSONPatchBytes)
		Expect(err).ShouldNot(HaveOccurred())

		patchedValuesBytes, err := patch.Apply(hec.values.JSONRepr)
		Expect(err).ShouldNot(HaveOccurred())
		hec.values = values_store.NewStoreFromRawJSON(patchedValuesBytes)
	}

	if len(configValuesJSONPatchBytes) != 0 {
		patch, err := addonutils.JsonPatchFromBytes(configValuesJSONPatchBytes)
		Expect(err).ShouldNot(HaveOccurred())

		patchedConfigValuesBytes, err := patch.Apply(hec.configValues.JSONRepr)
		Expect(err).ShouldNot(HaveOccurred())
		hec.configValues = values_store.NewStoreFromRawJSON(patchedConfigValuesBytes)
	}

	By("Validating resulting values")
	Expect(hec.ValuesValidator.ValidateJSONValues(moduleName, hec.values.JSONRepr, false)).To(Succeed())
	By("Validating resulting config values")
	Expect(hec.ValuesValidator.ValidateJSONValues(moduleName, hec.configValues.JSONRepr, true)).To(Succeed())

	if len(kubernetesPatchBytes) != 0 {
		operations, err := object_patch.ParseOperations(kubernetesPatchBytes)
		Expect(err).ShouldNot(HaveOccurred())

		patcher := object_patch.NewObjectPatcher(hec.getFakeClient())
		err = patcher.ExecuteOperations(operations)
		Expect(err).ToNot(HaveOccurred())
	}
}

func (hec *HookExecutionConfig) getFakeClient() *klient.Client {
	f := hec.fakeCluster
	if f == nil {
		f = fake.NewFakeCluster(hec.fakeClusterVersion)
	}

	return f.Client
}

// hookColoredOutput colored stdout and stderr streams for shell hooks
func hookColoredOutput(stream string, text []byte) string {
	if len(text) == 0 {
		text = []byte("\n") // line sticks together
	}

	var preamble string
	switch stream {
	case "stdout":
		preamble = "Hook stdout:"
		if !config.DefaultReporterConfig.NoColor {
			preamble = "\u001B[33mHook stdout:\u001B[0m"
			text = []byte(fmt.Sprintf("\u001B[93m%s\u001B[0m", text))
		}

	case "stderr":
		preamble = "Hook stderr:"
		if !config.DefaultReporterConfig.NoColor {
			preamble = "\u001B[33mHook stderr:\u001B[0m"
			text = []byte(fmt.Sprintf("\u001B[35m%s\u001B[0m", text))
		}
	}

	return fmt.Sprintf("%s %s", preamble, text)
}

func (hec *HookExecutionConfig) RunGoHook() {
	if hec.GoHook == nil {
		return
	}

	var (
		err error
	)

	Expect(hec.values.JSONRepr).ToNot(BeEmpty())

	Expect(hec.configValues.JSONRepr).ToNot(BeEmpty())

	values, err := addonutils.NewValuesFromBytes(hec.values.JSONRepr)
	Expect(err).ShouldNot(HaveOccurred())

	convigValues, err := addonutils.NewValuesFromBytes(hec.configValues.JSONRepr)
	Expect(err).ShouldNot(HaveOccurred())

	patchableValues, err := go_hook.NewPatchableValues(values)
	Expect(err).ShouldNot(HaveOccurred())

	patchableConfigValues, err := go_hook.NewPatchableValues(convigValues)
	Expect(err).ShouldNot(HaveOccurred())

	var formattedSnapshots = make(go_hook.Snapshots, len(hec.BindingContexts.BindingContexts))
	for _, bCtx := range hec.BindingContexts.BindingContexts {
		for snapBindingName, snaps := range bCtx.Snapshots {
			for _, snapshot := range snaps {
				formattedSnapshots[snapBindingName] = append(formattedSnapshots[snapBindingName], snapshot.FilterResult)
			}
		}
	}

	// TODO: assert on metrics
	metricsCollector := metrics.NewCollector(hec.HookPath)
	hec.MetricsCollector = metricsCollector

	// Catch all log messages into assertable buffer.
	hec.LogrusOutput = gbytes.NewBuffer()
	logger := logrus.New()
	logger.SetOutput(hec.LogrusOutput)

	// TODO: assert on binding actions
	var bindingActions []go_hook.BindingAction

	// make spec generator to reproduce behavior with deferred object mutations like in addon-operator
	patchCollector := object_patch.NewPatchCollector()
	hec.PatchCollector = patchCollector

	hookInput := &go_hook.HookInput{
		Snapshots:        formattedSnapshots,
		Values:           patchableValues,
		ConfigValues:     patchableConfigValues,
		MetricsCollector: metricsCollector,
		LogEntry:         logger.WithField("output", "gohook"),
		PatchCollector:   patchCollector,
		BindingActions:   &bindingActions,
	}

	if len(hec.extraHookEnvs) > 0 {
		for _, envpair := range hec.extraHookEnvs {
			pair := strings.Split(envpair, "=")
			_ = os.Setenv(pair[0], pair[1])
			defer func(key string) {
				_ = os.Unsetenv(key)
			}(pair[0])
		}
	}

	hec.GoHookError = hec.GoHook.Run(hookInput)

	if patches := hookInput.Values.GetPatches(); len(patches) != 0 {
		valuesPatch := addonutils.NewValuesPatch()
		valuesPatch.Operations = patches
		patchedValuesBytes, err := valuesPatch.ApplyStrict(hec.values.JSONRepr)
		Expect(err).ShouldNot(HaveOccurred())
		hec.values = values_store.NewStoreFromRawJSON(patchedValuesBytes)
	}

	if patches := hookInput.ConfigValues.GetPatches(); len(patches) != 0 {
		valuesPatch := addonutils.NewValuesPatch()
		valuesPatch.Operations = patches
		patchedConfigValuesBytes, err := valuesPatch.ApplyStrict(hec.configValues.JSONRepr)
		Expect(err).ShouldNot(HaveOccurred())
		hec.configValues = values_store.NewStoreFromRawJSON(patchedConfigValuesBytes)
	}

	if operations := patchCollector.Operations(); len(operations) > 0 {
		patcher := object_patch.NewObjectPatcher(hec.getFakeClient())
		err := patcher.ExecuteOperations(operations)
		Expect(err).ShouldNot(HaveOccurred())
	}

	hec.GoHookBindingActions = bindingActions

	By("Validating resulting values")
	Expect(hec.ValuesValidator.ValidateJSONValues(moduleName, hec.values.JSONRepr, false)).To(Succeed())
	By("Validating resulting config values")
	Expect(hec.ValuesValidator.ValidateJSONValues(moduleName, hec.configValues.JSONRepr, true)).To(Succeed())
}

var _ = BeforeSuite(func() {
	By("Setup testing env variable")
	Expect(os.Setenv("D8_IS_TESTS_ENVIRONMENT", "true")).Should(Succeed())
})

var _ = AfterSuite(func() {
	By("Removing temporary directories")
	Expect(os.RemoveAll(globalTmpDir)).Should(Succeed())
	By("Removing testing env variable")
	Expect(os.Unsetenv("D8_IS_TESTS_ENVIRONMENT")).Should(Succeed())
})
