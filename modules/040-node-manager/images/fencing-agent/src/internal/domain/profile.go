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

package domain

import "slices"

type ProfileName string

const (
	ProfileCritical ProfileName = "critical"
	ProfileMedium   ProfileName = "medium"
	ProfileModerate ProfileName = "moderate"
	ProfileSlow     ProfileName = "slow"
)

func ProfileNames() []ProfileName {
	return []ProfileName{ProfileCritical, ProfileMedium, ProfileModerate, ProfileSlow}
}

func (p ProfileName) IsValid() bool {
	return slices.Contains(ProfileNames(), p)
}
