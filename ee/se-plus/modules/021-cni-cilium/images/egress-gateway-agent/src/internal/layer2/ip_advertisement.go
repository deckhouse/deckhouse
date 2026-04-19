/*
Copyright 2018 MetalLB

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

package layer2

import (
	"net"

	"k8s.io/apimachinery/pkg/util/sets"
)

// IPAdvertisement is the advertisement Info about LB IP.
type IPAdvertisement struct {
	ip            net.IP
	interfaces    sets.Set[string]
	allInterfaces bool
}

func NewIPAdvertisement(ip net.IP, allInterfaces bool, interfaces sets.Set[string]) IPAdvertisement {
	return IPAdvertisement{
		ip:            ip,
		interfaces:    interfaces,
		allInterfaces: allInterfaces,
	}
}

func (i *IPAdvertisement) Equal(other *IPAdvertisement) bool {
	if i == nil && other == nil {
		return true
	}
	if i == nil || other == nil {
		return false
	}
	if !i.ip.Equal(other.ip) {
		return false
	}
	if i.allInterfaces != other.allInterfaces {
		return false
	}
	if i.allInterfaces {
		return true
	}
	return i.interfaces.Equal(other.interfaces)
}

func (i *IPAdvertisement) MatchInterfaces(intfs ...string) bool {
	if i.allInterfaces {
		return true
	}
	for _, intf := range intfs {
		if i.matchInterface(intf) {
			return true
		}
	}
	return false
}

func (i *IPAdvertisement) matchInterface(intf string) bool {
	if i == nil {
		return false
	}
	if i.allInterfaces {
		return true
	}
	return i.interfaces.Has(intf)
}
func (i *IPAdvertisement) IsAllInterfaces() bool {
	return i.allInterfaces
}

func (i *IPAdvertisement) GetInterfaces() sets.Set[string] {
	return i.interfaces
}
