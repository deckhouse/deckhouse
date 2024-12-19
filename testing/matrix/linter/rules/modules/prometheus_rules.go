/*
Copyright 2022 Flant JSC

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

package modules

import (
	"os"
	"os/exec"
	"sync"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/utils"
)

type checkResult struct {
	success bool
	errMsg  string
}

type rulesCacheStruct struct {
	cache map[string]checkResult
	mu    sync.RWMutex
}

const promtoolPath = "/deckhouse/bin/promtool"

var rulesCache = rulesCacheStruct{
	cache: make(map[string]checkResult),
	mu:    sync.RWMutex{},
}

func (r *rulesCacheStruct) Put(hash string, value checkResult) {
	rulesCache.mu.Lock()
	defer rulesCache.mu.Unlock()

	rulesCache.cache[hash] = value
}

func (r *rulesCacheStruct) Get(hash string) (checkResult, bool) {
	rulesCache.mu.RLock()
	defer rulesCache.mu.RUnlock()

	res, ok := rulesCache.cache[hash]
	return res, ok
}

func PromtoolAvailable() bool {
	info, err := os.Stat(promtoolPath)
	return err == nil && (info.Mode().Perm()&0111 != 0)
}

func marshalChartYaml(object storage.StoreObject) ([]byte, error) {
	marshal, err := yaml.Marshal(object.Unstructured.Object["spec"])
	if err != nil {
		return nil, err
	}
	return marshal, nil
}

func writeTempRuleFileFromObject(m utils.Module, marshalledYaml []byte) (string, error) {
	renderedFile, err := os.CreateTemp("", m.Name+".*.yml")
	if err != nil {
		return "", err
	}
	defer func(renderedFile *os.File) {
		_ = renderedFile.Close()
	}(renderedFile)

	_, err = renderedFile.Write(marshalledYaml)
	if err != nil {
		return "", err
	}
	_ = renderedFile.Sync()
	return renderedFile.Name(), nil
}

func checkRuleFile(path string) error {
	promtoolComand := exec.Command(promtoolPath, "check", "rules", path)
	_, err := promtoolComand.Output()
	return err
}

func createPromtoolError(m utils.Module, errMsg string) errors.LintRuleError {
	return errors.NewLintRuleError(
		"MODULE060",
		moduleLabel(m.Name),
		m.Path,
		"Promtool check failed for Helm chart:\n%s",
		errMsg,
	)
}

func PromtoolRuleCheck(m utils.Module, object storage.StoreObject) errors.LintRuleError {
	if object.Unstructured.GetKind() != "PrometheusRule" {
		return errors.EmptyRuleError
	}

	res, ok := rulesCache.Get(object.Hash)
	if ok {
		if !res.success {
			return createPromtoolError(m, res.errMsg)
		}
		return errors.EmptyRuleError
	}

	marshal, err := marshalChartYaml(object)
	if err != nil {
		return errors.NewLintRuleError(
			"MODULE060",
			m.Name,
			m.Path,
			"Error marshalling Helm chart to yaml",
		)
	}

	path, err := writeTempRuleFileFromObject(m, marshal)
	defer os.Remove(path)

	if err != nil {
		return errors.NewLintRuleError(
			"MODULE060",
			m.Name,
			m.Path,
			"Error creating temporary rule file from Helm chart:\n%s",
			err.Error(),
		)
	}

	err = checkRuleFile(path)
	if err != nil {
		errorMessage := string(err.(*exec.ExitError).Stderr)
		rulesCache.Put(object.Hash, checkResult{
			success: false,
			errMsg:  errorMessage,
		})
		return createPromtoolError(m, errorMessage)
	}

	rulesCache.Put(object.Hash, checkResult{success: true})
	return errors.EmptyRuleError
}
