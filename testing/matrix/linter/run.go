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

package linter

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/modules"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/utils"
)

// applyTags if ugly because values now are strongly untyped. We have to rewrite this after adding proper global schema
func applyTags(tags map[string]map[string]string, values interface{}) {
	values.(map[string]interface{})["global"].(map[string]interface{})["modulesImages"].(map[string]interface{})["tags"] = tags
}

func isExist(baseDir, filename string) bool {
	_, err := os.Stat(filepath.Join(baseDir, filename))
	return err == nil
}

// changeSymlink check symlink is exists and changes them
func changeSymlink(symlinkPath string, newDestination string) error {
	_, err := os.Lstat(symlinkPath)
	if err != nil {
		return err
	}

	err = os.Remove(symlinkPath)
	if err != nil {
		return err
	}

	return os.Symlink(newDestination, symlinkPath)
}

// changeSymlinks changes symlinks in module dir to proper place when modules in ee/fe not copied to main modules directory
func changeSymlinks(m utils.Module) error {
	switch m.Name {
	case "cloud-provider-openstack":
		err := changeSymlink("/deckhouse/ee/modules/030-cloud-provider-openstack/candi", "/deckhouse/ee/candi/cloud-providers/openstack/")
		if err != nil {
			return err
		}
	case "cloud-provider-vsphere":
		err := changeSymlink("/deckhouse/ee/modules/030-cloud-provider-vsphere/candi", "/deckhouse/ee/candi/cloud-providers/vsphere/")
		if err != nil {
			return err
		}
	case "node-manager":
		_, err := os.Lstat("/deckhouse/modules/040-node-manager/images_tags.json")
		if err != nil {
			return err
		}
		err = os.Remove("/deckhouse/modules/040-node-manager/images_tags.json")
		if err != nil {
			return err
		}
		err = os.Symlink("/deckhouse/ee/modules/030-cloud-provider-openstack/cloud-instance-manager/", "/deckhouse/modules/040-node-manager/cloud-providers/openstack")
		if err != nil {
			return err
		}
		err = os.Symlink("/deckhouse/ee/modules/030-cloud-provider-vsphere/cloud-instance-manager/", "/deckhouse/modules/040-node-manager/cloud-providers/vsphere")
		if err != nil {
			return err
		}

	}
	return nil
}

// restoreSymlinks restores symlinks in module dir to original place
func restoreSymlinks(m utils.Module) error {
	switch m.Name {
	case "cloud-provider-openstack":
		err := changeSymlink("/deckhouse/ee/modules/030-cloud-provider-openstack/candi", "/deckhouse/candi/cloud-providers/openstack/")
		if err != nil {
			return err
		}
	case "cloud-provider-vsphere":
		err := changeSymlink("/deckhouse/ee/modules/030-cloud-provider-vsphere/candi", "/deckhouse/candi/cloud-providers/vsphere/")
		if err != nil {
			return err
		}
	case "node-manager":
		err := os.Symlink("../images_tags.json", "/deckhouse/modules/040-node-manager/images_tags.json")
		if err != nil {
			return err
		}
		err = os.Remove("/deckhouse/modules/040-node-manager/cloud-providers/openstack")
		if err != nil {
			return err
		}
		err = os.Remove("/deckhouse/modules/040-node-manager/cloud-providers/vsphere")
		if err != nil {
			return err
		}
	}
	return nil
}

func Run(tmpDir string, m utils.Module) error {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("panic on linter run occurred: %v\n", r)
		}
	}()

	// Silence default loggers
	log.SetOutput(ioutil.Discard)      // helm
	logrus.SetLevel(logrus.PanicLevel) // shell-operator

	var values []string
	var err error

	err = changeSymlinks(m)
	if err != nil {
		return err
	}

	if isExist(m.Path, "openapi") && !isExist(m.Path, "values_matrix_test.yaml") {
		values, err = ComposeValuesFromSchemas(m)
		if err != nil {
			return fmt.Errorf("saving values from openapi: %v", err)
		}
	} else {
		f, err := LoadConfiguration(filepath.Join(m.Path, modules.ValuesConfigFilename), "", tmpDir)
		if err != nil {
			return fmt.Errorf("configuration loading error: %v", err)
		}
		defer f.Close()

		f.FindAll()

		values, err = f.ReturnValues()
		if err != nil {
			return fmt.Errorf("saving values error: %v", err)
		}
	}

	res := NewModuleController(m, values).Run()

	err = restoreSymlinks(m)
	if err != nil {
		return err
	}
	return res
}
