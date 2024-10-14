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

package registry

import "fmt"

const (
	UnknownChannelSecretDiscovery = "Unknown"
)

var _ fmt.Stringer = (*Channel)(nil)

type Channel string

func (ch Channel) String() string {
	return string(ch)
}

func (ch Channel) IsValid() bool {
	switch ch {
	case ChannelAlpha, ChannelBeta, ChannelStable, ChannelEarlyAccess, ChannelRockSolid:
		return true
	}

	return false
}

const (
	ChannelAlpha       = "alpha"
	ChannelBeta        = "beta"
	ChannelStable      = "stable"
	ChannelEarlyAccess = "eary-access"
	ChannelRockSolid   = "rock-solid"
)
