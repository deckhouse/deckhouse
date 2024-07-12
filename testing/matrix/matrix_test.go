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

package matrix

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/testing/matrix/linter"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/modules"
)

type TestMatrixSuite struct {
	suite.Suite
}

func TestMatrix(t *testing.T) {
	suite.Run(t, new(TestMatrixSuite))
}

func (s *TestMatrixSuite) SetupSuite() {
	s.changeSymlinks()
}

func (s *TestMatrixSuite) TearDownSuite() {
	s.restoreSymlinks()
}

func (s *TestMatrixSuite) TestMatrix() {
	// Use environment variable to focus on specific modules, e.g. FOCUS=user-authn,user-authz
	focus := os.Getenv("FOCUS")

	focusNames := set.New()
	if focus != "" {
		parts := strings.Split(focus, ",")
		for _, part := range parts {
			focusNames.Add(part)
		}
	}

	discoveredModules, err := modules.GetDeckhouseModulesWithValuesMatrixTests(focusNames)
	s.Require().NoError(err)

	for _, module := range discoveredModules {
		if focusNames.Size() == 0 || focusNames.Has(module.Name) {
			s.Run(module.Name, func() {
				s.Assert().NoError(linter.Run("", module))
			})
		}
	}
}

// changeSymlinks changes symlinks in module dir to proper place when modules in ee/fe not copied to main modules directory
func (s *TestMatrixSuite) changeSymlink(symlinkPath string, newDestination string) {
	err := os.Remove(symlinkPath)
	s.Require().NoError(err)

	err = os.Symlink(newDestination, symlinkPath)
	s.Require().NoError(err)
}

func (s *TestMatrixSuite) symlink(oldName, newName string) {
	if err := os.Symlink(oldName, newName); !os.IsExist(err) {
		s.Require().NoError(err)
	}
}

func (s *TestMatrixSuite) delSymlink(name string) {
	if err := os.Remove(name); !os.IsNotExist(err) {
		s.Require().NoError(err)
	}
}

func (s *TestMatrixSuite) changeSymlinks() {
	s.changeSymlink(
		"/deckhouse/ee/modules/030-cloud-provider-openstack/candi",
		"/deckhouse/ee/candi/cloud-providers/openstack/")
	s.changeSymlink(
		"/deckhouse/ee/modules/030-cloud-provider-vsphere/candi",
		"/deckhouse/ee/candi/cloud-providers/vsphere/")
	s.changeSymlink(
		"/deckhouse/ee/modules/030-cloud-provider-vcd/candi",
		"/deckhouse/ee/candi/cloud-providers/vcd/")
	s.changeSymlink(
		"/deckhouse/ee/modules/030-cloud-provider-zvirt/candi",
		"/deckhouse/ee/candi/cloud-providers/zvirt/")
	s.delSymlink("/deckhouse/modules/040-node-manager/images_digests.json")
	s.symlink(
		"/deckhouse/ee/modules/030-cloud-provider-openstack/cloud-instance-manager/",
		"/deckhouse/modules/040-node-manager/cloud-providers/openstack",
	)
	s.symlink(
		"/deckhouse/ee/modules/030-cloud-provider-vsphere/cloud-instance-manager/",
		"/deckhouse/modules/040-node-manager/cloud-providers/vsphere",
	)
}

// restoreSymlinks restores symlinks in module dir to original place
func (s *TestMatrixSuite) restoreSymlinks() {
	s.changeSymlink(
		"/deckhouse/ee/modules/030-cloud-provider-openstack/candi",
		"/deckhouse/candi/cloud-providers/openstack/")
	s.changeSymlink(
		"/deckhouse/ee/modules/030-cloud-provider-vsphere/candi",
		"/deckhouse/candi/cloud-providers/vsphere/")
	s.changeSymlink(
		"/deckhouse/ee/modules/030-cloud-provider-vcd/candi",
		"/deckhouse/candi/cloud-providers/vcd/")

	s.symlink(
		"../images_digests.json",
		"/deckhouse/modules/040-node-manager/images_digests.json",
	)

	s.delSymlink("/deckhouse/modules/040-node-manager/cloud-providers/openstack")
	s.delSymlink("/deckhouse/modules/040-node-manager/cloud-providers/vsphere")
}
