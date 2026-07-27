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

package testenv

import (
	"maps"
	"slices"
)

type (
	ControllerCRDFile  string
	NodeManagerCRDFile string
)

const (
	MachineCRDFile           ControllerCRDFile = "machine.yaml"
	MachineDeploymentCRDFile ControllerCRDFile = "machine-deployment.yaml"

	NodeGroupCRDFile NodeManagerCRDFile = "node_group.yaml"
	MCMCRDFile       NodeManagerCRDFile = "mcm.yaml"
	InstanceCRDFile  NodeManagerCRDFile = "instance.yaml"
)

func ControllerCRDPaths(crds ...ControllerCRDFile) []string {
	return resolveUpPaths("node-controller/crds", crds)
}

func NodeManagerCRDPaths(crds ...NodeManagerCRDFile) []string {
	return resolveUpPaths("040-node-manager/crds", crds)
}

type crdSet struct {
	controller  map[ControllerCRDFile]struct{}
	nodeManager map[NodeManagerCRDFile]struct{}
}

func (s *crdSet) controllerCRDFiles() []ControllerCRDFile {
	if len(s.controller) == 0 {
		return nil
	}
	return slices.Collect(maps.Keys(s.controller))
}

func (s *crdSet) nodeManagerCRDFiles() []NodeManagerCRDFile {
	if len(s.nodeManager) == 0 {
		return nil
	}
	return slices.Collect(maps.Keys(s.nodeManager))
}

type crdOpt func(*crdSet)

func WithController(crds ...ControllerCRDFile) crdOpt {
	return func(s *crdSet) {
		for _, crd := range crds {
			s.controller[crd] = struct{}{}
		}
	}
}

func WithNodeManager(crds ...NodeManagerCRDFile) crdOpt {
	return func(s *crdSet) {
		for _, crd := range crds {
			s.nodeManager[crd] = struct{}{}
		}
	}
}

func WithMachineCRDFile() crdOpt {
	return WithController(MachineCRDFile)
}

func WithMachineDeploymentCRDFile() crdOpt {
	return WithController(MachineDeploymentCRDFile)
}

func WithNodeGroupCRDFile() crdOpt {
	return WithNodeManager(NodeGroupCRDFile)
}

func WithMCMCRDFile() crdOpt {
	return WithNodeManager(MCMCRDFile)
}

func WithInstanceCRDFile() crdOpt {
	return WithNodeManager(InstanceCRDFile)
}

func CRDPaths(opts ...crdOpt) []string {
	s := &crdSet{
		controller:  make(map[ControllerCRDFile]struct{}),
		nodeManager: make(map[NodeManagerCRDFile]struct{}),
	}

	for _, opt := range opts {
		opt(s)
	}

	return slices.Concat(
		ControllerCRDPaths(s.controllerCRDFiles()...),
		NodeManagerCRDPaths(s.nodeManagerCRDFiles()...),
	)
}
