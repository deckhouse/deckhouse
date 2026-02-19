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

package runtimeextension

import (
	"encoding/json"
	"net/http"

	dvpapi "dvp-common/api"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

const (
	runtimeHookBasePath = "/hooks.runtime.cluster.x-k8s.io/v1alpha1"

	HandlerNameCanUpdateMachineSet = "dvp-can-update-machineset"
	HandlerNameCanUpdateMachine    = "dvp-can-update-machine"
	HandlerNameUpdateMachine       = "dvp-update-machine"

	DiscoveryPath           = runtimeHookBasePath + "/discovery"
	CanUpdateMachineSetPath = runtimeHookBasePath + "/canupdatemachineset/" + HandlerNameCanUpdateMachineSet
	CanUpdateMachinePath    = runtimeHookBasePath + "/canupdatemachine/" + HandlerNameCanUpdateMachine
	UpdateMachinePath       = runtimeHookBasePath + "/updatemachine/" + HandlerNameUpdateMachine
)

// Extension implements CAPI Runtime SDK hooks for DVP in-place updates.
type Extension struct {
	client      client.Client
	dvp         *dvpapi.DVPCloudAPI
	clusterUUID string
	log         logr.Logger
}

// NewExtension creates a new runtime extension handler.
func NewExtension(dvp *dvpapi.DVPCloudAPI, c client.Client, clusterUUID string) *Extension {
	return &Extension{
		client:      c,
		dvp:         dvp,
		clusterUUID: clusterUUID,
		log:         ctrl.Log.WithName("runtime-extension"),
	}
}

// SetupWithWebhookServer registers HTTP handlers on the existing webhook server.
func (e *Extension) SetupWithWebhookServer(srv webhook.Server) {
	srv.Register(DiscoveryPath, http.HandlerFunc(e.HandleDiscovery))
	srv.Register(CanUpdateMachineSetPath, http.HandlerFunc(e.HandleCanUpdateMachineSet))
	srv.Register(CanUpdateMachinePath, http.HandlerFunc(e.HandleCanUpdateMachine))
	srv.Register(UpdateMachinePath, http.HandlerFunc(e.HandleUpdateMachine))
	e.log.Info("Runtime extension handlers registered",
		"discovery", DiscoveryPath,
		"canUpdateMachineSet", CanUpdateMachineSetPath,
		"canUpdateMachine", CanUpdateMachinePath,
		"updateMachine", UpdateMachinePath,
	)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{
		"status":  "Failure",
		"message": msg,
	})
}
