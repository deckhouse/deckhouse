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

package version

import (
	"golang.org/x/mod/semver"
)

func GetMax(v, w string) string {
	switch semver.Compare(v, w) {
	case -1:
		return w
	case 0:
		return w
	case 1:
		return v
	}
	return v
}

func GetMin(v, w string) string {
	switch semver.Compare(v, w) {
	case -1:
		return v
	case 0:
		return w
	case 1:
		return w
	}
	return v
}
