/*
Copyright 2025 Flant JSC

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

package controller

import (
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"github.com/deckhouse/node-controller/internal/controller/bashiblecleanup"
	"github.com/deckhouse/node-controller/internal/controller/draining"
	"github.com/deckhouse/node-controller/internal/controller/fencing"
	instance_controller "github.com/deckhouse/node-controller/internal/controller/instance"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup"
	"github.com/deckhouse/node-controller/internal/controller/nodetemplate"
	"github.com/deckhouse/node-controller/internal/controller/staticproviderid"
	"github.com/deckhouse/node-controller/internal/controller/updateapproval"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterGroup(register.NodeControllers, &corev1.Node{},
		&bashiblecleanup.Reconciler{},
		&draining.Reconciler{},
		&fencing.Reconciler{},
		&nodetemplate.Reconciler{},
		&staticproviderid.Reconciler{},
	)

	register.RegisterGroup(register.NodeGroupControllers, &v1.NodeGroup{},
		&nodegroup.Status{},
		updateapproval.New(),
	)

	register.RegisterController(register.InstanceControllers, &deckhousev1alpha2.Instance{},
		&instance_controller.InstanceController{},
	)
}
