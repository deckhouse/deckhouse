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

package bashible

import (
	"strings"
)

type ModeType = string

const (
	ModeUnmanaged ModeType = "Unmanaged"
	ModeDirect    ModeType = "Direct"
	ModeProxy     ModeType = "Proxy"
	ModeLocal     ModeType = "Local"
)

func ToModeType(mode string) ModeType {
	val := strings.ToLower(mode)
	switch val {
	case "direct":
		return ModeDirect
	case "proxy":
		return ModeProxy
	case "local":
		return ModeLocal
	default:
		return ModeUnmanaged
	}
}
