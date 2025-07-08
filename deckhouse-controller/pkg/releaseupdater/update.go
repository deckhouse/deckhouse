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

package releaseupdater

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
)

type Settings struct {
	NotificationConfig     NotificationConfig
	DisruptionApprovalMode string
	Mode                   v1alpha2.UpdateMode
	Windows                update.Windows
	Subject                string
}

func (s *Settings) InDisruptionApprovalMode() bool {
	if s.DisruptionApprovalMode == "" || s.DisruptionApprovalMode == v1alpha2.UpdateModeAuto.String() {
		return false
	}

	return true
}

func (s *Settings) InManualMode() bool {
	return s.Mode == v1alpha2.UpdateModeManual
}
