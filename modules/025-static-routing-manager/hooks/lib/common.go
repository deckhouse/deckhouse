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
	"github.com/deckhouse/deckhouse/modules/025-static-routing-manager/hooks/lib/v1alpha1"
)

// Common types

type RoutingTableInfo struct {
	Name           string
	IPRouteTableID int
	Routes         []v1alpha1.Route
	NodeSelector   map[string]string
}

type NodeRoutingTableInfo struct {
	Name              string
	NodeRoutingTables v1alpha1.NodeRoutingTablesSpec
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
	RTResource   = "routingtables"
	RTKind       = "RoutingTable"
	NRTKind      = "NodeRoutingTable"
)

// Common var

// Common func

func NRTSAppend(nrts *v1alpha1.NodeRoutingTablesSpec, rti RoutingTableInfo) {
	if _, ok := nrts.RoutingTables[rti.IPRouteTableID]; !ok {
		var tmpRts v1alpha1.Routes
		tmpRts.Routes = rti.Routes
		nrts.RoutingTables[rti.IPRouteTableID] = tmpRts
	} else {
		for _, rt := range rti.Routes {
			for _, nrt := range nrts.RoutingTables[rti.IPRouteTableID].Routes {
				if rt.Destination == nrt.Destination && rt.Gateway == nrt.Gateway {
					continue
				}
				tmpNRts := nrts.RoutingTables[rti.IPRouteTableID]
				tmpNRts.Routes = append(tmpNRts.Routes, rt)
				nrts.RoutingTables[rti.IPRouteTableID] = tmpNRts
			}
		}
	}
}
