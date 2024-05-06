/*
Copyright 2024 Flant JSC

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

package lib

import (
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/modules/025-static-routing-manager/hooks/lib/v1alpha1"
)

// Common types

type RoutingTableInfo struct {
	Name           string
	UID            types.UID
	IPRouteTableID int
	Routes         []v1alpha1.Route
	NodeSelector   map[string]string
}

type NodeRoutingTableInfo struct {
	Name             string
	NodeRoutingTable v1alpha1.NodeRoutingTableSpec
}

type NodeInfo struct {
	Name   string
	Labels map[string]string
}

// Common const

const (
	Group        = "network.deckhouse.io"
	Version      = "v1alpha1"
	GroupVersion = Group + "/" + Version
	RTKind       = "RoutingTable"
	NRTKind      = "NodeRoutingTable"
	// RTResource   = "routingtables"
)

// Common var

// Common func

func NRTSAppend(nrts *v1alpha1.NodeRoutingTableSpec, rti RoutingTableInfo) {
	nrts.IPRouteTableID = rti.IPRouteTableID
	nrts.Routes = rti.Routes
}
