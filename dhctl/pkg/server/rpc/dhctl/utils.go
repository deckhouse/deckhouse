// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dhctl

import (
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type logAfterReturnFunc func()

func logInformationAboutInstance(params ServiceParams, logger log.Logger) logAfterReturnFunc {
	podName := params.PodName
	podWithPrefix := fmt.Sprintf("pod/%s", podName)
	warnAboutNs := ""
	ns := params.PodNamespace

	if ns == "" {
		warnAboutNs = "Warning! Use default namespace."
		ns = "d8-commander"
	}

	logger.LogInfoF(
		"Task is running by DHCTL Server '%s'. For get dhctl logs using next command: d8 k -n %s logs %s %s\n",
		podWithPrefix,
		ns,
		podName,
		warnAboutNs,
	)

	return func() { logger.LogInfoF("Task done by DHCTL Server %s\n", podWithPrefix) }
}
