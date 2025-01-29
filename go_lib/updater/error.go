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

package updater

var ErrDeployConditionsNotMet = NewNotReadyForDeployError("deploy conditions not met")

func NewNotReadyForDeployError(message string) *NotReadyForDeployError {
	return &NotReadyForDeployError{message: message}
}

type NotReadyForDeployError struct {
	message string
}

func (n *NotReadyForDeployError) Error() string {
	message := "not ready for deploy"
	if n.message != "" {
		message += ": " + n.message
	}

	return message
}
